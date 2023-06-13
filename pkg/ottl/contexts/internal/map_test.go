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
		keys func() ottl.Key
		err  error
	}{
		{
			name: "no keys",
			keys: func() ottl.Key {
				return ottl.NewEmptyKey()
			},
			err: fmt.Errorf("cannot get map value without key"),
		},
		{
			name: "first key not a string",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetInt(ottltest.Intp(0))
				return k
			},
			err: fmt.Errorf("non-string indexing is not supported"),
		},
		{
			name: "index map with int",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("map"))
				k2 := ottl.NewEmptyKey()
				k2.SetInt(ottltest.Intp(0))
				k.SetNext(&k2)
				return k
			},
			err: fmt.Errorf("map must be indexed by a string"),
		},
		{
			name: "index slice with string",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("slice"))
				k2 := ottl.NewEmptyKey()
				k2.SetString(ottltest.Strp("invalid"))
				k.SetNext(&k2)
				return k
			},
			err: fmt.Errorf("slice must be indexed by an int"),
		},
		{
			name: "index too large",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("slice"))
				k2 := ottl.NewEmptyKey()
				k2.SetInt(ottltest.Intp(1))
				k.SetNext(&k2)
				return k
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("slice"))
				k2 := ottl.NewEmptyKey()
				k2.SetInt(ottltest.Intp(-1))
				k.SetNext(&k2)
				return k
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("string"))
				k2 := ottl.NewEmptyKey()
				k2.SetString(ottltest.Strp("string"))
				k.SetNext(&k2)
				return k
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

			_, err := GetMapValue(m, tt.keys())
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_GetMapValue_MissingKey(t *testing.T) {
	m := pcommon.NewMap()
	m.PutEmptyMap("map1").PutEmptyMap("map2")

	k := ottl.NewEmptyKey()
	k.SetString(ottltest.Strp("map1"))
	k2 := ottl.NewEmptyKey()
	k2.SetString(ottltest.Strp("unknown key"))
	k.SetNext(&k2)

	result, err := GetMapValue(m, k)
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func Test_SetMapValue_Invalid(t *testing.T) {
	tests := []struct {
		name string
		keys func() ottl.Key
		err  error
	}{
		{
			name: "no keys",
			keys: func() ottl.Key { return ottl.Key{} },
			err:  fmt.Errorf("cannot set map value without key"),
		},
		{
			name: "first key not a string",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetInt(ottltest.Intp(0))
				return k
			},
			err: fmt.Errorf("non-string indexing is not supported"),
		},
		{
			name: "index map with int",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("map"))
				k2 := ottl.NewEmptyKey()
				k2.SetInt(ottltest.Intp(0))
				k.SetNext(&k2)
				return k
			},
			err: fmt.Errorf("map must be indexed by a string"),
		},
		{
			name: "index slice with string",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("slice"))
				k2 := ottl.NewEmptyKey()
				k2.SetString(ottltest.Strp("map"))
				k.SetNext(&k2)
				return k
			},
			err: fmt.Errorf("slice must be indexed by an int"),
		},
		{
			name: "slice index too large",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("slice"))
				k2 := ottl.NewEmptyKey()
				k2.SetInt(ottltest.Intp(1))
				k.SetNext(&k2)
				return k
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "slice index too small",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("slice"))
				k2 := ottl.NewEmptyKey()
				k2.SetInt(ottltest.Intp(-1))
				k.SetNext(&k2)
				return k
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "slice index too small",
			keys: func() ottl.Key {
				k := ottl.NewEmptyKey()
				k.SetString(ottltest.Strp("string"))
				k2 := ottl.NewEmptyKey()
				k2.SetString(ottltest.Strp("string"))
				k.SetNext(&k2)
				return k
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

			err := SetMapValue(m, tt.keys(), "value")
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_SetMapValue_AddingNewSubMap(t *testing.T) {
	m := pcommon.NewMap()
	m.PutEmptyMap("map1").PutStr("test", "test")

	k := ottl.NewEmptyKey()
	k.SetString(ottltest.Strp("map1"))
	k2 := ottl.NewEmptyKey()
	k2.SetString(ottltest.Strp("map2"))
	k3 := ottl.NewEmptyKey()
	k3.SetString(ottltest.Strp("foo"))
	k2.SetNext(&k3)
	k.SetNext(&k2)

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

	k := ottl.NewEmptyKey()
	k.SetString(ottltest.Strp("map1"))
	k2 := ottl.NewEmptyKey()
	k2.SetString(ottltest.Strp("map2"))
	k3 := ottl.NewEmptyKey()
	k3.SetString(ottltest.Strp("foo"))
	k2.SetNext(&k3)
	k.SetNext(&k2)

	err := SetMapValue(m, k, "bar")
	assert.Nil(t, err)

	expected := pcommon.NewMap()
	expected.PutEmptyMap("map1").PutEmptyMap("map2").PutStr("foo", "bar")

	assert.Equal(t, expected, m)
}
