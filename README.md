# Kaguya

4chan Koiwai-compatible scraper

## Features
- Reasonable, configurable memory usage
- Low CPU usage
- S3 compatibility (also not compatible with anything other than S3)
- File deduping across every board at once with SHA-256
- Reasonable number of database operations (one insert per post (obviously), one update to the OP each time a thread is updated (to correct the number of replies and possibly also other fields), one update per post with image to inject the image hash, one update per post with image to inject the thumbnail hash, extra updates whenever posts are modified or deleted)
- Stores raw post HTML in database

## Usage

* First of all, the schema file needs to be ran manually on postgres
* Edit the config.example.toml file to fit your use case
* Either export the KAGUYA_CONFIG variable to point it to your configuration file or leave it as config.toml in the project root
* Install golang 1.18 or above
* Run `go build .` on the project root to build your executable
* Run the executable

## Known Issues

* Largely untested
