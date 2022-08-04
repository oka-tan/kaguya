package oekaki

type oekakiWithHash struct {
	Board              string `bun:"board"`
	PostNumber         int64  `bun:"post_number"`
	OekakiInternalHash []byte `bun:"oekaki_internal_hash"`
}
