package media

type mediaWithHash struct {
	Board             string `bun:"board"`
	PostNumber        int64  `bun:"post_number"`
	MediaInternalHash []byte `bun:"media_internal_hash"`
}
