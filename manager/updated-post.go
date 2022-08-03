package manager

import (
	"kaguya/api"
)

type updatedPost struct {
	PostNumber   int64   `bun:"post_number"`
	Comment      *string `bun:"comment"`
	MediaDeleted *bool   `bun:"media_deleted"`
}

var trueBool = true
var falseBool = false

func toUpdatedPost(p api.Post) updatedPost {
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

	return updatedPost{
		PostNumber:   postNumber,
		Comment:      comment,
		MediaDeleted: mediaDeleted,
	}
}
