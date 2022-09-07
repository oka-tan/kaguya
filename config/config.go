//Package config wraps Kaguya's configuration
package config

import (
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

//Config parametrizes Kaguya's configuration.
type Config struct {
	APIConfig        APIConfig      `toml:"api"`
	MediaConfig      *MediaConfig   `toml:"media"`
	ThumbnailsConfig *MediaConfig   `toml:"thumbnails"`
	OekakiConfig     *MediaConfig   `toml:"oekaki"`
	PostgresConfig   PostgresConfig `toml:"postgres"`
	Boards           []BoardConfig  `toml:"boards"`
}

//APIConfig parametrizes Kaguya's configuration for the consumption of 4chan's API.
type APIConfig struct {
	RequestTimeout string `toml:"request_timeout"`
	Host           string `toml:"host"`
	Goroutines     int    `toml:"goroutines"`
	NapTime        string `toml:"nap_time"`
}

//MediaConfig parametrizes Kaguya's configuration for downloading media from 4chan and posting it to S3.
type MediaConfig struct {
	RequestTimeout    string `toml:"request_timeout"`
	Host              string `toml:"host"`
	Goroutines        int    `toml:"goroutines"`
	S3Endpoint        string `toml:"s3_endpoint"`
	S3AccessKeyID     string `toml:"s3_access_key_id"`
	S3SecretAccessKey string `toml:"s3_secret_access_key"`
	S3UseSSL          bool   `toml:"s3_use_ssl"`
	S3BucketName      string `toml:"s3_bucket_name"`
	S3Region          string `toml:"s3_region"`
}

//PostgresConfig parametrizes Kaguya's configuration for PostgreSQL
type PostgresConfig struct {
	BatchSize        int    `toml:"batch_size"`
	ConnectionString string `toml:"connection_string"`
}

//BoardConfig parametrizes Kaguya's configuration for each board being scraped
type BoardConfig struct {
	Name        string `toml:"name"`
	LongNapTime string `toml:"long_nap_time"`
	Thumbnails  bool   `toml:"thumbnails"`
	Media       bool   `toml:"media"`
	Oekaki      bool   `toml:"oekaki"`
	SkipArchive bool   `toml:"skip_archive"`
	BStyle      bool   `toml:"b_style"`
	PageCap     uint8  `toml:"page_cap"`
}

//LoadConfig reads config.json and unmarshals it into a Config struct.
//Errors might be returned due to IO or invalid JSON.
func LoadConfig() Config {
	configFile := os.Getenv("KAGUYA_CONFIG")

	if configFile == "" {
		configFile = "./config.toml"
	}

	f, err := os.Open(configFile)

	if err != nil {
		log.Fatalf("Error loading configuration file: %v", err)
	}

	defer f.Close()

	var conf Config

	if _, err := toml.NewDecoder(f).Decode(&conf); err != nil {
		log.Fatalln(err)
	}

	return conf
}
