// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_GetMapValue_Invalid(t *testing.T) {
	tests := []struct {
		name string
		keys []ottl.Key
		err  error
	}{
		{
			name: "no keys",
			keys: []ottl.Key{},
			err:  fmt.Errorf("cannot get map value without key"),
		},
		{
			name: "first key not a string",
			keys: []ottl.Key{
				{
					Int: ottltest.Intp(0),
				},
			},
			err: fmt.Errorf("non-string indexing is not supported"),
		},
		{
			name: "index map with int",
			keys: []ottl.Key{
				{
					String: ottltest.Strp("map"),
				},
				{
					Int: ottltest.Intp(0),
				},
			},
			err: fmt.Errorf("map must be indexed by a string"),
		},
		{
			name: "index slice with string",
			keys: []ottl.Key{
				{
					String: ottltest.Strp("slice"),
				},
				{
					String: ottltest.Strp("invalid"),
				},
			},
			err: fmt.Errorf("slice must be indexed by an int"),
		},
		{
			name: "index too large",
			keys: []ottl.Key{
				{
					String: ottltest.Strp("slice"),
				},
				{
					Int: ottltest.Intp(1),
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: []ottl.Key{
				{
					String: ottltest.Strp("slice"),
				},
				{
					Int: ottltest.Intp(-1),
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: []ottl.Key{
				{
					String: ottltest.Strp("string"),
				},
				{
					String: ottltest.Strp("string"),
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

			_, err := GetMapValue(m, tt.keys)
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_GetMapValue_MissingKey(t *testing.T) {
	m := pcommon.NewMap()
	m.PutEmptyMap("map1").PutEmptyMap("map2")
	keys := []ottl.Key{
		{
			String: ottltest.Strp("map1"),
		},
		{
			String: ottltest.Strp("unknown key"),
		},
	}
	result, err := GetMapValue(m, keys)
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func Test_SetMapValue_Invalid(t *testing.T) {
	tests := []struct {
		name string
		keys []ottl.Key
		err  error
	}{
		{
			name: "no keys",
			keys: []ottl.Key{},
			err:  fmt.Errorf("cannot set map value without key"),
		},
		{
			name: "first key not a string",
			keys: []ottl.Key{
				{
					Int: ottltest.Intp(0),
				},
			},
			err: fmt.Errorf("non-string indexing is not supported"),
		},
		{
			name: "index map with int",
			keys: []ottl.Key{
				{
					String: ottltest.Strp("map"),
				},
				{
					Int: ottltest.Intp(0),
				},
			},
			err: fmt.Errorf("map must be indexed by a string"),
		},
		{
			name: "index slice with string",
			keys: []ottl.Key{
				{
					String: ottltest.Strp("slice"),
				},
				{
					String: ottltest.Strp("map"),
				},
			},
			err: fmt.Errorf("slice must be indexed by an int"),
		},
		{
			name: "slice index too large",
			keys: []ottl.Key{
				{
					String: ottltest.Strp("slice"),
				},
				{
					Int: ottltest.Intp(1),
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "slice index too small",
			keys: []ottl.Key{
				{
					String: ottltest.Strp("slice"),
				},
				{
					Int: ottltest.Intp(-1),
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "slice index too small",
			keys: []ottl.Key{
				{
					String: ottltest.Strp("string"),
				},
				{
					String: ottltest.Strp("string"),
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

			err := SetMapValue(m, tt.keys, "value")
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_SetMapValue_AddingNewSubMap(t *testing.T) {
	m := pcommon.NewMap()
	m.PutEmptyMap("map1").PutStr("test", "test")
	keys := []ottl.Key{
		{
			String: ottltest.Strp("map1"),
		},
		{
			String: ottltest.Strp("map2"),
		},
		{
			String: ottltest.Strp("foo"),
		},
	}
	err := SetMapValue(m, keys, "bar")
	assert.Nil(t, err)

	expected := pcommon.NewMap()
	sub := expected.PutEmptyMap("map1")
	sub.PutStr("test", "test")
	sub.PutEmptyMap("map2").PutStr("foo", "bar")

	assert.Equal(t, expected, m)
}

func Test_SetMapValue_EmptyMap(t *testing.T) {
	m := pcommon.NewMap()
	keys := []ottl.Key{
		{
			String: ottltest.Strp("map1"),
		},
		{
			String: ottltest.Strp("map2"),
		},
		{
			String: ottltest.Strp("foo"),
		},
	}
	err := SetMapValue(m, keys, "bar")
	assert.Nil(t, err)

	expected := pcommon.NewMap()
	expected.PutEmptyMap("map1").PutEmptyMap("map2").PutStr("foo", "bar")

	assert.Equal(t, expected, m)
}
