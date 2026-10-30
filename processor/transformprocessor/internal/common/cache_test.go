// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/transformconfig"
)

func TestLoadContextCache(t *testing.T) {
	t.Run("returns nil when sharedCache is false", func(t *testing.T) {
		cache := newCacheWithContexts([]transformconfig.ContextID{transformconfig.Resource, transformconfig.Scope, transformconfig.Log})
		result := LoadContextCache(cache, transformconfig.Resource, false)
		assert.Nil(t, result)
	})

	t.Run("returns same map on subsequent calls for same context", func(t *testing.T) {
		cache := newCacheWithContexts([]transformconfig.ContextID{transformconfig.Log})
		first := LoadContextCache(cache, transformconfig.Log, true)
		second := LoadContextCache(cache, transformconfig.Log, true)
		assert.Same(t, first, second)
	})

	t.Run("returns different maps for different contexts", func(t *testing.T) {
		cache := newCacheWithContexts([]transformconfig.ContextID{transformconfig.Resource, transformconfig.Log})
		resourceCache := LoadContextCache(cache, transformconfig.Resource, true)
		logCache := LoadContextCache(cache, transformconfig.Log, true)
		require.NotNil(t, resourceCache)
		require.NotNil(t, logCache)
		assert.NotSame(t, resourceCache, logCache)
		assert.Len(t, cache, 2)
	})

	t.Run("writes are visible through subsequent lookups", func(t *testing.T) {
		cache := newCacheWithContexts([]transformconfig.ContextID{transformconfig.Scope})
		first := LoadContextCache(cache, transformconfig.Scope, true)
		first.PutStr("key", "value")

		second := LoadContextCache(cache, transformconfig.Scope, true)
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
		result := NewSharedCaches([]transformconfig.ContextID{})
		assert.Nil(t, result)
	})

	t.Run("one ID returns a one-entry map with a usable empty map behind it", func(t *testing.T) {
		result := NewSharedCaches([]transformconfig.ContextID{transformconfig.Resource})
		require.Len(t, result, 1)
		require.NotNil(t, result[transformconfig.Resource])
		assert.Equal(t, 0, result[transformconfig.Resource].Len())
		result[transformconfig.Resource].PutStr("key", "value")
		val, ok := result[transformconfig.Resource].Get("key")
		require.True(t, ok)
		assert.Equal(t, "value", val.Str())
	})

	t.Run("multiple IDs return independent maps", func(t *testing.T) {
		result := NewSharedCaches([]transformconfig.ContextID{transformconfig.Resource, transformconfig.Log})
		require.Len(t, result, 2)
		result[transformconfig.Resource].PutStr("key", "value")
		assert.Equal(t, 0, result[transformconfig.Log].Len())
	})

	t.Run("two calls return distinct maps", func(t *testing.T) {
		first := NewSharedCaches([]transformconfig.ContextID{transformconfig.Resource})
		first[transformconfig.Resource].PutStr("key", "value")

		second := NewSharedCaches([]transformconfig.ContextID{transformconfig.Resource})
		assert.Equal(t, 0, second[transformconfig.Resource].Len())
	})
}

func newCacheWithContexts(contexts []transformconfig.ContextID) map[transformconfig.ContextID]*pcommon.Map {
	cache := make(map[transformconfig.ContextID]*pcommon.Map)
	for _, context := range contexts {
		m := pcommon.NewMap()
		cache[context] = &m
	}
	return cache
}
