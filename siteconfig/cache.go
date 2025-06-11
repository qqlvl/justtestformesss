package siteconfig

import "time"

const cacheTtl = time.Minute * 120

var cacheHeaders = []string{
	"Content-Type",
	"Content-Encoding",
	"Content-Language",
}

type PageCacheEntry struct {
	Path      string
	FetchedAt time.Time
	Content   []byte
	Status    int
	Headers   map[string]string
}
