// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var _ ottl.Key[testContext] = testKey{}

type testContext struct {
	cache pcommon.Map
}

func (t testContext) getCache() pcommon.Map {
	return t.cache
}

type testKey struct {
	s *string
}

func (k testKey) String(_ context.Context, _ testContext) (*string, error) {
	return k.s, nil
}

func (k testKey) Int(_ context.Context, _ testContext) (*int64, error) {
	return nil, nil
}

func (k testKey) ExpressionGetter(_ context.Context, _ testContext) (ottl.Getter[testContext], error) {
	return nil, nil
}

func createKey(s string) ottl.Key[testContext] {
	return testKey{s: &s}
}

func Test_GetSetter_Get_NoKeys(t *testing.T) {
	cache := pcommon.NewMap()
	cache.PutStr("test", "value")

	ctx := testContext{cache: cache}
	getter, err := GetSetter[testContext](testContext.getCache, nil)
	assert.NoError(t, err)

	val, err := getter.Get(context.Background(), ctx)
	assert.NoError(t, err)
	assert.Equal(t, cache, val)
}

func Test_GetSetter_Set_NoKeys(t *testing.T) {
	cache := pcommon.NewMap()
	newCache := pcommon.NewMap()
	newCache.PutStr("new", "cache")

	ctx := testContext{cache: cache}
	getter, err := GetSetter[testContext](testContext.getCache, nil)
	assert.NoError(t, err)

	err = getter.Set(context.Background(), ctx, newCache)
	assert.NoError(t, err)

	val, exists := cache.Get("new")
	assert.True(t, exists)
	assert.Equal(t, "cache", val.Str())
}

func Test_GetSetter_Get_WithKeys(t *testing.T) {
	cache := pcommon.NewMap()
	outer := cache.PutEmptyMap("outer")
	outer.PutStr("inner", "value")

	ctx := testContext{cache: cache}
	keys := []ottl.Key[testContext]{
		createKey("outer"),
		createKey("inner"),
	}

	getter, err := GetSetter[testContext](testContext.getCache, keys)
	assert.NoError(t, err)

	val, err := getter.Get(context.Background(), ctx)
	assert.NoError(t, err)
	assert.Equal(t, "value", val)
}

func Test_GetSetter_Set_WithKeys(t *testing.T) {
	cache := pcommon.NewMap()
	outer := cache.PutEmptyMap("outer")
	outer.PutStr("inner", "value")

	ctx := testContext{cache: cache}
	keys := []ottl.Key[testContext]{
		createKey("outer"),
		createKey("inner"),
	}

	getter, err := GetSetter[testContext](testContext.getCache, keys)
	assert.NoError(t, err)

	err = getter.Set(context.Background(), ctx, "new_value")
	assert.NoError(t, err)

	outerVal, exists := cache.Get("outer")
	assert.True(t, exists)
	innerVal, exists := outerVal.Map().Get("inner")
	assert.True(t, exists)
	assert.Equal(t, "new_value", innerVal.Str())
}

func Test_GetSetter_Set_InvalidType(t *testing.T) {
	cache := pcommon.NewMap()
	ctx := testContext{cache: cache}
	getter, err := GetSetter[testContext](testContext.getCache, nil)
	assert.NoError(t, err)

	err = getter.Set(context.Background(), ctx, "not a map")
	assert.NoError(t, err)
	assert.Equal(t, 0, cache.Len())
}

func Test_GetSetter_Get_InvalidKey(t *testing.T) {
	cache := pcommon.NewMap()
	ctx := testContext{cache: cache}
	keys := []ottl.Key[testContext]{
		createKey("nonexistent"),
	}

	getter, err := GetSetter[testContext](testContext.getCache, keys)
	assert.NoError(t, err)

	val, err := getter.Get(context.Background(), ctx)
	assert.NoError(t, err)
	assert.Nil(t, val)
}
