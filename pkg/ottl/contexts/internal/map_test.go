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
		key  ottl.Key
		err  error
	}{
		{
			name: "no key",
			key:  nil,
			err:  fmt.Errorf("cannot get map value without key"),
		},
		{
			name: "first key not a string",
			key: &testKey{
				i: ottltest.Intp(0),
			},
			err: fmt.Errorf("non-string indexing is not supported"),
		},
		{
			name: "index map with int",
			key: &testKey{
				s: ottltest.Strp("map"),
				nextKey: &testKey{
					i: ottltest.Intp(0),
				},
			},
			err: fmt.Errorf("map must be indexed by a string"),
		},
		{
			name: "index slice with string",
			key: &testKey{
				s: ottltest.Strp("slice"),
				nextKey: &testKey{
					s: ottltest.Strp("invalid"),
				},
			},
			err: fmt.Errorf("slice must be indexed by an int"),
		},
		{
			name: "index too large",
			key: &testKey{
				s: ottltest.Strp("slice"),
				nextKey: &testKey{
					i: ottltest.Intp(1),
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			key: &testKey{
				s: ottltest.Strp("slice"),
				nextKey: &testKey{
					i: ottltest.Intp(-1),
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			key: &testKey{
				s: ottltest.Strp("string"),
				nextKey: &testKey{
					s: ottltest.Strp("string"),
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

			_, err := GetMapValue(m, tt.key)
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_GetMapValue_MissingKey(t *testing.T) {
	m := pcommon.NewMap()
	m.PutEmptyMap("map1").PutEmptyMap("map2")

	k := &testKey{
		s: ottltest.Strp("map1"),
		nextKey: &testKey{
			s: ottltest.Strp("unknown key"),
		},
	}

	result, err := GetMapValue(m, k)
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func Test_SetMapValue_Invalid(t *testing.T) {
	tests := []struct {
		name string
		key  ottl.Key
		err  error
	}{
		{
			name: "no key",
			key:  nil,
			err:  fmt.Errorf("cannot set map value without key"),
		},
		{
			name: "first key not a string",
			key: &testKey{
				i: ottltest.Intp(0),
			},
			err: fmt.Errorf("non-string indexing is not supported"),
		},
		{
			name: "index map with int",
			key: &testKey{
				s: ottltest.Strp("map"),
				nextKey: &testKey{
					i: ottltest.Intp(0),
				},
			},
			err: fmt.Errorf("map must be indexed by a string"),
		},
		{
			name: "index slice with string",
			key: &testKey{
				s: ottltest.Strp("slice"),
				nextKey: &testKey{
					s: ottltest.Strp("map"),
				},
			},
			err: fmt.Errorf("slice must be indexed by an int"),
		},
		{
			name: "slice index too large",
			key: &testKey{
				s: ottltest.Strp("slice"),
				nextKey: &testKey{
					i: ottltest.Intp(1),
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "slice index too small",
			key: &testKey{
				s: ottltest.Strp("slice"),
				nextKey: &testKey{
					i: ottltest.Intp(-1),
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			key: &testKey{
				s: ottltest.Strp("string"),
				nextKey: &testKey{
					s: ottltest.Strp("string"),
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

			err := SetMapValue(m, tt.key, "value")
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_SetMapValue_AddingNewSubMap(t *testing.T) {
	m := pcommon.NewMap()
	m.PutEmptyMap("map1").PutStr("test", "test")

	k := &testKey{
		s: ottltest.Strp("map1"),
		nextKey: &testKey{
			s: ottltest.Strp("map2"),
			nextKey: &testKey{
				s: ottltest.Strp("foo"),
			},
		},
	}

	err := SetMapValue(m, k, "bar")
	assert.Nil(t, err)

	expected := pcommon.NewMap()
	sub := expected.PutEmptyMap("map1")
	sub.PutStr("test", "test")
	sub.PutEmptyMap("map2").PutStr("foo", "bar")

	assert.Equal(t, expected, m)
}

func Test_SetMapValue_EmptyMap(t *testing.T) {
	m := pcommon.NewMap()

	k := &testKey{
		s: ottltest.Strp("map1"),
		nextKey: &testKey{
			s: ottltest.Strp("map2"),
			nextKey: &testKey{
				s: ottltest.Strp("foo"),
			},
		},
	}

	err := SetMapValue(m, k, "bar")
	assert.Nil(t, err)

	expected := pcommon.NewMap()
	expected.PutEmptyMap("map1").PutEmptyMap("map2").PutStr("foo", "bar")

	assert.Equal(t, expected, m)
}
