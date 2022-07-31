package thumbnail

type thumbnailWithHash struct {
	Board                 string `json:"board"`
	PostNumber            int64  `json:"post_number"`
	ThumbnailInternalHash []byte `json:"thumbnail_internal_hash"`
}
