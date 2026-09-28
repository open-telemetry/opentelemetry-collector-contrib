// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestLoadContextCache(t *testing.T) {
	t.Run("returns nil when sharedCache is false", func(t *testing.T) {
		cache := newCacheWithContexts([]ContextID{Resource, Scope, Log})
		result := LoadContextCache(cache, Resource, false)
		assert.Nil(t, result)
	})

	t.Run("returns same map on subsequent calls for same context", func(t *testing.T) {
		cache := newCacheWithContexts([]ContextID{Log})
		first := LoadContextCache(cache, Log, true)
		second := LoadContextCache(cache, Log, true)
		assert.Same(t, first, second)
	})

	t.Run("returns different maps for different contexts", func(t *testing.T) {
		cache := newCacheWithContexts([]ContextID{Resource, Log})
		resourceCache := LoadContextCache(cache, Resource, true)
		logCache := LoadContextCache(cache, Log, true)
		require.NotNil(t, resourceCache)
		require.NotNil(t, logCache)
		assert.NotSame(t, resourceCache, logCache)
		assert.Len(t, cache, 2)
	})

	t.Run("writes are visible through subsequent lookups", func(t *testing.T) {
		cache := newCacheWithContexts([]ContextID{Scope})
		first := LoadContextCache(cache, Scope, true)
		first.PutStr("key", "value")

		second := LoadContextCache(cache, Scope, true)
		val, ok := second.Get("key")
		require.True(t, ok)
		assert.Equal(t, "value", val.Str())
	})
}

func TestNewSharedCaches(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		result := NewSharedCaches(nil)
		assert.Nil(t, result)
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		result := NewSharedCaches([]ContextID{})
		assert.Nil(t, result)
	})

	t.Run("one ID returns a one-entry map with a usable empty map behind it", func(t *testing.T) {
		result := NewSharedCaches([]ContextID{Resource})
		require.Len(t, result, 1)
		require.NotNil(t, result[Resource])
		assert.Equal(t, 0, result[Resource].Len())
		result[Resource].PutStr("key", "value")
		val, ok := result[Resource].Get("key")
		require.True(t, ok)
		assert.Equal(t, "value", val.Str())
	})

	t.Run("multiple IDs return independent maps", func(t *testing.T) {
		result := NewSharedCaches([]ContextID{Resource, Log})
		require.Len(t, result, 2)
		result[Resource].PutStr("key", "value")
		assert.Equal(t, 0, result[Log].Len())
	})

	t.Run("two calls return distinct maps", func(t *testing.T) {
		first := NewSharedCaches([]ContextID{Resource})
		first[Resource].PutStr("key", "value")

		second := NewSharedCaches([]ContextID{Resource})
		assert.Equal(t, 0, second[Resource].Len())
	})
}

func newCacheWithContexts(contexts []ContextID) map[ContextID]*pcommon.Map {
	cache := make(map[ContextID]*pcommon.Map)
	for _, context := range contexts {
		m := pcommon.NewMap()
		cache[context] = &m
	}
	return cache
}
