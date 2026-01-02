// Copyright observIQ, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package sidcache provides a high-performance LRU cache for Windows SID-to-name resolution.
// It uses the Windows Local Security Authority (LSA) API to resolve Security Identifiers (SIDs)
// to human-readable user and group names, with support for well-known SIDs and TTL-based expiration.
package sidcache

import (
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// Default configuration values
const (
	DefaultCacheSize = 10000
	DefaultCacheTTL  = 15 * time.Minute
)

// sidPattern validates SID format: S-1-<revision>-<authority>-<sub-authorities>
var sidPattern = regexp.MustCompile(`^S-1-\d+(-\d+)+$`)

// cacheEntry wraps a ResolvedSID with TTL information
type cacheEntry struct {
	resolved  *ResolvedSID
	expiresAt time.Time
}

// cache implements the Cache interface with LRU eviction and TTL expiration
type cache struct {
	config Config
	lru    *lru.Cache[string, *cacheEntry]

	// Statistics (using atomic for thread safety)
	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
	errors    atomic.Uint64

	mu sync.RWMutex
}

// New creates a new SID cache with the given configuration
func New(config Config) (Cache, error) {
	// Validate and set defaults
	if config.Size <= 0 {
		config.Size = DefaultCacheSize
	}
	if config.TTL <= 0 {
		config.TTL = DefaultCacheTTL
	}

	c := &cache{
		config: config,
	}

	// Create LRU cache with eviction callback
	lruCache, err := lru.NewWithEvict(config.Size, func(_ string, _ *cacheEntry) {
		c.evictions.Add(1)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	}

	c.lru = lruCache
	return c, nil
}

// Resolve looks up a SID and returns its resolved information
func (c *cache) Resolve(sid string) (*ResolvedSID, error) {
	// Validate SID format
	if !isSIDFormat(sid) {
		c.errors.Add(1)
		return nil, fmt.Errorf("invalid SID format: %s", sid)
	}

	// Check well-known SIDs first (never cached, always available)
	if resolved, ok := isWellKnownSID(sid); ok {
		c.hits.Add(1)
		return resolved, nil
	}

	// Check cache
	c.mu.RLock()
	entry, found := c.lru.Get(sid)
	c.mu.RUnlock()

	if found {
		// Check if entry has expired
		if time.Now().Before(entry.expiresAt) {
			c.hits.Add(1)
			return entry.resolved, nil
		}
		// Entry expired, remove it
		c.mu.Lock()
		c.lru.Remove(sid)
		c.mu.Unlock()
	}

	// Cache miss - resolve via Windows API
	c.misses.Add(1)
	resolved, err := lookupSID(sid)
	if err != nil {
		c.errors.Add(1)
		return nil, fmt.Errorf("failed to lookup SID %s: %w", sid, err)
	}

	// Add to cache with TTL
	c.mu.Lock()
	c.lru.Add(sid, &cacheEntry{
		resolved:  resolved,
		expiresAt: time.Now().Add(c.config.TTL),
	})
	c.mu.Unlock()

	return resolved, nil
}

// Close releases any resources held by the cache
func (c *cache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lru.Purge()
	return nil
}

// Stats returns current cache statistics
func (c *cache) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return Stats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
		Size:      c.lru.Len(),
		Errors:    c.errors.Load(),
	}
}

// isSIDFormat checks if a string matches the SID format
func isSIDFormat(sid string) bool {
	return sidPattern.MatchString(sid)
}

// IsSIDField checks if a field name likely contains a SID
// Used by the receiver to identify which fields need resolution
func IsSIDField(fieldName string) bool {
	// Common SID field patterns in Windows events
	return len(fieldName) >= 3 && (fieldName[len(fieldName)-3:] == "Sid" || // ends with "Sid"
		fieldName == "UserID") // exact match for security.user_id
}
