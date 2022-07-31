package manager

import (
	"hash/fnv"
	"kaguya/api"
)

type cachedThread struct {
	lastModified uint64
	posts        map[int64]cachedPost
}

type cachedPost struct {
	comHash      uint32
	mediaDeleted bool
}

func hashString(s *string) uint32 {
	if s == nil {
		return 0
	}

	hash := fnv.New32()
	hash.Sum([]byte(*s))
	return hash.Sum32()
}

func toCachedPost(p *api.Post) cachedPost {
	comHash := hashString(p.Com)
	mediaDeleted := p.FileDeleted != nil && *p.FileDeleted == 1

	return cachedPost{
		comHash:      comHash,
		mediaDeleted: mediaDeleted,
	}
}

func toCachedPosts(posts []api.Post) map[int64]cachedPost {
	cachedPosts := make(map[int64]cachedPost)

	for _, p := range posts {
		cachedPosts[p.No] = toCachedPost(&p)
	}

	return cachedPosts
}

func postModified(p *api.Post, cp *cachedPost) bool {
	if p.FileDeleted != nil && *p.FileDeleted == 1 && (!cp.mediaDeleted) {
		return true
	}

	if hashString(p.Com) != cp.comHash {
		return true
	}

	return false
}
