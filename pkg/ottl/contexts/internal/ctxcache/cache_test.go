// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxcache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_PathExpressionParser(t *testing.T) {
	cache := pcommon.NewMap()
	cache.PutStr("key1", "value1")
	cache.PutInt("key2", 42)

	ctx := newTestContext(cache)

	parser := PathExpressionParser(func(tCtx testContext) pcommon.Map {
		return tCtx.getCache()
	})

	t.Run("access entire cache", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "cache",
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(context.Background(), ctx)
		require.NoError(t, err)
		require.Equal(t, cache, val)

		result, ok := val.(pcommon.Map)
		require.True(t, ok)

		v1, ok := result.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, "value1", v1.Str())

		v2, ok := result.Get("key2")
		assert.True(t, ok)
		assert.Equal(t, int64(42), v2.Int())
	})

	t.Run("access specific cache key", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "cache",
			KeySlice: []ottl.Key[testContext]{
				&pathtest.Key[testContext]{
					S: ottltest.Strp("key1"),
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(context.Background(), ctx)
		require.NoError(t, err)
		assert.Equal(t, "value1", val)
	})

	t.Run("modify entire cache", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "cache",
		}

		getter, err := parser(path)
		require.NoError(t, err)

		newCache := pcommon.NewMap()
		newCache.PutStr("new_key", "new_value")

		err = getter.Set(context.Background(), ctx, newCache)
		require.NoError(t, err)

		val, ok := ctx.cache.Get("new_key")
		assert.True(t, ok)
		assert.Equal(t, "new_value", val.Str())
		require.NotEqual(t, cache, val)
	})

	t.Run("modify entire cache raw", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "cache",
		}

		getter, err := parser(path)
		require.NoError(t, err)

		newCache := pcommon.NewMap()
		newCache.PutStr("new_key", "new_value")

		err = getter.Set(context.Background(), ctx, newCache.AsRaw())
		require.NoError(t, err)

		val, ok := ctx.cache.Get("new_key")
		assert.True(t, ok)
		assert.Equal(t, "new_value", val.Str())
		require.NotEqual(t, cache, val)
	})

	t.Run("modify specific cache key", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "cache",
			KeySlice: []ottl.Key[testContext]{
				&pathtest.Key[testContext]{
					S: ottltest.Strp("key1"),
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		err = getter.Set(context.Background(), ctx, "updated_value")
		require.NoError(t, err)

		v, ok := ctx.cache.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, "updated_value", v.Str())
	})

	t.Run("add new cache key", func(t *testing.T) {
		path := &pathtest.Path[testContext]{
			N: "cache",
			KeySlice: []ottl.Key[testContext]{
				&pathtest.Key[testContext]{
					S: ottltest.Strp("key3"),
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		err = getter.Set(context.Background(), ctx, "value3")
		require.NoError(t, err)

		v, ok := ctx.cache.Get("key3")
		assert.True(t, ok)
		assert.Equal(t, "value3", v.Str())
	})

	t.Run("access nested key", func(t *testing.T) {
		nestedMap := pcommon.NewMap()
		nestedMap.PutStr("nested_key", "nested_value")
		parentMap := ctx.cache.PutEmptyMap("parent")
		nestedMap.CopyTo(parentMap)

		path := &pathtest.Path[testContext]{
			N: "cache",
			KeySlice: []ottl.Key[testContext]{
				&pathtest.Key[testContext]{
					S: ottltest.Strp("parent"),
				},
				&pathtest.Key[testContext]{
					S: ottltest.Strp("nested_key"),
				},
			},
		}

		getter, err := parser(path)
		require.NoError(t, err)

		val, err := getter.Get(context.Background(), ctx)
		require.NoError(t, err)
		assert.Equal(t, "nested_value", val)

		err = getter.Set(context.Background(), ctx, "updated_nested_value")
		require.NoError(t, err)

		parentValue, ok1 := ctx.cache.Get("parent")
		require.True(t, ok1)
		parentMap = parentValue.Map()
		nestedValue, ok2 := parentMap.Get("nested_key")
		require.True(t, ok2)
		assert.Equal(t, "updated_nested_value", nestedValue.Str())
	})
}

type testContext struct {
	cache pcommon.Map
}

func (tCtx testContext) getCache() pcommon.Map {
	return tCtx.cache
}

func newTestContext(cache pcommon.Map) testContext {
	return testContext{
		cache: cache,
	}
}
