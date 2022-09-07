package utils

import (
	"kaguya/db"
	"time"
)

//PatchLastModifiedCreatedAt edits the LastModified and CreatedAt values of
//each element in the array so they all equal now
func PatchLastModifiedCreatedAt(posts []db.Post, now time.Time) {
	for i := range posts {
		posts[i].LastModified = now
		posts[i].CreatedAt = now

		if posts[i].MediaDeleted != nil && *posts[i].MediaDeleted {
			posts[i].TimeMediaDeleted = &now
		}
	}
}
