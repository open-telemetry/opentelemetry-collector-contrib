// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxutil_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
)

func Test_GetMapValue_Invalid(t *testing.T) {
	getSetter := &ottl.StandardGetSetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, nil
		},
		Setter: func(context.Context, any, any) error {
			return nil
		},
	}
	tests := []struct {
		name string
		keys []ottl.Key[any]
		err  string
	}{
		{
			name: "first key not a string",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: new(int64(0)),
					G: getSetter,
				},
			},
			err: "cannot get map value: unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'",
		},
		{
			name: "index map with int",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: new("map"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: new(int64(0)),
					G: getSetter,
				},
			},
			err: "unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'",
		},
		{
			name: "index slice with string",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: new("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					S: new("invalid"),
					G: getSetter,
				},
			},
			err: "unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'",
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: new("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: new(int64(1)),
					G: getSetter,
				},
			},
			err: "index 1 out of bounds",
		},
		{
			name: "index too small",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: new("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: new(int64(-1)),
					G: getSetter,
				},
			},
			err: "index -1 out of bounds",
		},
		{
			name: "invalid type",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: new("string"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					S: new("string"),
					G: getSetter,
				},
			},
			err: "type Str does not support string indexing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			m.PutStr("string", "invalid")
			m.PutEmptyMap("map").PutStr("foo", "bar")

			s := m.PutEmptySlice("slice")
			s.AppendEmpty()

			_, err := ctxutil.GetMapValue[any](t.Context(), nil, m, tt.keys)
			assert.EqualError(t, err, tt.err)
		})
	}
}

func Test_GetMapValue_MissingKey(t *testing.T) {
	m := pcommon.NewMap()
	m.PutEmptyMap("map1").PutEmptyMap("map2")
	keys := []ottl.Key[any]{
		&pathtest.Key[any]{
			S: new("map1"),
		},
		&pathtest.Key[any]{
			S: new("unknown key"),
		},
	}
	result, err := ctxutil.GetMapValue[any](t.Context(), nil, m, keys)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func Test_GetMapValue_NilKey(t *testing.T) {
	_, err := ctxutil.GetMapValue[any](t.Context(), nil, pcommon.NewMap(), nil)
	assert.Error(t, err)
}

func Test_SetMapValue_Invalid(t *testing.T) {
	getSetter := &ottl.StandardGetSetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, nil
		},
		Setter: func(context.Context, any, any) error {
			return nil
		},
	}
	tests := []struct {
		name string
		keys []ottl.Key[any]
		err  string
	}{
		{
			name: "first key not a string",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: new(int64(0)),
					G: getSetter,
				},
			},
			err: "cannot set map value: unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'",
		},
		{
			name: "index map with int",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: new("map"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: new(int64(0)),
					G: getSetter,
				},
			},
			err: "unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'",
		},
		{
			name: "index slice with string",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: new("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					S: new("map"),
					G: getSetter,
				},
			},
			err: "unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'",
		},
		{
			name: "slice index too large",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: new("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: new(int64(1)),
					G: getSetter,
				},
			},
			err: "index 1 out of bounds",
		},
		{
			name: "slice index too small",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: new("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: new(int64(-1)),
					G: getSetter,
				},
			},
			err: "index -1 out of bounds",
		},
		{
			name: "slice index too small",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: new("string"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					S: new("string"),
					G: getSetter,
				},
			},
			err: "type Str does not support string indexing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			m.PutStr("string", "invalid")
			m.PutEmptyMap("map")

			s := m.PutEmptySlice("slice")
			s.AppendEmpty()

			err := ctxutil.SetMapValue[any](t.Context(), nil, m, tt.keys, "value")
			assert.EqualError(t, err, tt.err)
		})
	}
}

func Test_SetMapValue_AddingNewSubMap(t *testing.T) {
	m := pcommon.NewMap()
	m.PutEmptyMap("map1").PutStr("test", "test")
	keys := []ottl.Key[any]{
		&pathtest.Key[any]{
			S: new("map1"),
		},
		&pathtest.Key[any]{
			S: new("map2"),
		},
		&pathtest.Key[any]{
			S: new("foo"),
		},
	}
	err := ctxutil.SetMapValue[any](t.Context(), nil, m, keys, "bar")
	require.NoError(t, err)

	expected := pcommon.NewMap()
	sub := expected.PutEmptyMap("map1")
	sub.PutStr("test", "test")
	sub.PutEmptyMap("map2").PutStr("foo", "bar")

	assert.Equal(t, expected, m)
}

func Test_SetMapValue_EmptyMap(t *testing.T) {
	m := pcommon.NewMap()
	keys := []ottl.Key[any]{
		&pathtest.Key[any]{
			S: new("map1"),
		},
		&pathtest.Key[any]{
			S: new("map2"),
		},
		&pathtest.Key[any]{
			S: new("foo"),
		},
	}
	err := ctxutil.SetMapValue[any](t.Context(), nil, m, keys, "bar")
	require.NoError(t, err)

	expected := pcommon.NewMap()
	expected.PutEmptyMap("map1").PutEmptyMap("map2").PutStr("foo", "bar")

	assert.Equal(t, expected, m)
}

func Test_SetMapValue_NilKey(t *testing.T) {
	err := ctxutil.SetMapValue[any](t.Context(), nil, pcommon.NewMap(), nil, "bar")
	assert.Error(t, err)
}

func Test_SetMapValue_NilClearsValue(t *testing.T) {
	m := pcommon.NewMap()
	keys := []ottl.Key[any]{
		&pathtest.Key[any]{
			S: new("test"),
		},
	}
	err := ctxutil.SetMapValue[any](t.Context(), nil, m, keys, nil)
	require.NoError(t, err)

	val, ok := m.Get("test")
	require.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeEmpty, val.Type())
}

func Test_SetMap_NilClearsMap(t *testing.T) {
	m := pcommon.NewMap()
	m.PutStr("foo", "bar")
	err := ctxutil.SetMap(m, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, m.Len())
}

func Test_GetMap_NilReturnsEmptyMap(t *testing.T) {
	m, err := ctxutil.GetMap(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, m.Len())
}

func Test_GetMap_FromRawError(t *testing.T) {
	// A map[string]any containing an unsupported value type causes FromRaw to error.
	_, err := ctxutil.GetMap(map[string]any{"key": struct{}{}})
	require.Error(t, err)
}

func Test_SetMap(t *testing.T) {
	createMap := func() pcommon.Map {
		m := pcommon.NewMap()
		require.NoError(t, m.FromRaw(map[string]any{"foo": "bar"}))
		return m
	}
	tests := []struct {
		name     string
		val      any
		err      string
		expected any
	}{
		{
			name:     "invalid type",
			val:      "invalid",
			err:      "unsupported type provided for setting a pcommon.Map: string",
			expected: pcommon.NewMap(),
		},
		{
			name:     "raw map",
			val:      map[string]any{"foo": "bar"},
			expected: createMap(),
		},
		{
			name:     "pcommon.Map",
			val:      createMap(),
			expected: createMap(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			err := ctxutil.SetMap(m, tt.val)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, m)
		})
	}
}

func Test_GetMap(t *testing.T) {
	createMap := func() pcommon.Map {
		m := pcommon.NewMap()
		require.NoError(t, m.FromRaw(map[string]any{"foo": "bar"}))
		return m
	}
	tests := []struct {
		name string
		val  any
		err  string
	}{
		{
			name: "invalid type",
			val:  "invalid",
			err:  "failed to convert type string into pcommon.Map",
		},
		{
			name: "raw map",
			val:  map[string]any{"foo": "bar"},
		},
		{
			name: "pcommon.Map",
			val:  createMap(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := ctxutil.GetMap(tt.val)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			assert.Equal(t, m, createMap())
		})
	}
}

func Test_GetMapKeyName(t *testing.T) {
	getSetter := &ottl.StandardGetSetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, nil
		},
	}
	tests := []struct {
		name string
		keys []ottl.Key[any]
		err  string
		key  string
	}{
		{
			name: "first key not a string",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: new(int64(0)),
					G: getSetter,
				},
			},
			err: "unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'",
		},
		{
			name: "first key not initialized",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{},
			},
			err: "unable to resolve a string index in map: invalid key type",
		},
		{
			name: "valid",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: new("string"),
				},
			},
			key: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolvedKey, err := ctxutil.GetMapKeyName[any](t.Context(), nil, tt.keys[0])
			if tt.err != "" {
				assert.EqualError(t, err, tt.err)
				return
			}
			assert.Equal(t, tt.key, *resolvedKey)
		})
	}
}

func Test_MapGetSetter(t *testing.T) {
	t.Run("Get and Set", func(t *testing.T) {
		target := pcommon.NewMap()
		target.PutStr("k1", "v1")

		gs := ctxutil.MapGetSetter[any]{
			Getter: func(context.Context, any) (pcommon.Map, error) {
				return target, nil
			},
		}

		got, err := gs.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, target, got)

		require.NoError(t, gs.Set(t.Context(), nil, map[string]any{"k2": "v2"}))
		expected := pcommon.NewMap()
		expected.PutStr("k2", "v2")
		assert.Equal(t, expected, target)

		newMap := pcommon.NewMap()
		newMap.PutStr("k3", "v3")
		require.NoError(t, gs.Set(t.Context(), nil, newMap))
		assert.Equal(t, newMap, target)

		require.NoError(t, gs.Set(t.Context(), nil, nil))
		assert.Equal(t, 0, target.Len())

		err = gs.Set(t.Context(), nil, "invalid")
		assert.EqualError(t, err, "unsupported type provided for setting a pcommon.Map: string")
	})

	t.Run("PMapGetter and PMapGetSetter", func(t *testing.T) {
		target := pcommon.NewMap()
		target.PutStr("k1", "v1")

		gs := ctxutil.MapGetSetter[any]{
			Getter: func(context.Context, any) (pcommon.Map, error) {
				return target, nil
			},
		}

		pmapGetter := gs.PMapGetter()
		got, err := pmapGetter.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, target, got)

		pmapGetSetter := gs.PMapGetSetter()
		got, err = pmapGetSetter.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, target, got)

		replacement := pcommon.NewMap()
		replacement.PutStr("k2", "v2")
		require.NoError(t, pmapGetSetter.Set(t.Context(), nil, replacement))
		assert.Equal(t, replacement, target)
	})

	t.Run("Getter error", func(t *testing.T) {
		getterErr := errors.New("getter error")
		gs := ctxutil.MapGetSetter[any]{
			Getter: func(context.Context, any) (pcommon.Map, error) {
				return pcommon.Map{}, getterErr
			},
		}

		got, err := gs.Get(t.Context(), nil)
		assert.ErrorIs(t, err, getterErr)
		assert.Nil(t, got)

		err = gs.Set(t.Context(), nil, pcommon.NewMap())
		assert.ErrorIs(t, err, getterErr)

		_, err = gs.PMapGetter().Get(t.Context(), nil)
		assert.ErrorIs(t, err, getterErr)

		_, err = gs.PMapGetSetter().Get(t.Context(), nil)
		assert.ErrorIs(t, err, getterErr)

		err = gs.PMapGetSetter().Set(t.Context(), nil, pcommon.NewMap())
		assert.ErrorIs(t, err, getterErr)
	})
}
