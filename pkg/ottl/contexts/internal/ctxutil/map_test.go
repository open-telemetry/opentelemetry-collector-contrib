// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxutil_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
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
					I: ottltest.Intp(0),
					G: getSetter,
				},
			},
			err: "cannot get map value: unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'",
		},
		{
			name: "index map with int",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("map"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: ottltest.Intp(0),
					G: getSetter,
				},
			},
			err: "unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'",
		},
		{
			name: "index slice with string",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					S: ottltest.Strp("invalid"),
					G: getSetter,
				},
			},
			err: "unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'",
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: ottltest.Intp(1),
					G: getSetter,
				},
			},
			err: "index 1 out of bounds",
		},
		{
			name: "index too small",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: ottltest.Intp(-1),
					G: getSetter,
				},
			},
			err: "index -1 out of bounds",
		},
		{
			name: "invalid type",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("string"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					S: ottltest.Strp("string"),
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
			S: ottltest.Strp("map1"),
		},
		&pathtest.Key[any]{
			S: ottltest.Strp("unknown key"),
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
					I: ottltest.Intp(0),
					G: getSetter,
				},
			},
			err: "cannot set map value: unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'",
		},
		{
			name: "index map with int",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("map"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: ottltest.Intp(0),
					G: getSetter,
				},
			},
			err: "unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'",
		},
		{
			name: "index slice with string",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					S: ottltest.Strp("map"),
					G: getSetter,
				},
			},
			err: "unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'",
		},
		{
			name: "slice index too large",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: ottltest.Intp(1),
					G: getSetter,
				},
			},
			err: "index 1 out of bounds",
		},
		{
			name: "slice index too small",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					I: ottltest.Intp(-1),
					G: getSetter,
				},
			},
			err: "index -1 out of bounds",
		},
		{
			name: "slice index too small",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("string"),
					G: getSetter,
				},
				&pathtest.Key[any]{
					S: ottltest.Strp("string"),
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
			S: ottltest.Strp("map1"),
		},
		&pathtest.Key[any]{
			S: ottltest.Strp("map2"),
		},
		&pathtest.Key[any]{
			S: ottltest.Strp("foo"),
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
			S: ottltest.Strp("map1"),
		},
		&pathtest.Key[any]{
			S: ottltest.Strp("map2"),
		},
		&pathtest.Key[any]{
			S: ottltest.Strp("foo"),
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
					I: ottltest.Intp(0),
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
					S: ottltest.Strp("string"),
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
