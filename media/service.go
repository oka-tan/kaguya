package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"kaguya/config"
	"kaguya/db"
	"kaguya/utils"
	"mime"
	"net/http"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
)

//Service organizes media downloads and uploads
type Service struct {
	s3Client  *minio.Client
	webClient http.Client
	pg        *bun.DB
	queue     chan queuedImage
	host      string
	bucket    string
	logger    *zap.Logger
	batch     []mediaWithHash
	mutex     sync.Mutex
}

//NewService creates a Service
func NewService(conf *config.ImagesConfig, bucketName string, pg *bun.DB, s3Client *minio.Client, logger *zap.Logger) *Service {
	defer logger.Sync()
	s := &Service{
		s3Client:  s3Client,
		webClient: http.Client{},
		pg:        pg,
		queue:     make(chan queuedImage, 50000),
		host:      conf.Host,
		bucket:    bucketName,
		logger:    logger,
	}

	for i := 0; i < conf.Goroutines; i++ {
		logger.Debug("Starting media goroutine", zap.Int("number", i))
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
		s.logger.Debug("Inserting media hashes", zap.Int("count", len(s.batch)))

		if len(s.batch) > 0 {
			_, err := s.pg.NewUpdate().
				With("_data", s.pg.NewValues(&s.batch)).
				Model(&db.Post{}).
				TableExpr("_data").
				Set("media_internal_hash = _data.media_internal_hash").
				Set("last_modified = ?", now).
				Where("post.board = _data.board").
				Where("post.post_number = _data.post_number").
				Exec(context.Background())

			if err != nil {
				s.logger.Error("Error inserting media hashes into db", zap.Error(err))
			} else {
				s.batch = s.batch[0:0]
			}
		}

		s.mutex.Unlock()
		s.logger.Sync()
	}
}

func (s *Service) run() {
	buffer := bytes.NewBuffer(make([]byte, 0, 6*1024*1024))

	for queuedImage := range s.queue {
		source := fmt.Sprintf("%s/%s/%d.%s", s.host, queuedImage.board, queuedImage.mediaTimestamp, queuedImage.mediaExtension)

		for i := 0; ; i++ {
			resp, err := s.webClient.Get(source)

			if err != nil {
				s.logger.Error("Error downloading media", zap.String("source", source), zap.Error(err))
				if i < 3 {
					time.Sleep(30 * time.Second)
					continue
				} else {
					break
				}
			}

			if resp.StatusCode == 404 {
				resp.Body.Close()
				s.logger.Error("Media 404d", zap.String("source", source))
				break
			}

			if resp.StatusCode != 200 {
				resp.Body.Close()
				s.logger.Error("Error downloading media", zap.String("source", source), zap.String("status-code", resp.Status))
				if i < 3 {
					time.Sleep(30 * time.Second)
					continue
				} else {
					break
				}
			}

			buffer.Reset()
			if _, err := buffer.ReadFrom(resp.Body); err != nil {
				resp.Body.Close()
				s.logger.Error("Error reading response body into buffer", zap.String("source", source), zap.Error(err))
				if i < 3 {
					time.Sleep(30 * time.Second)
					continue
				} else {
					break
				}
			}

			resp.Body.Close()

			sha256HashCalculator := sha256.New()
			if _, err := io.Copy(sha256HashCalculator, bytes.NewReader(buffer.Bytes())); err != nil {
				s.logger.Error("Error computing SHA-256 hash", zap.String("source", source), zap.Error(err))
				break
			}

			sha256Hash := sha256HashCalculator.Sum(nil)

			sha256AlreadyExists, err := s.pg.
				NewSelect().
				Model(&db.Media{}).
				Where("hash = ?", sha256Hash).
				Exists(context.Background())

			if err != nil {
				s.logger.Error("Error verifying if hash already exists", zap.String("source", source), zap.Error(err))
				break
			}

			if sha256AlreadyExists {
				s.logger.Debug("Media already exists", zap.String("source", source), zap.Binary("hash", sha256Hash))

				s.mutex.Lock()
				s.batch = append(s.batch, mediaWithHash{
					Board:             queuedImage.board,
					MediaInternalHash: sha256Hash,
					PostNumber:        queuedImage.postNumber,
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
					CacheControl: "public, immutable, max-age=604800",
					ContentType:  mime.TypeByExtension("." + queuedImage.mediaExtension),
				},
			); err != nil {
				s.logger.Error("Error putting file on S3", zap.String("source", source), zap.Binary("hash", sha256Hash), zap.Error(err))
			}

			_, err = s.pg.NewInsert().
				Model(&db.Media{Hash: sha256Hash}).
				Exec(context.Background())

			if err != nil {
				s.logger.Error("Error putting hash on db", zap.String("source", source), zap.Binary("hash", sha256Hash), zap.Error(err))
			} else {
				s.logger.Debug("File successfully put on db", zap.String("source", source), zap.Binary("hash", sha256Hash))
			}

			s.mutex.Lock()
			s.batch = append(s.batch, mediaWithHash{
				Board:             queuedImage.board,
				MediaInternalHash: sha256Hash,
				PostNumber:        queuedImage.postNumber,
			})
			s.mutex.Unlock()

			break
		}
	}
}

//Enqueue enqueues some media for download
//This deduping thing probably looks absolutely beyond stupid but it saves absurd amounts of memory
func (s *Service) Enqueue(posts []db.Post) {
	queuedImages := make([]queuedImage, 0, 10)
	extensions := map[string]string{
		"jpg":  "jpg",
		"png":  "png",
		"gif":  "gif",
		"pdf":  "pdf",
		"swf":  "swf",
		"tgk":  "tgk",
		"webm": "webm",
	}

	for _, p := range posts {
		if p.HasMedia && p.MediaTimestamp != nil && p.MediaExtension != nil {
			cachedExtensionString, extensionCached := extensions[*p.MediaExtension]

			if extensionCached {
				queuedImages = append(queuedImages, queuedImage{
					board:          p.Board,
					postNumber:     p.PostNumber,
					mediaTimestamp: *p.MediaTimestamp,
					mediaExtension: cachedExtensionString,
				})
			} else {
				clonedExtension := utils.CloneString(*p.MediaExtension)
				extensions[clonedExtension] = clonedExtension

				queuedImages = append(queuedImages, queuedImage{
					board:          p.Board,
					postNumber:     p.PostNumber,
					mediaTimestamp: *p.MediaTimestamp,
					mediaExtension: clonedExtension,
				})
			}
		}
	}

	go func(queuedImages []queuedImage) {
		for _, queuedImage := range queuedImages {
			s.queue <- queuedImage
		}
	}(queuedImages)
}
