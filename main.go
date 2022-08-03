package main

import (
	"context"
	"database/sql"
	"fmt"
	"kaguya/api"
	"kaguya/config"
	"kaguya/manager"
	"kaguya/media"
	"kaguya/thumbnail"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Error initializing logger: %v", err)
	}

	logger.Info("Starting Kaguya")
	logger.Sync()

	time.Sleep(5 * time.Second)

	conf := config.LoadConfig()
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(conf.PostgresConfig.ConnectionString)))
	pg := bun.NewDB(sqldb, pgdialect.New())

	s3Client, err := minio.New(conf.S3Config.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(conf.S3Config.S3AccessKeyID, conf.S3Config.S3SecretAccessKey, ""),
		Secure: conf.S3Config.S3UseSSL,
	})

	if err != nil {
		logger.Fatal("Error creating S3 client", zap.Error(err))
	}

	bucketExists, err := s3Client.BucketExists(context.Background(), conf.S3Config.S3BucketName)

	if err != nil {
		logger.Fatal("Error checking if bucket exists", zap.Error(err))
	}

	if !bucketExists {
		err = s3Client.MakeBucket(context.Background(), conf.S3Config.S3BucketName, minio.MakeBucketOptions{
			Region:        conf.S3Config.S3Region,
			ObjectLocking: false,
		})

		if err != nil {
			logger.Fatal("Error creating S3 media bucket", zap.Error(err))
		}
	}

	mediaService := media.NewService(&conf.ImagesConfig, conf.S3Config.S3BucketName, pg, s3Client, logger)
	thumbnailService := thumbnail.NewService(&conf.ThumbnailsConfig, conf.S3Config.S3BucketName, pg, s3Client, logger)
	apiService := api.NewService(conf.APIConfig, logger)

	for _, boardConfig := range conf.Boards {
		_, err := pg.QueryContext(
			context.Background(),
			fmt.Sprintf(
				"CREATE TABLE IF NOT EXISTS post_%s PARTITION OF post FOR VALUES IN ('%s')",
				boardConfig.Name,
				boardConfig.Name,
			),
		)

		if err != nil {
			logger.Fatal(
				"Error creating board partition: presumably it already exists with the incorrect name",
				zap.String("board", boardConfig.Name),
				zap.Error(err),
			)
		}

		boardManager := manager.NewBoardManager(
			pg,
			mediaService,
			thumbnailService,
			boardConfig,
			apiService,
			logger,
			conf.PostgresConfig.BatchSize,
		)

		go func(boardManager manager.BoardManager) {
			if err := boardManager.Init(); err != nil {
				logger.Fatal("Erorr init'ing board manager", zap.Error(err))
			}

			boardManager.Run()
		}(boardManager)
	}

	logger.Sync()
	forever := make(chan bool)
	<-forever
}
