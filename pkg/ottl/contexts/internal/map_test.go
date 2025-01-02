// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_GetMapValue_Invalid(t *testing.T) {
	getSetter := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return nil, nil
		},
		Setter: func(_ context.Context, tCtx any, val any) error {
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
				&TestKey[any]{
					I: ottltest.Intp(0),
					G: getSetter,
				},
			},
			err: fmt.Errorf("unable to resolve a string index: could not resolve key for map/slice, expecting 'string' but got '<nil>'"),
		},
		{
			name: "index map with int",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("map"),
					G: getSetter,
				},
				&TestKey[any]{
					I: ottltest.Intp(0),
					G: getSetter,
				},
			},
			err: fmt.Errorf("unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'"),
		},
		{
			name: "index slice with string",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&TestKey[any]{
					S: ottltest.Strp("invalid"),
					G: getSetter,
				},
			},
			err: fmt.Errorf("unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'"),
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&TestKey[any]{
					I: ottltest.Intp(1),
					G: getSetter,
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&TestKey[any]{
					I: ottltest.Intp(-1),
					G: getSetter,
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("string"),
					G: getSetter,
				},
				&TestKey[any]{
					S: ottltest.Strp("string"),
					G: getSetter,
				},
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			m.PutStr("string", "invalid")
			m.PutEmptyMap("map").PutStr("foo", "bar")

			s := m.PutEmptySlice("slice")
			s.AppendEmpty()

			_, err := GetMapValue[any](context.Background(), nil, m, tt.keys)
			assert.Equal(t, tt.err.Error(), err.Error())
		})
	}
}

func Test_GetMapValue_MissingKey(t *testing.T) {
	m := pcommon.NewMap()
	m.PutEmptyMap("map1").PutEmptyMap("map2")
	keys := []ottl.Key[any]{
		&TestKey[any]{
			S: ottltest.Strp("map1"),
		},
		&TestKey[any]{
			S: ottltest.Strp("unknown key"),
		},
	}
	result, err := GetMapValue[any](context.Background(), nil, m, keys)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func Test_GetMapValue_NilKey(t *testing.T) {
	_, err := GetMapValue[any](context.Background(), nil, pcommon.NewMap(), nil)
	assert.Error(t, err)
}

func Test_SetMapValue_Invalid(t *testing.T) {
	getSetter := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return nil, nil
		},
		Setter: func(_ context.Context, tCtx any, val any) error {
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
				&TestKey[any]{
					I: ottltest.Intp(0),
					G: getSetter,
				},
			},
			err: fmt.Errorf("unable to resolve a string index: could not resolve key for map/slice, expecting 'string' but got '<nil>'"),
		},
		{
			name: "index map with int",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("map"),
					G: getSetter,
				},
				&TestKey[any]{
					I: ottltest.Intp(0),
					G: getSetter,
				},
			},
			err: fmt.Errorf("unable to resolve a string index in map: could not resolve key for map/slice, expecting 'string' but got '<nil>'"),
		},
		{
			name: "index slice with string",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&TestKey[any]{
					S: ottltest.Strp("map"),
					G: getSetter,
				},
			},
			err: fmt.Errorf("unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'"),
		},
		{
			name: "slice index too large",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&TestKey[any]{
					I: ottltest.Intp(1),
					G: getSetter,
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "slice index too small",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("slice"),
					G: getSetter,
				},
				&TestKey[any]{
					I: ottltest.Intp(-1),
					G: getSetter,
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "slice index too small",
			keys: []ottl.Key[any]{
				&TestKey[any]{
					S: ottltest.Strp("string"),
					G: getSetter,
				},
				&TestKey[any]{
					S: ottltest.Strp("string"),
					G: getSetter,
				},
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			m.PutStr("string", "invalid")
			m.PutEmptyMap("map")

			s := m.PutEmptySlice("slice")
			s.AppendEmpty()

			err := SetMapValue[any](context.Background(), nil, m, tt.keys, "value")
			assert.Equal(t, tt.err.Error(), err.Error())
		})
	}
}

func Test_SetMapValue_AddingNewSubMap(t *testing.T) {
	m := pcommon.NewMap()
	m.PutEmptyMap("map1").PutStr("test", "test")
	keys := []ottl.Key[any]{
		&TestKey[any]{
			S: ottltest.Strp("map1"),
		},
		&TestKey[any]{
			S: ottltest.Strp("map2"),
		},
		&TestKey[any]{
			S: ottltest.Strp("foo"),
		},
	}
	err := SetMapValue[any](context.Background(), nil, m, keys, "bar")
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
		&TestKey[any]{
			S: ottltest.Strp("map1"),
		},
		&TestKey[any]{
			S: ottltest.Strp("map2"),
		},
		&TestKey[any]{
			S: ottltest.Strp("foo"),
		},
	}
	err := SetMapValue[any](context.Background(), nil, m, keys, "bar")
	assert.NoError(t, err)

	expected := pcommon.NewMap()
	expected.PutEmptyMap("map1").PutEmptyMap("map2").PutStr("foo", "bar")

	assert.Equal(t, expected, m)
}

func Test_SetMapValue_NilKey(t *testing.T) {
	err := SetMapValue[any](context.Background(), nil, pcommon.NewMap(), nil, "bar")
	assert.Error(t, err)
}
