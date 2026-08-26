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
		cache := make(map[ContextID]*pcommon.Map)
		result := LoadContextCache(cache, Resource, false)
		assert.Nil(t, result)
		assert.Empty(t, cache)
	})

	t.Run("creates new map on first call", func(t *testing.T) {
		cache := make(map[ContextID]*pcommon.Map)
		result := LoadContextCache(cache, Resource, true)
		require.NotNil(t, result)
		assert.Len(t, cache, 1)
		assert.Equal(t, result, cache[Resource])
	})

	t.Run("returns same map on subsequent calls for same context", func(t *testing.T) {
		cache := make(map[ContextID]*pcommon.Map)
		first := LoadContextCache(cache, Log, true)
		second := LoadContextCache(cache, Log, true)
		assert.Same(t, first, second)
	})

	t.Run("returns different maps for different contexts", func(t *testing.T) {
		cache := make(map[ContextID]*pcommon.Map)
		resourceCache := LoadContextCache(cache, Resource, true)
		logCache := LoadContextCache(cache, Log, true)
		require.NotNil(t, resourceCache)
		require.NotNil(t, logCache)
		assert.NotSame(t, resourceCache, logCache)
		assert.Len(t, cache, 2)
	})

	t.Run("writes are visible through subsequent lookups", func(t *testing.T) {
		cache := make(map[ContextID]*pcommon.Map)
		first := LoadContextCache(cache, Scope, true)
		first.PutStr("key", "value")

		second := LoadContextCache(cache, Scope, true)
		val, ok := second.Get("key")
		require.True(t, ok)
		assert.Equal(t, "value", val.Str())
	})

	t.Run("sharedCache=false does not populate the cache map", func(t *testing.T) {
		cache := make(map[ContextID]*pcommon.Map)
		LoadContextCache(cache, Resource, false)
		LoadContextCache(cache, Log, false)
		assert.Empty(t, cache)
	})
}
