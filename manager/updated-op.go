package manager

import (
	"kaguya/api"
)

type updatedOp struct {
	PostNumber   int64   `bun:"post_number"`
	Comment      *string `bun:"comment"`
	MediaDeleted *bool   `bun:"media_deleted"`
	Sticky       bool    `bun:"sticky"`
	Closed       bool    `bun:"closed"`
}

func toUpdatedOp(p *api.Post) updatedOp {
	postNumber := p.No

	comment := p.Com
	if comment != nil && *comment == "" {
		comment = nil
	}

	var mediaDeleted *bool
	if p.FileDeleted != nil && *p.FileDeleted == 1 {
		mediaDeleted = &trueBool
	} else if p.Tim != nil {
		mediaDeleted = &falseBool
	}

	sticky := p.Sticky != nil && *p.Sticky == 1
	closed := p.Closed != nil && *p.Closed == 1

	return updatedOp{
		PostNumber:   postNumber,
		Comment:      comment,
		MediaDeleted: mediaDeleted,
		Sticky:       sticky,
		Closed:       closed,
	}
}
