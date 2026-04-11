package smbfs

import (
	"io/fs"
	"sync"
	"time"
)

// CacheConfig configures the metadata cache behavior.
type CacheConfig struct {
	// EnableCache enables metadata caching. Default: false for safety.
	EnableCache bool

	// DirCacheTTL is the time-to-live for directory listings.
	// Default: 5 seconds. Set to 0 to disable directory caching.
	DirCacheTTL time.Duration

	// StatCacheTTL is the time-to-live for file stat results.
	// Default: 5 seconds. Set to 0 to disable stat caching.
	StatCacheTTL time.Duration

	// MaxCacheEntries is the maximum number of cache entries.
	// When exceeded, oldest entries are evicted. Default: 1000.
	MaxCacheEntries int
}

// DefaultCacheConfig returns a cache configuration with reasonable defaults.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		EnableCache:     false, // Disabled by default for consistency
		DirCacheTTL:     5 * time.Second,
		StatCacheTTL:    5 * time.Second,
		MaxCacheEntries: 1000,
	}
}

// metadataCache provides caching for directory listings and file stats.
// This significantly improves performance for repeated metadata operations.
type metadataCache struct {
	mu            sync.RWMutex
	config        CacheConfig
	dirCache      map[string]*dirCacheEntry
	statCache     map[string]*statCacheEntry
	accessOrder   []string // LRU tracking
	enabled       bool
}

type dirCacheEntry struct {
	entries  []fs.DirEntry
	cachedAt time.Time
}

type statCacheEntry struct {
	info     fs.FileInfo
	cachedAt time.Time
}

// newMetadataCache creates a new metadata cache with the given configuration.
func newMetadataCache(config CacheConfig) *metadataCache {
	if config.MaxCacheEntries == 0 {
		config.MaxCacheEntries = 1000
	}
	if config.DirCacheTTL == 0 {
		config.DirCacheTTL = 5 * time.Second
	}
	if config.StatCacheTTL == 0 {
		config.StatCacheTTL = 5 * time.Second
	}

	return &metadataCache{
		config:      config,
		dirCache:    make(map[string]*dirCacheEntry),
		statCache:   make(map[string]*statCacheEntry),
		accessOrder: make([]string, 0, config.MaxCacheEntries),
		enabled:     config.EnableCache,
	}
}

// getDirEntries retrieves cached directory entries if available and not expired.
func (c *metadataCache) getDirEntries(path string) ([]fs.DirEntry, bool) {
	if !c.enabled || c.config.DirCacheTTL == 0 {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.dirCache[path]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Since(entry.cachedAt) > c.config.DirCacheTTL {
		return nil, false
	}

	return entry.entries, true
}

// putDirEntries stores directory entries in the cache.
func (c *metadataCache) putDirEntries(path string, entries []fs.DirEntry) {
	if !c.enabled || c.config.DirCacheTTL == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.dirCache[path] = &dirCacheEntry{
		entries:  entries,
		cachedAt: time.Now(),
	}

	c.trackAccess(path)
	c.evictIfNeeded()
}

// getStatInfo retrieves cached file info if available and not expired.
func (c *metadataCache) getStatInfo(path string) (fs.FileInfo, bool) {
	if !c.enabled || c.config.StatCacheTTL == 0 {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.statCache[path]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Since(entry.cachedAt) > c.config.StatCacheTTL {
		return nil, false
	}

	return entry.info, true
}

// putStatInfo stores file info in the cache.
func (c *metadataCache) putStatInfo(path string, info fs.FileInfo) {
	if !c.enabled || c.config.StatCacheTTL == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.statCache[path] = &statCacheEntry{
		info:     info,
		cachedAt: time.Now(),
	}

	c.trackAccess(path)
	c.evictIfNeeded()
}

// invalidate removes cache entries for a specific path and its parent directory.
// This should be called after any write operation.
func (c *metadataCache) invalidate(path string) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Invalidate the path itself
	delete(c.dirCache, path)
	delete(c.statCache, path)

	// Invalidate parent directory (since its listing has changed)
	parentPath := c.getParentPath(path)
	delete(c.dirCache, parentPath)
}

// invalidateAll clears all cache entries.
func (c *metadataCache) invalidateAll() {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.dirCache = make(map[string]*dirCacheEntry)
	c.statCache = make(map[string]*statCacheEntry)
	c.accessOrder = c.accessOrder[:0]
}

// trackAccess tracks access order for LRU eviction.
func (c *metadataCache) trackAccess(path string) {
	// Remove if exists
	for i, p := range c.accessOrder {
		if p == path {
			c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
			break
		}
	}

	// Add to end (most recently used)
	c.accessOrder = append(c.accessOrder, path)
}

// evictIfNeeded evicts oldest entries if cache is full.
func (c *metadataCache) evictIfNeeded() {
	totalEntries := len(c.dirCache) + len(c.statCache)
	if totalEntries <= c.config.MaxCacheEntries {
		return
	}

	// Evict oldest entries until we're under the limit
	entriesToEvict := totalEntries - c.config.MaxCacheEntries
	for i := 0; i < entriesToEvict && len(c.accessOrder) > 0; i++ {
		oldestPath := c.accessOrder[0]
		c.accessOrder = c.accessOrder[1:]

		delete(c.dirCache, oldestPath)
		delete(c.statCache, oldestPath)
	}
}

// getParentPath returns the parent directory path.
func (c *metadataCache) getParentPath(path string) string {
	if path == "/" || path == "" {
		return "/"
	}

	// Find last separator
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}

	return "/"
}

// CacheStats provides statistics about cache usage.
type CacheStats struct {
	Enabled         bool
	DirCacheEntries int
	StatCacheEntries int
	TotalEntries    int
	MaxEntries      int
}

// Stats returns cache statistics.
func (c *metadataCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Enabled:          c.enabled,
		DirCacheEntries:  len(c.dirCache),
		StatCacheEntries: len(c.statCache),
		TotalEntries:     len(c.dirCache) + len(c.statCache),
		MaxEntries:       c.config.MaxCacheEntries,
	}
}
