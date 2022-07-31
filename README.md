# kaguya

4chan Koiwai-compatible scraper

## Features
- Reasonable, configurable memory usage
- Low CPU usage
- S3 compatibility (also not compatible with anything other than S3)
- File deduping across every board at once with SHA-256
- Reasonable number of database operations (one insert per post (obviously), one update to the OP each time a thread is updated (to correct the number of replies and possibly also other fields), one update per post with image to inject the image hash, one update per post with image to inject the thumbnail hash, extra updates whenever posts are modified or deleted)
- Stores raw post HTML in database

## Usage

* First of all, the koiwai schema file needs to be ran manually on postgres, like `psql -U koiwai -f schema.sql`
* The S3 bucket also needs to already exist
* Edit the config.example.json file to fit your use case
* Either export the KAGUYA_CONFIG variable to point it to your configuration file or leave it as config.json in the project root
* Install golang 1.18 or above (there is no above as of the writing of this README.md)
* Run `go build .` on the project root to build your executable
* Run the executable

## Configuration

* APIConfig - API consumption configuration
  * RequestTimeout - Request timeout for the API as a Go duration string (i.e. "30s" for 30 seconds)
  * Host - API host 
  * Goroutines - Number of simultaneous requests Kaguya may make to the API
  * NapTime - Minimum time between API requests
* ImagesConfig - Media download and storage configuration
  * RequestTimeout - Request timeout for media downloads as a Go duration string (i.e. "30s" for 30 seconds)
  * Host - 4chan media cdn host
  * S3Endpoint - S3 endpoint for media storage
  * S3AccessKeyID - S3 Access Key ID
  * S3SecretAccessKey - S3 Secret Access Key
  * S3UseSSL - Whether or not to use SSL for S3
  * S3BucketName - S3 bucket name
  * Goroutines - Number of simultaneous requests for media (media is stored in memory before being uploaded to S3, hence each of these costs 6mb of memory)
* ThumbnailsConfig - Thumbnail download and storage configuration
  * RequestTimeout - Request timeout for thumbnail downloads as a Go duration string (i.e. "30s" for 30 seconds)
  * Host - 4chan media cdn host
  * S3Endpoint - S3 endpoint for thumbnail storage
  * S3AccessKeyID - S3 Access Key ID
  * S3SecretAccessKey - S3 Secret Access Key
  * S3UseSSL - Whether or not to use SSL for S3
  * S3BucketName - S3 bucket name
  * Goroutines - Number of simultaneous requests for thumbnails (thumbnails are stored in memory before being uploaded to S3, hence each of these costs 64kb of memory)
* PostgresConfig - Postgres DB configuration
  * ConnectionString - PostgreSQL connection string in the form `postgres://${USER}:${PASSWORD}@${HOST}:${PORT}/${DBNAME}?timeout=${TIMEOUT_DURATION}&sslmode=${POSTGRES_SSL_MODE}`, where the timeout is a Go duration string and SSL mode is either "disable" or "enable"
* Boards - Array of boards to be scraped
  * Name - Board name (i.e. "a", "pol")
  * LongNapTime - Wait between loops as a Go duration string (i.e. "30s" for 30 seconds)
  * Thumbnails - Whether or not to download thumbnails for the board
  * Media - Whether or not to download media for the board
  * SkipArchive - Whether or not to skip the archive when initially scraping (scraping the archive can take half an hour on boards like /v/)

## Known Issues

* Largely untested
* Doesn't download oekaki

