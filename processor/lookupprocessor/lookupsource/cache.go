// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"

import (
	"container/list"
	"context"
	"errors"
	"sync"
	"time"
)

type CacheConfig struct {
	Enabled bool `mapstructure:"enabled"`

	Size int `mapstructure:"size"`

	TTL time.Duration `mapstructure:"ttl"`

	// NegativeTTL is the time-to-live for negative cache entries (not found results).
	// Set to 0 to disable negative caching.
	// Default: 0 (disabled)
	NegativeTTL time.Duration `mapstructure:"negative_ttl"`
}

func (c CacheConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Size <= 0 {
		return errors.New("cache.size must be greater than 0 when cache is enabled")
	}
	if c.TTL < 0 {
		return errors.New("cache.ttl cannot be negative")
	}
	if c.NegativeTTL < 0 {
		return errors.New("cache.negative_ttl cannot be negative")
	}
	return nil
}

type cacheEntry struct {
	key       string
	value     any
	found     bool
	expiresAt time.Time
	element   *list.Element
}

func (e *cacheEntry) isExpired() bool {
	if e.expiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.expiresAt)
}

type Cache struct {
	config  CacheConfig
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	order   *list.List
}

func NewCache(cfg CacheConfig) *Cache {
	return &Cache{
		config:  cfg,
		entries: make(map[string]*cacheEntry, cfg.Size),
		order:   list.New(),
	}
}

// get retrieves a value from the cache and indicates whether the original lookup found a value.
// Returns (value, lookupFound, cacheHit).
func (c *Cache) get(key string) (any, bool, bool) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		return nil, false, false
	}

	if !entry.isExpired() {
		return entry.value, entry.found, true
	}

	c.mu.Lock()
	// Re-check after acquiring write lock
	if entry, exists = c.entries[key]; exists && entry.isExpired() {
		c.removeEntryLocked(entry)
	}
	c.mu.Unlock()
	return nil, false, false
}

// set adds or updates a value in the cache.
func (c *Cache) set(key string, value any, found bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Invalid cache size means this cache cannot store entries.
	// Config validation should reject this, but keep runtime behavior safe.
	if c.config.Size <= 0 {
		return
	}

	ttl := c.config.TTL
	if !found {
		if c.config.NegativeTTL == 0 {
			return
		}
		ttl = c.config.NegativeTTL
	}

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	if existing, exists := c.entries[key]; exists {
		existing.value = value
		existing.found = found
		existing.expiresAt = expiresAt
		// MRU: move to back (most recently used)
		c.order.MoveToBack(existing.element)
		return
	}

	// Evict oldest entry if at capacity
	for len(c.entries) >= c.config.Size && c.order.Len() > 0 {
		oldest := c.order.Front()
		c.removeEntryLocked(oldest.Value.(*cacheEntry))
	}

	entry := &cacheEntry{
		key:       key,
		value:     value,
		found:     found,
		expiresAt: expiresAt,
	}
	entry.element = c.order.PushBack(entry)
	c.entries[key] = entry
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
	c.order.Init()
}

func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// removeEntryLocked removes an entry from the cache.
// Must be called with the write lock held.
func (c *Cache) removeEntryLocked(entry *cacheEntry) {
	c.order.Remove(entry.element)
	delete(c.entries, entry.key)
}

// WrapWithCache wraps a lookup function with caching.
//
// The cache supports:
//   - LRU eviction when max size is reached
//   - TTL-based expiration for positive results
//   - Negative caching (caching "not found" results) with separate TTL
//
// Example:
//
//	cache := lookupsource.NewCache(lookupsource.CacheConfig{
//	    Enabled:     true,
//	    Size:        1000,
//	    TTL:         5 * time.Minute,
//	    NegativeTTL: 1 * time.Minute,
//	})
//	cachedLookup := lookupsource.WrapWithCache(cache, myLookupFunc)
func WrapWithCache(cache *Cache, fn LookupFunc) LookupFunc {
	if cache == nil || !cache.config.Enabled {
		return fn
	}
	return func(ctx context.Context, key string) (any, bool, error) {
		if val, lookupFound, cacheHit := cache.get(key); cacheHit {
			return val, lookupFound, nil
		}

		val, found, err := fn(ctx, key)
		if err != nil {
			return nil, false, err
		}

		cache.set(key, val, found)
		return val, found, nil
	}
}
