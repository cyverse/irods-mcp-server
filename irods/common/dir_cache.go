package common

import (
	"fmt"
	"time"

	"github.com/cyverse/irods-mcp-server/irods/model"
	gocache "github.com/patrickmn/go-cache"
)

const DirListCacheTTL = 10 * time.Minute

// DirListCache caches full directory entry lists keyed by (tool, username, path).
// Pagination is applied on the cached slice so iRODS is only queried once per
// 10-minute window for the same user+path combination.
type DirListCache struct {
	cache *gocache.Cache
}

func NewDirListCache() *DirListCache {
	return &DirListCache{
		cache: gocache.New(DirListCacheTTL, DirListCacheTTL),
	}
}

func dirCacheKey(prefix, username, irodsPath string) string {
	return fmt.Sprintf("%s:%s:%s", prefix, username, irodsPath)
}

// Get returns the cached entry slice for the given key, or (nil, false) on miss.
func (c *DirListCache) Get(prefix, username, irodsPath string) ([]model.EntryWithAccess, bool) {
	key := dirCacheKey(prefix, username, irodsPath)
	val, ok := c.cache.Get(key)
	if !ok {
		return nil, false
	}
	entries, ok := val.([]model.EntryWithAccess)
	return entries, ok
}

// Set stores entry slice with the default 10-minute TTL.
func (c *DirListCache) Set(prefix, username, irodsPath string, entries []model.EntryWithAccess) {
	key := dirCacheKey(prefix, username, irodsPath)
	c.cache.Set(key, entries, gocache.DefaultExpiration)
}

// Invalidate removes the cached entry for a specific (prefix, username, path).
func (c *DirListCache) Invalidate(prefix, username, irodsPath string) {
	key := dirCacheKey(prefix, username, irodsPath)
	c.cache.Delete(key)
}
