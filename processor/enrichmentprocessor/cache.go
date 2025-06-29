// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"sync"
	"time"
)

// CacheEntry represents a cached entry
type CacheEntry struct {
	Data      map[string]interface{}
	ExpiresAt time.Time
}

// Cache provides caching functionality for enrichment data
type Cache struct {
	config  CacheConfig
	entries map[string]*CacheEntry
	mutex   sync.RWMutex
}

// NewCache creates a new cache
func NewCache(config CacheConfig) *Cache {
	return &Cache{
		config:  config,
		entries: make(map[string]*CacheEntry),
	}
}

// Get retrieves a value from the cache
func (c *Cache) Get(key string) (map[string]interface{}, bool) {
	if !c.config.Enabled {
		return nil, false
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Check if entry has expired
	if time.Now().After(entry.ExpiresAt) {
		// Don't delete here to avoid lock contention, let cleanup handle it
		return nil, false
	}

	return entry.Data, true
}

// Set stores a value in the cache
func (c *Cache) Set(key string, data map[string]interface{}) {
	if !c.config.Enabled {
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if we need to make room
	if len(c.entries) >= c.config.MaxSize {
		c.evictOldest()
	}

	c.entries[key] = &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(c.config.TTL),
	}
}

// evictOldest removes the oldest entry from the cache
func (c *Cache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.ExpiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.ExpiresAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// Cleanup removes expired entries from the cache
func (c *Cache) Cleanup() {
	if !c.config.Enabled {
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
		}
	}
}

// Clear clears all entries from the cache
func (c *Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.entries = make(map[string]*CacheEntry)
}

// Size returns the current size of the cache
func (c *Cache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.entries)
}
