// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cachetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/cachetest"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
)

type Context[K interface{ Close() }, O any] struct {
	Name                 string
	PathExpressionParser ottl.PathExpressionParser[K]
	NewTransformContext  func(options ...O) K
	WithCache            func(cache *pcommon.Map) O
	LocalCache           func(tCtx K) pcommon.Map
	ExternalCache        func(tCtx K) *pcommon.Map
}

func TestWithCache[K interface{ Close() }, O any](t *testing.T, c Context[K, O]) {
	for name, pathContext := range map[string]string{
		"without_path_context": "",
		"with_path_context":    c.Name,
	} {
		t.Run(name, func(t *testing.T) {
			testWithCache(t, c, pathContext)
		})
	}
}

func testWithCache[K interface{ Close() }, O any](t *testing.T, c Context[K, O], pathContext string) {
	cachePath := func(keys ...string) ottl.GetSetter[K] {
		path := &pathtest.Path[K]{C: pathContext, N: "cache"}
		for _, key := range keys {
			path.KeySlice = append(path.KeySlice, &pathtest.Key[K]{S: new(key)})
		}
		accessor, err := c.PathExpressionParser(path)
		require.NoError(t, err)
		return accessor
	}
	wholeCache := cachePath()
	cacheKey := cachePath("key")

	t.Run("reads caller cache", func(t *testing.T) {
		cache := pcommon.NewMap()
		cache.PutStr("key", "value")
		tCtx := c.NewTransformContext(c.WithCache(&cache))
		defer tCtx.Close()

		got, err := cacheKey.Get(t.Context(), tCtx)
		require.NoError(t, err)
		assert.Equal(t, "value", got)

		got, err = wholeCache.Get(t.Context(), tCtx)
		require.NoError(t, err)
		require.IsType(t, pcommon.Map{}, got)
		got.(pcommon.Map).PutStr("other", "value")
		assert.Equal(t, map[string]any{"key": "value", "other": "value"}, cache.AsRaw())
	})

	t.Run("writes to caller cache", func(t *testing.T) {
		cache := pcommon.NewMap()
		tCtx := c.NewTransformContext(c.WithCache(&cache))
		defer tCtx.Close()

		require.NoError(t, cacheKey.Set(t.Context(), tCtx, "value"))
		assert.Equal(t, map[string]any{"key": "value"}, cache.AsRaw())
		assert.Zero(t, c.LocalCache(tCtx).Len())
	})

	t.Run("replaces caller cache contents", func(t *testing.T) {
		cache := pcommon.NewMap()
		cache.PutStr("old", "value")
		tCtx := c.NewTransformContext(c.WithCache(&cache))
		defer tCtx.Close()

		newCache := pcommon.NewMap()
		newCache.PutStr("new", "value")
		require.NoError(t, wholeCache.Set(t.Context(), tCtx, newCache))
		assert.Equal(t, map[string]any{"new": "value"}, cache.AsRaw())
		assert.Zero(t, c.LocalCache(tCtx).Len())
	})

	t.Run("shared between open contexts", func(t *testing.T) {
		cache := pcommon.NewMap()
		first := c.NewTransformContext(c.WithCache(&cache))
		defer first.Close()
		second := c.NewTransformContext(c.WithCache(&cache))
		defer second.Close()

		require.NoError(t, cacheKey.Set(t.Context(), first, "first"))
		got, err := cacheKey.Get(t.Context(), second)
		require.NoError(t, err)
		assert.Equal(t, "first", got)

		require.NoError(t, cacheKey.Set(t.Context(), second, "second"))
		got, err = cacheKey.Get(t.Context(), first)
		require.NoError(t, err)
		assert.Equal(t, "second", got)
	})

	t.Run("close keeps caller cache and detaches it", func(t *testing.T) {
		cache := pcommon.NewMap()
		tCtx := c.NewTransformContext(c.WithCache(&cache))
		require.NoError(t, cacheKey.Set(t.Context(), tCtx, "value"))
		tCtx.Close()

		assert.Nil(t, c.ExternalCache(tCtx))
		assert.Equal(t, map[string]any{"key": "value"}, cache.AsRaw())

		next := c.NewTransformContext(c.WithCache(&cache))
		defer next.Close()
		got, err := cacheKey.Get(t.Context(), next)
		require.NoError(t, err)
		assert.Equal(t, "value", got)
	})

	t.Run("separate caches are isolated", func(t *testing.T) {
		firstCache := pcommon.NewMap()
		first := c.NewTransformContext(c.WithCache(&firstCache))
		defer first.Close()
		secondCache := pcommon.NewMap()
		second := c.NewTransformContext(c.WithCache(&secondCache))
		defer second.Close()

		require.NoError(t, cacheKey.Set(t.Context(), first, "first"))
		require.NoError(t, cacheKey.Set(t.Context(), second, "second"))
		assert.Equal(t, map[string]any{"key": "first"}, firstCache.AsRaw())
		assert.Equal(t, map[string]any{"key": "second"}, secondCache.AsRaw())
	})

	t.Run("not shared with contexts without cache", func(t *testing.T) {
		cache := pcommon.NewMap()
		shared := c.NewTransformContext(c.WithCache(&cache))
		defer shared.Close()
		require.NoError(t, cacheKey.Set(t.Context(), shared, "shared"))

		local := c.NewTransformContext()
		defer local.Close()
		got, err := cacheKey.Get(t.Context(), local)
		require.NoError(t, err)
		assert.Nil(t, got)

		require.NoError(t, cacheKey.Set(t.Context(), local, "local"))
		assert.Equal(t, map[string]any{"key": "local"}, c.LocalCache(local).AsRaw())
		assert.Equal(t, map[string]any{"key": "shared"}, cache.AsRaw())
	})

	t.Run("nil cache uses context cache", func(t *testing.T) {
		tCtx := c.NewTransformContext(c.WithCache(nil))
		defer tCtx.Close()
		assert.Nil(t, c.ExternalCache(tCtx))

		require.NoError(t, cacheKey.Set(t.Context(), tCtx, "value"))
		assert.Equal(t, map[string]any{"key": "value"}, c.LocalCache(tCtx).AsRaw())
	})

	t.Run("last cache wins", func(t *testing.T) {
		firstCache := pcommon.NewMap()
		lastCache := pcommon.NewMap()
		tCtx := c.NewTransformContext(c.WithCache(&firstCache), c.WithCache(&lastCache))
		defer tCtx.Close()

		require.NoError(t, cacheKey.Set(t.Context(), tCtx, "value"))
		assert.Zero(t, firstCache.Len())
		assert.Equal(t, map[string]any{"key": "value"}, lastCache.AsRaw())
	})

	t.Run("nil cache keeps earlier cache", func(t *testing.T) {
		cache := pcommon.NewMap()
		tCtx := c.NewTransformContext(c.WithCache(&cache), c.WithCache(nil))
		defer tCtx.Close()

		require.NoError(t, cacheKey.Set(t.Context(), tCtx, "value"))
		assert.Equal(t, map[string]any{"key": "value"}, cache.AsRaw())
	})
}
