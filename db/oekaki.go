package db

import "github.com/uptrace/bun"

//Oekaki is file hash in the db
type Oekaki struct {
	bun.BaseModel `bun:"oekaki"`

	Hash []byte `bun:"hash,pk"`
}
