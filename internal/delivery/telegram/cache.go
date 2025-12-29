package telegram

import (
	"context"
	"crypto/md5"
	"fmt"
	"sync"
	"time"
)

// cachedResponse represents a cached AI response
type cachedResponse struct {
	response  string
	timestamp time.Time
}

// responseCache manages cached responses
type responseCache struct {
	cache   map[string]cachedResponse
	mu      sync.RWMutex
	ttl     time.Duration
	maxSize int

	// Statistics
	hits   int64
	misses int64
}

const (
	defaultCacheTTL     = 5 * time.Minute
	defaultMaxCacheSize = 1000
)

// newResponseCache creates a new response cache
func newResponseCache(ttl time.Duration, maxSize int) *responseCache {
	if ttl == 0 {
		ttl = defaultCacheTTL
	}
	if maxSize == 0 {
		maxSize = defaultMaxCacheSize
	}

	rc := &responseCache{
		cache:   make(map[string]cachedResponse),
		ttl:     ttl,
		maxSize: maxSize,
	}

	return rc
}

// get retrieves a cached response if it exists and is valid
func (rc *responseCache) get(key string) (string, bool) {
	rc.mu.RLock()
	cached, exists := rc.cache[key]
	rc.mu.RUnlock()

	if !exists {
		rc.mu.Lock()
		rc.misses++
		rc.mu.Unlock()
		return "", false
	}

	// Check if expired
	if time.Since(cached.timestamp) > rc.ttl {
		rc.mu.Lock()
		delete(rc.cache, key)
		rc.misses++
		rc.mu.Unlock()
		return "", false
	}

	rc.mu.Lock()
	rc.hits++
	rc.mu.Unlock()

	return cached.response, true
}

// set stores a response in the cache
func (rc *responseCache) set(key, response string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Check cache size limit
	if len(rc.cache) >= rc.maxSize {
		// Simple LRU: remove oldest entry
		var oldestKey string
		var oldestTime time.Time
		first := true

		for k, v := range rc.cache {
			if first || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
				first = false
			}
		}

		if oldestKey != "" {
			delete(rc.cache, oldestKey)
		}
	}

	rc.cache[key] = cachedResponse{
		response:  response,
		timestamp: time.Now(),
	}
}

// cleanup removes expired entries
func (rc *responseCache) cleanup(ctx context.Context) {
	ticker := time.NewTicker(rc.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rc.mu.Lock()
			now := time.Now()
			for key, cached := range rc.cache {
				if now.Sub(cached.timestamp) > rc.ttl {
					delete(rc.cache, key)
				}
			}
			rc.mu.Unlock()
		}
	}
}

// stats returns cache statistics
func (rc *responseCache) stats() (hits, misses int64, size int) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.hits, rc.misses, len(rc.cache)
}

// clear clears all cached entries
func (rc *responseCache) clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache = make(map[string]cachedResponse)
}

// generateCacheKey generates a cache key from user ID and text
func generateCacheKey(userID int64, text string) string {
	// Use MD5 hash to create a fixed-length key
	hash := md5.Sum([]byte(fmt.Sprintf("%d:%s", userID, text)))
	return fmt.Sprintf("%x", hash)
}
