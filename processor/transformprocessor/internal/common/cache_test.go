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

func newCacheWithContexts(contexts []ContextID) map[ContextID]*pcommon.Map {
	cache := make(map[ContextID]*pcommon.Map)
	for _, context := range contexts {
		m := pcommon.NewMap()
		cache[context] = &m
	}
	return cache
}
