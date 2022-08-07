package thumbnail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"kaguya/config"
	"kaguya/db"
	"net/http"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

//Service organizes downloads and uploads of thumbnails
type Service struct {
	s3Client  *minio.Client
	webClient http.Client
	pg        *bun.DB
	queue     chan queuedThumbnail
	host      string
	bucket    string
	logger    *zap.Logger
	batch     []thumbnailWithHash
	mutex     sync.Mutex
}

//NewService creates a Service
func NewService(conf *config.ImagesConfig, bucketName string, pg *bun.DB, s3Client *minio.Client, logger *zap.Logger) *Service {
	defer logger.Sync()
	s := &Service{
		s3Client:  s3Client,
		webClient: http.Client{},
		pg:        pg,
		queue:     make(chan queuedThumbnail, 50000),
		host:      conf.Host,
		bucket:    bucketName,
		logger:    logger,
	}

	for i := 0; i < conf.Goroutines; i++ {
		logger.Debug("Starting thumbnail goroutine", zap.Int("number", i))
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
		s.logger.Debug("Updating thumbnail hashes", zap.Int("count", len(s.batch)))

		if len(s.batch) > 0 {
			_, err := s.pg.NewUpdate().
				With("_data", s.pg.NewValues(&s.batch)).
				Model(&db.Post{}).
				TableExpr("_data").
				Set("thumbnail_internal_hash = _data.thumbnail_internal_hash").
				Set("last_modified = ?", now).
				Where("post.board = _data.board").
				Where("post.post_number = _data.post_number").
				Exec(context.Background())

			if err != nil {
				s.logger.Error("Error updating thumbnail hashes", zap.Error(err))
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

	for queuedThumbnail := range s.queue {
		source := fmt.Sprintf("%s/%s/%ds.jpg", s.host, queuedThumbnail.board, queuedThumbnail.mediaTimestamp)

		for i := 0; ; i++ {
			resp, err := s.webClient.Get(source)

			if err != nil {
				s.logger.Error("Error downloading thumbnail", zap.String("source", source), zap.Int("attempt", i), zap.Error(err))
				if i < 3 {
					time.Sleep(30 * time.Second)
					continue
				} else {
					break
				}
			}

			if resp.StatusCode == 404 {
				s.logger.Error("Thumbnail 404d", zap.String("source", source), zap.Int("attempt", i))
				resp.Body.Close()
				break
			}

			if resp.StatusCode != 200 {
				s.logger.Error("Error downloading thumbnail", zap.String("source", source), zap.Int("attempt", i), zap.String("status-code", resp.Status))
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
				s.logger.Error("Error reading thumbnail body into buffer", zap.String("source", source), zap.Int("attempt", i), zap.Error(err))
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
				s.logger.Error("Error calculating thumbnail hash", zap.String("source", source), zap.Error(err))
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
				Model(&db.Media{}).
				Where("hash = ?", sha256Hash).
				Exists(context.Background())

			if err != nil {
				s.logger.Error("Error verifying if thumbnail already exists", zap.String("source", source), zap.Error(err))
				break
			}

			if sha256AlreadyExists {
				s.logger.Debug("Thumbnail already exists", zap.String("source", source), zap.Binary("hash", sha256Hash))
				s.mutex.Lock()
				s.batch = append(s.batch, thumbnailWithHash{
					Board:                 queuedThumbnail.board,
					PostNumber:            queuedThumbnail.postNumber,
					ThumbnailInternalHash: sha256Hash,
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
					ContentType:  "image/jpg",
					CacheControl: "public, immutable, max-age=604800",
				},
			); err != nil {
				s.logger.Error("Error putting thumbnail on S3", zap.String("source", source), zap.Binary("hash", sha256Hash), zap.Error(err))
				break
			}

			_, err = s.pg.NewInsert().
				Model(&db.Media{Hash: sha256Hash}).
				On("CONFLICT DO NOTHING").
				Exec(context.Background())

			if err != nil {
				s.logger.Error("Error putting thumbnail's hash on db", zap.String("source", source), zap.Binary("hash", sha256Hash), zap.Error(err))
				break
			} else {
				s.logger.Debug("Successfully put thumbnail on db", zap.String("source", source), zap.Binary("hash", sha256Hash), zap.Error(err))
			}

			s.mutex.Lock()
			s.batch = append(s.batch, thumbnailWithHash{
				Board:                 queuedThumbnail.board,
				PostNumber:            queuedThumbnail.postNumber,
				ThumbnailInternalHash: sha256Hash,
			})
			s.mutex.Unlock()

			break
		}
	}
}

//Retrieve downloads a thumbnail and returns the SHA-256 hash on success
//and nil otherwise
func (s *Service) Enqueue(posts []db.Post) {
	queuedThumbnails := make([]queuedThumbnail, 0, 10)

	for _, p := range posts {
		if p.HasMedia && p.MediaTimestamp != nil {
			queuedThumbnails = append(queuedThumbnails, queuedThumbnail{
				board:          p.Board,
				mediaTimestamp: *p.MediaTimestamp,
				postNumber:     p.PostNumber,
			})
		}
	}

	go func(queuedThumbnails []queuedThumbnail) {
		for _, queuedThumbnail := range queuedThumbnails {
			s.queue <- queuedThumbnail
		}
	}(queuedThumbnails)
}
