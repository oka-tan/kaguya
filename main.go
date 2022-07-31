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

	mediaService := media.NewService(&conf.ImagesConfig, pg, logger)
	thumbnailService := thumbnail.NewService(&conf.ThumbnailsConfig, pg, logger)
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
