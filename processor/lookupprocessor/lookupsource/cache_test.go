// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCache(t *testing.T) {
	cfg := CacheConfig{
		Enabled: true,
		Size:    100,
		TTL:     5 * time.Minute,
	}
	cache := NewCache(cfg)
	require.NotNil(t, cache)
	assert.Equal(t, 0, cache.Size())
}

func TestNewCache_InvalidSizeDoesNotStoreEntries(t *testing.T) {
	cache := NewCache(CacheConfig{Enabled: true, Size: 0})
	require.NotNil(t, cache)
	cache.set("key", "value", true)
	assert.Equal(t, 0, cache.Size())
}

func TestCacheConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     CacheConfig
		wantErr bool
	}{
		{
			name: "disabled cache with zero size is valid",
			cfg: CacheConfig{
				Enabled: false,
				Size:    0,
			},
			wantErr: false,
		},
		{
			name: "enabled cache requires positive size",
			cfg: CacheConfig{
				Enabled: true,
				Size:    0,
			},
			wantErr: true,
		},
		{
			name: "negative ttl is invalid",
			cfg: CacheConfig{
				Enabled: true,
				Size:    1,
				TTL:     -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "negative ttl for not found is invalid",
			cfg: CacheConfig{
				Enabled:     true,
				Size:        1,
				NegativeTTL: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "enabled cache with valid config passes",
			cfg: CacheConfig{
				Enabled:     true,
				Size:        100,
				TTL:         5 * time.Minute,
				NegativeTTL: 1 * time.Minute,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCacheBasicOperations(t *testing.T) {
	cache := NewCache(CacheConfig{Enabled: true, Size: 100})

	val, lookupFound, cacheHit := cache.get("key")
	assert.False(t, cacheHit)
	assert.False(t, lookupFound)
	assert.Nil(t, val)

	cache.set("key", "value", true)
	val, lookupFound, cacheHit = cache.get("key")
	assert.True(t, cacheHit)
	assert.True(t, lookupFound)
	assert.Equal(t, "value", val)

	assert.Equal(t, 1, cache.Size())

	cache.Clear()
	assert.Equal(t, 0, cache.Size())
	val, lookupFound, cacheHit = cache.get("key")
	assert.False(t, cacheHit)
	assert.False(t, lookupFound)
	assert.Nil(t, val)
}

func TestCacheLRUEviction(t *testing.T) {
	cache := NewCache(CacheConfig{Enabled: true, Size: 3})

	// Fill cache
	cache.set("key1", "value1", true)
	cache.set("key2", "value2", true)
	cache.set("key3", "value3", true)
	assert.Equal(t, 3, cache.Size())

	// Add one more, should evict oldest (key1)
	cache.set("key4", "value4", true)
	assert.Equal(t, 3, cache.Size())

	// key1 should be evicted
	_, _, cacheHit := cache.get("key1")
	assert.False(t, cacheHit)

	// key2, key3, key4 should still be there
	val, lookupFound, cacheHit := cache.get("key2")
	assert.True(t, cacheHit)
	assert.True(t, lookupFound)
	assert.Equal(t, "value2", val)

	val, lookupFound, cacheHit = cache.get("key3")
	assert.True(t, cacheHit)
	assert.True(t, lookupFound)
	assert.Equal(t, "value3", val)

	val, lookupFound, cacheHit = cache.get("key4")
	assert.True(t, cacheHit)
	assert.True(t, lookupFound)
	assert.Equal(t, "value4", val)
}

func TestCacheTTLExpiration(t *testing.T) {
	cache := NewCache(CacheConfig{
		Enabled: true,
		Size:    100,
		TTL:     50 * time.Millisecond,
	})

	cache.set("key", "value", true)

	// Should be found
	val, lookupFound, cacheHit := cache.get("key")
	assert.True(t, cacheHit)
	assert.True(t, lookupFound)
	assert.Equal(t, "value", val)

	time.Sleep(100 * time.Millisecond)

	// Should be expired
	val, lookupFound, cacheHit = cache.get("key")
	assert.False(t, cacheHit)
	assert.False(t, lookupFound)
	assert.Nil(t, val)
}

func TestCacheNegativeCaching(t *testing.T) {
	cache := NewCache(CacheConfig{
		Enabled:     true,
		Size:        100,
		TTL:         5 * time.Minute,
		NegativeTTL: 50 * time.Millisecond,
	})

	// Set a negative cache entry (not found)
	cache.set("missing", nil, false)

	// Should be cached
	val, lookupFound, cacheHit := cache.get("missing")
	assert.True(t, cacheHit)
	assert.False(t, lookupFound)
	assert.Nil(t, val)

	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, _, cacheHit = cache.get("missing")
	assert.False(t, cacheHit)
}

func TestCacheNegativeCachingDisabled(t *testing.T) {
	cache := NewCache(CacheConfig{
		Enabled:     true,
		Size:        100,
		TTL:         5 * time.Minute,
		NegativeTTL: 0, // Disabled
	})

	// Try to set a negative cache entry
	cache.set("missing", nil, false)

	// Should not be cached
	_, _, cacheHit := cache.get("missing")
	assert.False(t, cacheHit)
}

func TestWrapWithCacheDisabled(t *testing.T) {
	lookupCount := 0
	baseFn := func(_ context.Context, key string) (any, bool, error) {
		lookupCount++
		return "value-" + key, true, nil
	}

	// Disabled cache should not wrap
	cache := NewCache(CacheConfig{Enabled: false})
	wrappedFn := WrapWithCache(cache, baseFn)

	// Multiple calls should all hit the base function
	_, _, _ = wrappedFn(t.Context(), "key1")
	_, _, _ = wrappedFn(t.Context(), "key1")
	_, _, _ = wrappedFn(t.Context(), "key1")

	assert.Equal(t, 3, lookupCount)
}

func TestWrapWithCacheNil(t *testing.T) {
	lookupCount := 0
	baseFn := func(_ context.Context, key string) (any, bool, error) {
		lookupCount++
		return "value-" + key, true, nil
	}

	// Nil cache should not wrap
	wrappedFn := WrapWithCache(nil, baseFn)

	val, found, err := wrappedFn(t.Context(), "key1")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "value-key1", val)
	assert.Equal(t, 1, lookupCount)
}

func TestWrapWithCacheEnabled(t *testing.T) {
	lookupCount := 0
	baseFn := func(_ context.Context, key string) (any, bool, error) {
		lookupCount++
		return "value-" + key, true, nil
	}

	cache := NewCache(CacheConfig{Enabled: true, Size: 100})
	wrappedFn := WrapWithCache(cache, baseFn)

	// First call should hit base function
	val, found, err := wrappedFn(t.Context(), "key1")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "value-key1", val)
	assert.Equal(t, 1, lookupCount)

	// Second call should hit cache
	val, found, err = wrappedFn(t.Context(), "key1")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "value-key1", val)
	assert.Equal(t, 1, lookupCount) // cache hit
}

func TestWrapWithCacheNegativeResults(t *testing.T) {
	lookupCount := 0
	baseFn := func(_ context.Context, _ string) (any, bool, error) {
		lookupCount++
		// Simulate not found
		return nil, false, nil
	}

	cache := NewCache(CacheConfig{
		Enabled:     true,
		Size:        100,
		TTL:         5 * time.Minute,
		NegativeTTL: 5 * time.Minute,
	})
	wrappedFn := WrapWithCache(cache, baseFn)

	// First call - lookup returns not found
	val, found, err := wrappedFn(t.Context(), "missing")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, val)
	assert.Equal(t, 1, lookupCount)

	// Second call - should return cached "not found"
	val, found, err = wrappedFn(t.Context(), "missing")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, val)
	assert.Equal(t, 1, lookupCount) // No additional lookup
}
