package db

import (
	"time"

	"github.com/uptrace/bun"
)

//Post is a post in the db
type Post struct {
	bun.BaseModel `bun:"table:post,alias:post"`

	Board                 string     `bun:"board,pk"`
	PostNumber            int64      `bun:"post_number,pk"`
	ThreadNumber          int64      `bun:"thread_number"`
	Op                    bool       `bun:"op"`
	Deleted               bool       `bun:"deleted"`
	Hidden                bool       `bun:"hidden"`
	TimePosted            time.Time  `bun:"time_posted"`
	LastModified          time.Time  `bun:"last_modified"`
	CreatedAt             time.Time  `bun:"created_at"`
	Name                  *string    `bun:"name"`
	Tripcode              *string    `bun:"tripcode"`
	Capcode               *string    `bun:"capcode"`
	PosterID              *string    `bun:"poster_id"`
	Country               *string    `bun:"country"`
	Flag                  *string    `bun:"flag"`
	Email                 *string    `bun:"email"`
	Subject               *string    `bun:"subject"`
	Comment               *string    `bun:"comment"`
	HasMedia              bool       `bun:"has_media"`
	MediaDeleted          *bool      `bun:"media_deleted"`
	TimeMediaDeleted      *time.Time `bun:"time_media_deleted"`
	MediaTimestamp        *int64     `bun:"media_timestamp"`
	Media4chanHash        *[]byte    `bun:"media_4chan_hash"`
	MediaInternalHash     *[]byte    `bun:"media_internal_hash"`
	ThumbnailInternalHash *[]byte    `bun:"thumbnail_internal_hash"`
	MediaExtension        *string    `bun:"media_extension"`
	MediaFileName         *string    `bun:"media_file_name"`
	MediaSize             *int       `bun:"media_size"`
	MediaHeight           *int16     `bun:"media_height"`
	MediaWidth            *int16     `bun:"media_width"`
	ThumbnailHeight       *int16     `bun:"thumbnail_height"`
	ThumbnailWidth        *int16     `bun:"thumbnail_width"`
	Spoiler               *bool      `bun:"spoiler"`
	CustomSpoiler         *int16     `bun:"custom_spoiler"`
	Sticky                *bool      `bun:"sticky"`
	Closed                *bool      `bun:"closed"`
	Posters               *int16     `bun:"posters"`
	Replies               *int16     `bun:"replies"`
	Since4Pass            *int16     `bun:"since4pass"`
}
