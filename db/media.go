package db

import "github.com/uptrace/bun"

//Media is file hash in the db
type Media struct {
	bun.BaseModel `bun:"media"`

	Hash []byte `bun:"hash,pk"`
}
