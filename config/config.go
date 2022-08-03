package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
)

//Config parametrizes Kaguya's configuration.
type Config struct {
	APIConfig        APIConfig
	ImagesConfig     ImagesConfig
	ThumbnailsConfig ImagesConfig
	PostgresConfig   PostgresConfig
	S3Config         S3Config
	SkipArchive      bool
	Boards           []BoardConfig
}

//S3Config parametrizes Kaguya's configuration for S3 storage.
type S3Config struct {
	S3Endpoint        string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3UseSSL          bool
	S3BucketName      string
	S3Region          string
}

//APIConfig parametrizes Kaguya's configuration for the consumption of 4chan's API.
type APIConfig struct {
	RequestTimeout string
	Host           string
	Goroutines     int
	NapTime        string
}

//ImagesConfig parametrizes Kaguya's configuration for downloading media from 4chan and posting it to S3.
type ImagesConfig struct {
	RequestTimeout string
	Host           string
	Goroutines     int
}

//PostgresConfig parametrizes Kaguya's configuration for PostgreSQL
type PostgresConfig struct {
	BatchSize        int
	ConnectionString string
}

//BoardConfig parametrizes Kaguya's configuration for each board being scraped
type BoardConfig struct {
	Name        string
	LongNapTime string
	Thumbnails  bool
	Media       bool
	SkipArchive bool
}

//LoadConfig reads config.json and unmarshals it into a Config struct.
//Errors might be returned due to IO or invalid JSON.
func LoadConfig() Config {
	configFile := os.Getenv("KAGUYA_CONFIG")

	if configFile == "" {
		configFile = "./config.json"
	}

	blob, err := ioutil.ReadFile(configFile)

	if err != nil {
		log.Fatalf("Error loading configuration file: %v", err)
	}

	var conf Config

	err = json.Unmarshal(blob, &conf)

	if err != nil {
		log.Fatalf(
			"Error unmarshalling configuration file contents to JSON:\n File contents: %s\n Error message: %s",
			blob,
			err,
		)
	}

	return conf
}
