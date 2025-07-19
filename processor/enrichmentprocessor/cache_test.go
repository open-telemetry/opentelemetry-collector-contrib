// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCache(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		TTL:     5 * time.Minute,
		MaxSize: 100,
	}

	cache := NewCache(config)
	assert.NotNil(t, cache)
	assert.Equal(t, config, cache.config)
	assert.Empty(t, cache.entries)
}

func TestCacheSetAndGet(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		TTL:     1 * time.Second,
		MaxSize: 10,
	}

	cache := NewCache(config)

	// Test setting and getting data
	data := map[string]interface{}{
		"field1": "value1",
		"field2": "value2",
	}

	cache.Set("test-key", data)

	retrieved, found := cache.Get("test-key")
	assert.True(t, found)
	assert.Equal(t, data, retrieved)

	// Test cache miss
	_, found = cache.Get("nonexistent-key")
	assert.False(t, found)
}

func TestCacheDisabled(t *testing.T) {
	config := CacheConfig{
		Enabled: false,
		TTL:     5 * time.Minute,
		MaxSize: 10,
	}

	cache := NewCache(config)

	data := map[string]interface{}{
		"field1": "value1",
	}

	// Setting should do nothing when disabled
	cache.Set("test-key", data)

	// Getting should always return false when disabled
	_, found := cache.Get("test-key")
	assert.False(t, found)
}

func TestCacheTTLExpiration(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		TTL:     100 * time.Millisecond,
		MaxSize: 10,
	}

	cache := NewCache(config)

	data := map[string]interface{}{
		"field1": "value1",
	}

	cache.Set("test-key", data)

	// Should be found immediately
	_, found := cache.Get("test-key")
	assert.True(t, found)

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should not be found after expiration
	_, found = cache.Get("test-key")
	assert.False(t, found)
}

func TestCacheMaxSizeEviction(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		TTL:     1 * time.Hour, // Long TTL to test size-based eviction
		MaxSize: 2,
	}

	cache := NewCache(config)

	data1 := map[string]interface{}{"field": "value1"}
	data2 := map[string]interface{}{"field": "value2"}
	data3 := map[string]interface{}{"field": "value3"}

	// Add entries up to max size
	cache.Set("key1", data1)
	cache.Set("key2", data2)

	assert.Equal(t, 2, cache.Size())

	// Both should be retrievable
	_, found := cache.Get("key1")
	assert.True(t, found)
	_, found = cache.Get("key2")
	assert.True(t, found)

	// Add one more entry, should evict oldest
	cache.Set("key3", data3)

	assert.Equal(t, 2, cache.Size())

	// key3 should be found
	_, found = cache.Get("key3")
	assert.True(t, found)

	// One of the original keys should be evicted
	key1Found := false
	key2Found := false

	if _, found := cache.Get("key1"); found {
		key1Found = true
	}
	if _, found := cache.Get("key2"); found {
		key2Found = true
	}

	// Only one of the original keys should remain
	assert.True(t, key1Found != key2Found, "Exactly one of the original keys should remain")
}

func TestCacheCleanup(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		TTL:     50 * time.Millisecond,
		MaxSize: 10,
	}

	cache := NewCache(config)

	// Add some entries
	for i := 0; i < 5; i++ {
		data := map[string]interface{}{
			"field": i,
		}
		cache.Set(fmt.Sprintf("key%d", i), data)
	}

	assert.Equal(t, 5, cache.Size())

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Run cleanup
	cache.Cleanup()

	// All entries should be removed
	assert.Equal(t, 0, cache.Size())
}

func TestCacheClear(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		TTL:     1 * time.Hour,
		MaxSize: 10,
	}

	cache := NewCache(config)

	// Add some entries
	for i := 0; i < 3; i++ {
		data := map[string]interface{}{
			"field": i,
		}
		cache.Set(fmt.Sprintf("key%d", i), data)
	}

	assert.Equal(t, 3, cache.Size())

	// Clear cache
	cache.Clear()

	// Should be empty
	assert.Equal(t, 0, cache.Size())

	// Entries should not be found
	_, found := cache.Get("key0")
	assert.False(t, found)
}

func TestCacheSize(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		TTL:     1 * time.Hour,
		MaxSize: 10,
	}

	cache := NewCache(config)

	assert.Equal(t, 0, cache.Size())

	data := map[string]interface{}{"field": "value"}

	cache.Set("key1", data)
	assert.Equal(t, 1, cache.Size())

	cache.Set("key2", data)
	assert.Equal(t, 2, cache.Size())

	cache.Set("key1", data) // Update existing key
	assert.Equal(t, 2, cache.Size())
}

func TestCacheThreadSafety(t *testing.T) {
	config := CacheConfig{
		Enabled: true,
		TTL:     1 * time.Hour,
		MaxSize: 100,
	}

	cache := NewCache(config)

	// Run concurrent operations
	const numGoroutines = 10
	const numOperations = 100

	done := make(chan bool, numGoroutines)

	// Start multiple goroutines that read and write to cache
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				data := map[string]interface{}{
					"goroutine": id,
					"operation": j,
				}

				// Set data
				cache.Set(key, data)

				// Try to get data
				retrieved, found := cache.Get(key)
				if found {
					assert.Equal(t, data, retrieved)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Cache should have some entries (exact number depends on timing and eviction)
	assert.True(t, cache.Size() > 0)
}

func TestCacheEntry(t *testing.T) {
	// Test CacheEntry structure
	data := map[string]interface{}{
		"field1": "value1",
		"field2": 42,
	}

	expiresAt := time.Now().Add(1 * time.Minute)

	entry := &CacheEntry{
		Data:      data,
		ExpiresAt: expiresAt,
	}

	assert.Equal(t, data, entry.Data)
	assert.Equal(t, expiresAt, entry.ExpiresAt)
}
