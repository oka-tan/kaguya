package db

import "github.com/uptrace/bun"

//Thumbnail is file hash in the db
type Thumbnail struct {
	bun.BaseModel `bun:"thumbnail"`

	Hash []byte `bun:"hash,pk"`
}
