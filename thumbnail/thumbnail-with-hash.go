package thumbnail

type thumbnailWithHash struct {
	Board                 string `bun:"board"`
	PostNumber            int64  `bun:"post_number"`
	ThumbnailInternalHash []byte `bun:"thumbnail_internal_hash"`
}
