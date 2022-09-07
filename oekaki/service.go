//Package oekaki wraps oekaki downloads and storage
package oekaki

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"kaguya/config"
	"kaguya/db"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/minio/sha256-simd"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

//Service organizes downloads and uploads of oekaki
type Service struct {
	s3Client  *minio.Client
	webClient http.Client
	pg        *bun.DB
	queue     chan queuedOekaki
	host      string
	bucket    string
	logger    *zap.Logger
	batch     []oekakiWithHash
	mutex     sync.Mutex
}

//NewService creates a Service
func NewService(conf *config.MediaConfig, pg *bun.DB, logger *zap.Logger) *Service {
	defer logger.Sync()

	s3Client, err := minio.New(conf.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(conf.S3AccessKeyID, conf.S3SecretAccessKey, ""),
		Secure: conf.S3UseSSL,
	})

	if err != nil {
		logger.Fatal("Error creating S3 client", zap.Error(err))
	}

	bucketExists, err := s3Client.BucketExists(context.Background(), conf.S3BucketName)

	if err != nil {
		logger.Fatal("Error checking if bucket exists", zap.Error(err))
	}

	if !bucketExists {
		err = s3Client.MakeBucket(context.Background(), conf.S3BucketName, minio.MakeBucketOptions{
			Region:        conf.S3Region,
			ObjectLocking: false,
		})

		if err != nil {
			logger.Fatal("Error creating S3 oekaki bucket", zap.Error(err))
		}
	}

	s := &Service{
		s3Client:  s3Client,
		webClient: http.Client{},
		pg:        pg,
		queue:     make(chan queuedOekaki, 10000),
		host:      conf.Host,
		bucket:    conf.S3BucketName,
		logger:    logger,
	}

	for i := 0; i < conf.Goroutines; i++ {
		logger.Debug("Starting oekaki goroutine", zap.Int("number", i))
		go func() {
			s.run()
		}()
	}

	go func() {
		s.insert()
	}()

	return s
}

func (s *Service) insert() {
	for {
		time.Sleep(30 * time.Second)

		now := time.Now()

		s.mutex.Lock()
		s.logger.Debug("Updating oekaki hashes", zap.Int("count", len(s.batch)))

		if len(s.batch) > 0 {
			_, err := s.pg.NewUpdate().
				With("_data", s.pg.NewValues(&s.batch)).
				Model(&db.Post{}).
				TableExpr("_data").
				Set("oekaki_internal_hash = _data.oekaki_internal_hash").
				Set("last_modified = ?", now).
				Where("post.board = _data.board").
				Where("post.post_number = _data.post_number").
				Where("post.oekaki_internal_hash IS NULL").
				Exec(context.Background())

			if err != nil {
				s.logger.Error("Error updating oekaki hashes", zap.Error(err))
			} else {
				s.batch = s.batch[0:0]
			}
		}

		s.mutex.Unlock()
		s.logger.Sync()
	}
}

func (s *Service) run() {
	buffer := bytes.NewBuffer(make([]byte, 0, 64*1024))

	for queuedOekaki := range s.queue {
		source := fmt.Sprintf("%s/%s/%d.tgkr", s.host, queuedOekaki.board, queuedOekaki.mediaTimestamp)

		for i := 0; ; i++ {
			resp, err := s.webClient.Get(source)

			if err != nil {
				s.logger.Error("Error downloading oekaki", zap.String("source", source), zap.Int("attempt", i), zap.Error(err))
				if i < 3 {
					time.Sleep(30 * time.Second)
					continue
				} else {
					break
				}
			}

			if resp.StatusCode == 404 {
				s.logger.Error("Oekaki 404d", zap.String("source", source), zap.Int("attempt", i))
				resp.Body.Close()
				break
			}

			if resp.StatusCode != 200 {
				s.logger.Error("Error downloading oekaki", zap.String("source", source), zap.Int("attempt", i), zap.String("status-code", resp.Status))
				resp.Body.Close()
				if i < 3 {
					time.Sleep(30 * time.Second)
					continue
				} else {
					break
				}
			}

			buffer.Reset()
			if _, err := buffer.ReadFrom(resp.Body); err != nil {
				s.logger.Error("Error reading oekaki body into buffer", zap.String("source", source), zap.Int("attempt", i), zap.Error(err))
				if i < 3 {
					resp.Body.Close()
					time.Sleep(30 * time.Second)
					continue
				} else {
					resp.Body.Close()
					break
				}
			}

			resp.Body.Close()

			sha256HashCalculator := sha256.New()
			if _, err := io.Copy(sha256HashCalculator, bytes.NewReader(buffer.Bytes())); err != nil {
				s.logger.Error("Error calculating oekaki hash", zap.String("source", source), zap.Error(err))
				if i < 3 {
					time.Sleep(30 * time.Second)
					continue
				} else {
					break
				}
			}

			sha256Hash := sha256HashCalculator.Sum(nil)

			sha256AlreadyExists, err := s.pg.
				NewSelect().
				Model(&db.Oekaki{}).
				Where("hash = ?", sha256Hash).
				Exists(context.Background())

			if err != nil {
				s.logger.Error("Error verifying if oekaki already exists", zap.String("source", source), zap.Error(err))
				break
			}

			if sha256AlreadyExists {
				s.logger.Debug("Oekaki already exists", zap.String("source", source), zap.Binary("hash", sha256Hash))
				s.mutex.Lock()
				s.batch = append(s.batch, oekakiWithHash{
					Board:              queuedOekaki.board,
					PostNumber:         queuedOekaki.postNumber,
					OekakiInternalHash: sha256Hash,
				})
				s.mutex.Unlock()
				break
			}

			sha256String := base64.URLEncoding.EncodeToString(sha256Hash)

			if _, err := s.s3Client.PutObject(
				context.Background(),
				s.bucket,
				sha256String,
				bytes.NewReader(buffer.Bytes()),
				int64(buffer.Len()),
				minio.PutObjectOptions{
					ContentType:  "application/tegaki-replay",
					CacheControl: "private, immutable, max-age=604800",
				},
			); err != nil {
				s.logger.Error("Error putting oekaki on S3", zap.String("source", source), zap.Binary("hash", sha256Hash), zap.Error(err))
				break
			}

			_, err = s.pg.NewInsert().
				Model(&db.Oekaki{Hash: sha256Hash}).
				On("CONFLICT DO NOTHING").
				Exec(context.Background())

			if err != nil {
				s.logger.Error("Error putting oekaki's hash on db", zap.String("source", source), zap.Binary("hash", sha256Hash), zap.Error(err))
				break
			} else {
				s.logger.Debug("Successfully put oekaki on db", zap.String("source", source), zap.Binary("hash", sha256Hash), zap.Error(err))
			}

			s.mutex.Lock()
			s.batch = append(s.batch, oekakiWithHash{
				Board:              queuedOekaki.board,
				PostNumber:         queuedOekaki.postNumber,
				OekakiInternalHash: sha256Hash,
			})
			s.mutex.Unlock()

			break
		}
	}
}

//Enqueue enqueues oekaki downloads.
func (s *Service) Enqueue(posts []db.Post) {
	queuedOekakis := make([]queuedOekaki, 0, 10)

	for _, p := range posts {
		if p.HasMedia && p.MediaTimestamp != nil && p.Comment != nil && strings.Contains(*p.Comment, "<a href=\"javascript:oeReplay") {
			queuedOekakis = append(queuedOekakis, queuedOekaki{
				board:          p.Board,
				mediaTimestamp: *p.MediaTimestamp,
				postNumber:     p.PostNumber,
			})
		}
	}

	go func(queuedOekakis []queuedOekaki) {
		for _, queuedOekaki := range queuedOekakis {
			s.queue <- queuedOekaki
		}
	}(queuedOekakis)
}
