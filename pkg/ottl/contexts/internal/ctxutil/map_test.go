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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_GetMapValue_Invalid(t *testing.T) {
	getSetter := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, nil
		},
		Setter: func(_ context.Context, _ any, _ any) error {
			return nil
		},
	}
	tests := []struct {
		name string
		keys []ottl.Key[any]
		err  error
	}{
		{
			name: "first key not a string",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: ottltest.Intp(0),
					G: getSetter,
				},
			},
			err: errors.New("cannot get map value: unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'"),
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
			err: errors.New("unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'"),
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
			err: errors.New("unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'"),
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
			err: errors.New("index 1 out of bounds"),
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
			err: errors.New("index -1 out of bounds"),
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
			err: errors.New("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			m.PutStr("string", "invalid")
			m.PutEmptyMap("map").PutStr("foo", "bar")

			s := m.PutEmptySlice("slice")
			s.AppendEmpty()

			_, err := ctxutil.GetMapValue[any](context.Background(), nil, m, tt.keys)
			assert.Equal(t, tt.err.Error(), err.Error())
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
	result, err := ctxutil.GetMapValue[any](context.Background(), nil, m, keys)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func Test_GetMapValue_NilKey(t *testing.T) {
	_, err := ctxutil.GetMapValue[any](context.Background(), nil, pcommon.NewMap(), nil)
	assert.Error(t, err)
}

func Test_SetMapValue_Invalid(t *testing.T) {
	getSetter := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, nil
		},
		Setter: func(_ context.Context, _ any, _ any) error {
			return nil
		},
	}
	tests := []struct {
		name string
		keys []ottl.Key[any]
		err  error
	}{
		{
			name: "first key not a string",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: ottltest.Intp(0),
					G: getSetter,
				},
			},
			err: errors.New("cannot set map value: unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'"),
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
			err: errors.New("unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'"),
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
			err: errors.New("unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'"),
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
			err: errors.New("index 1 out of bounds"),
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
			err: errors.New("index -1 out of bounds"),
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
			err: errors.New("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			m.PutStr("string", "invalid")
			m.PutEmptyMap("map")

			s := m.PutEmptySlice("slice")
			s.AppendEmpty()

			err := ctxutil.SetMapValue[any](context.Background(), nil, m, tt.keys, "value")
			assert.Equal(t, tt.err.Error(), err.Error())
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
	err := ctxutil.SetMapValue[any](context.Background(), nil, m, keys, "bar")
	assert.NoError(t, err)

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
	err := ctxutil.SetMapValue[any](context.Background(), nil, m, keys, "bar")
	assert.NoError(t, err)

	expected := pcommon.NewMap()
	expected.PutEmptyMap("map1").PutEmptyMap("map2").PutStr("foo", "bar")

	assert.Equal(t, expected, m)
}

func Test_SetMapValue_NilKey(t *testing.T) {
	err := ctxutil.SetMapValue[any](context.Background(), nil, pcommon.NewMap(), nil, "bar")
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
		err      error
		expected any
	}{
		{
			name:     "invalid type",
			val:      "invalid",
			err:      nil, // This is an issue in SetMap(), not returning an error here.
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
			if tt.err != nil {
				require.Equal(t, tt.err, err)
				return
			}
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
		err  error
	}{
		{
			name: "invalid type",
			val:  "invalid",
			err:  errors.New("failed to convert type string into pcommon.Map"),
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
			if tt.err != nil {
				require.Equal(t, tt.err, err)
				return
			}
			assert.Equal(t, m, createMap())
		})
	}
}

func Test_GetMapKeyName(t *testing.T) {
	getSetter := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, nil
		},
	}
	tests := []struct {
		name string
		keys []ottl.Key[any]
		err  error
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
			err: errors.New("unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'"),
		},
		{
			name: "first key not initialized",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{},
			},
			err: errors.New("unable to resolve a string index in map: invalid key type"),
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
			resolvedKey, err := ctxutil.GetMapKeyName[any](context.Background(), nil, tt.keys[0])
			if tt.err != nil {
				assert.Equal(t, tt.err.Error(), err.Error())
				return
			}
			assert.Equal(t, tt.key, *resolvedKey)
		})
	}
}
