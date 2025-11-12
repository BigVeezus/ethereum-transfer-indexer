package ethereum

import (
	"sync"
	"time"
)

// BlockHeaderCache provides in-memory caching of block timestamps
// Reduces redundant RPC calls when processing multiple logs from the same block
type BlockHeaderCache struct {
	cache map[uint64]cachedHeader
	mu    sync.RWMutex
	ttl   time.Duration
}

type cachedHeader struct {
	timestamp time.Time
	expiresAt time.Time
}

// NewBlockHeaderCache creates a new block header cache with specified TTL
// TTL should be long enough to cover typical batch processing (5 minutes is safe)
func NewBlockHeaderCache(ttl time.Duration) *BlockHeaderCache {
	return &BlockHeaderCache{
		cache: make(map[uint64]cachedHeader),
		ttl:   ttl,
	}
}

// Get retrieves a cached block timestamp
// Returns timestamp and true if found and not expired, false otherwise
func (c *BlockHeaderCache) Get(blockNumber uint64) (time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.cache[blockNumber]
	if !exists {
		return time.Time{}, false
	}

	// Check if expired
	if time.Now().After(cached.expiresAt) {
		// Expired - remove it (async cleanup)
		go c.Delete(blockNumber)
		return time.Time{}, false
	}

	return cached.timestamp, true
}

// Set stores a block timestamp in cache with TTL
func (c *BlockHeaderCache) Set(blockNumber uint64, timestamp time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[blockNumber] = cachedHeader{
		timestamp: timestamp,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes a block from cache
func (c *BlockHeaderCache) Delete(blockNumber uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, blockNumber)
}

// Clear removes all entries (useful for testing or memory management)
func (c *BlockHeaderCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[uint64]cachedHeader)
}

// Size returns the number of cached entries (for monitoring)
func (c *BlockHeaderCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}
