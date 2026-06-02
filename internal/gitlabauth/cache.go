package gitlabauth

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// cacheEntry records a resolved token outcome. valid distinguishes a confirmed
// identity (username set) from a confirmed-bad token (negative cache).
type cacheEntry struct {
	username string
	valid    bool
	expiry   time.Time
}

// tokenCache maps a hash of a raw token to its last resolved outcome, with
// separate TTLs for successful and rejected validations. Raw tokens are never
// stored: keys are sha256 hex digests so a memory dump cannot leak credentials.
// Expiry is enforced lazily on read.
type tokenCache struct {
	mu          sync.RWMutex
	entries     map[string]cacheEntry
	positiveTTL time.Duration
	negativeTTL time.Duration
	now         func() time.Time
}

func newTokenCache(positiveTTL, negativeTTL time.Duration) *tokenCache {
	return &tokenCache{
		entries:     make(map[string]cacheEntry),
		positiveTTL: positiveTTL,
		negativeTTL: negativeTTL,
		now:         time.Now,
	}
}

func cacheKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// get returns the cached entry for token if present and unexpired.
func (c *tokenCache) get(token string) (cacheEntry, bool) {
	key := cacheKey(token)
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || !c.now().Before(e.expiry) {
		return cacheEntry{}, false
	}
	return e, true
}

// putValid caches a confirmed identity for positiveTTL.
func (c *tokenCache) putValid(token, username string) {
	if c.positiveTTL <= 0 {
		return
	}
	c.store(token, cacheEntry{username: username, valid: true, expiry: c.now().Add(c.positiveTTL)})
}

// putInvalid caches a confirmed-bad token for negativeTTL.
func (c *tokenCache) putInvalid(token string) {
	if c.negativeTTL <= 0 {
		return
	}
	c.store(token, cacheEntry{valid: false, expiry: c.now().Add(c.negativeTTL)})
}

func (c *tokenCache) store(token string, e cacheEntry) {
	key := cacheKey(token)
	c.mu.Lock()
	c.entries[key] = e
	c.mu.Unlock()
}
