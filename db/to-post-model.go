package db

import (
	"encoding/base64"
	"kaguya/api"
	"time"

	"github.com/samber/lo"
)

// ToPostModel turns an API post into a
// database post.
// This function does NOT calculate replies,
// set last_modified, set created_at, calculate replies
// counts or calculate hashes
func ToPostModel(board string, p api.Post) Post {
	threadNumber := p.Resto
	if threadNumber == 0 {
		threadNumber = p.No
	}

	op := p.No == threadNumber

	timePosted := time.Unix(int64(p.Time), 0)

	name := p.Name
	if name != nil && (*name == "" || *name == "Anonymous") {
		name = nil
	}

	tripcode := p.Trip
	if tripcode != nil && *tripcode == "" {
		tripcode = nil
	}

	capcode := p.Capcode
	if capcode != nil && *capcode == "" {
		capcode = nil
	}

	posterID := p.ID
	if posterID != nil && *posterID == "" {
		posterID = nil
	}

	country := p.Country
	if country != nil && *country == "" {
		country = nil
	}

	flag := p.BoardFlag
	if country != nil || (flag != nil && *flag == "") {
		flag = nil
	}

	subject := p.Sub
	if subject != nil && *subject == "" {
		subject = nil
	}

	comment := p.Com
	if comment != nil && *comment == "" {
		comment = nil
	}

	hasMedia := false

	mediaTimestamp := p.Tim
	if mediaTimestamp != nil && *mediaTimestamp == 0 {
		hasMedia = true
		mediaTimestamp = nil
	}

	var media4chanHash *[]byte
	if p.MD5 != nil && *p.MD5 != "" {
		hasMedia = true
		base64Bytes, err := base64.StdEncoding.DecodeString(*p.MD5)
		if err == nil {
			media4chanHash = &base64Bytes
		}
	}

	var mediaExtension *string
	if p.Ext != nil && len(*p.Ext) > 1 {
		hasMedia = true
		mediaExtensionV := (*p.Ext)[1:]
		mediaExtension = &mediaExtensionV
	}

	mediaFileName := p.Filename
	if mediaFileName != nil && *mediaFileName == "" {
		hasMedia = true
		mediaFileName = nil
	}

	mediaSize := p.Fsize
	if mediaSize != nil && *mediaSize == 0 {
		hasMedia = true
		mediaSize = nil
	}

	mediaHeight := p.H
	mediaWidth := p.W
	if (mediaHeight != nil && *mediaHeight == 0) || (mediaWidth != nil && *mediaWidth == 0) {
		hasMedia = true
		mediaHeight = nil
		mediaWidth = nil
	}

	thumbnailHeight := p.TnH
	thumbnailWidth := p.TnW
	if (thumbnailHeight != nil && *thumbnailHeight == 0) || (thumbnailWidth != nil && *thumbnailWidth == 0) {
		hasMedia = true
		thumbnailHeight = nil
		thumbnailWidth = nil
	}

	var mediaDeleted *bool
	if p.FileDeleted != nil && *p.FileDeleted == 1 {
		hasMedia = true
		mediaDeleted = lo.ToPtr(true)
	} else if hasMedia {
		mediaDeleted = lo.ToPtr(false)
	}

	var spoiler *bool
	var customSpoiler *int16
	if hasMedia {
		if p.Spoiler != nil && *p.Spoiler == 1 {
			spoiler = lo.ToPtr(true)
			if p.CustomSpoiler != nil {
				customSpoilerV := int16(*p.CustomSpoiler)
				customSpoiler = &customSpoilerV
			}
		} else {
			spoiler = lo.ToPtr(false)
		}
	}

	var sticky *bool
	if op {
		if p.Sticky != nil && *p.Sticky == 1 {
			sticky = lo.ToPtr(true)
		} else {
			sticky = lo.ToPtr(false)
		}
	}

	var closed *bool
	if op {
		if p.Closed != nil && *p.Closed == 1 {
			closed = lo.ToPtr(true)
		} else {
			closed = lo.ToPtr(false)
		}
	}

	var posters *int16
	if op && p.UniqueIPs != nil && *p.UniqueIPs != 0 {
		posters = p.UniqueIPs
	}

	var since4Pass *int16
	if p.Since4Pass != nil && *p.Since4Pass != 0 {
		since4Pass = p.Since4Pass
	}

	return Post{
		Board:           board,
		PostNumber:      p.No,
		ThreadNumber:    threadNumber,
		Op:              op,
		Deleted:         false,
		TimePosted:      timePosted,
		Name:            name,
		Tripcode:        tripcode,
		Capcode:         capcode,
		PosterID:        posterID,
		Country:         country,
		Flag:            flag,
		Email:           nil,
		Subject:         subject,
		Comment:         comment,
		HasMedia:        hasMedia,
		MediaDeleted:    mediaDeleted,
		MediaTimestamp:  mediaTimestamp,
		Media4chanHash:  media4chanHash,
		MediaExtension:  mediaExtension,
		MediaFileName:   mediaFileName,
		MediaSize:       mediaSize,
		MediaHeight:     mediaHeight,
		MediaWidth:      mediaWidth,
		ThumbnailHeight: thumbnailHeight,
		ThumbnailWidth:  thumbnailWidth,
		Spoiler:         spoiler,
		CustomSpoiler:   customSpoiler,
		Sticky:          sticky,
		Closed:          closed,
		Posters:         posters,
		Since4Pass:      since4Pass,

		Hidden: false,
	}
}
