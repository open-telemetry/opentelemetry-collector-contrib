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
}

func TestCacheStub(t *testing.T) {
	// The cache is a stub for now (full implementation forthcoming)
	cache := NewCache(CacheConfig{Enabled: true, Size: 100})

	// Get always returns not found (stub)
	val, found := cache.Get("key")
	assert.False(t, found)
	assert.Nil(t, val)

	// Set is a no-op (stub)
	cache.Set("key", "value")

	// Still not found after Set (stub behavior)
	val, found = cache.Get("key")
	assert.False(t, found)
	assert.Nil(t, val)

	// Clear is a no-op (stub)
	cache.Clear()
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
	// Note: Since cache is a stub, this test verifies the wrapper logic
	// but cache.Get always returns false and cache.Set is a no-op

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

	// Second call also hits base function (cache is stub)
	val, found, err = wrappedFn(t.Context(), "key1")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "value-key1", val)
	assert.Equal(t, 2, lookupCount) // Would be 1 with real cache
}
