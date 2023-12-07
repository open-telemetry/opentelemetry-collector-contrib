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

type testKey struct {
	s       *string
	i       *int64
	nextKey *testKey
}

func (k *testKey) String() *string {
	return k.s
}

func (k *testKey) Int() *int64 {
	return k.i
}

func (k *testKey) Next() ottl.Key {
	if k.nextKey == nil {
		return nil
	}
	return k.nextKey
}

func Test_GetSliceValue_Invalid(t *testing.T) {
	tests := []struct {
		name string
		key  ottl.Key
		err  error
	}{
		{
			name: "no key",
			key:  nil,
			err:  fmt.Errorf("cannot get slice value without key"),
		},
		{
			name: "first key not an integer",
			key: &testKey{
				s: ottltest.Strp("key"),
			},
			err: fmt.Errorf("non-integer indexing is not supported"),
		},
		{
			name: "index too large",
			key: &testKey{
				i: ottltest.Intp(1),
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			key: &testKey{
				i: ottltest.Intp(-1),
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			key: &testKey{
				i: ottltest.Intp(0),
				nextKey: &testKey{
					s: ottltest.Strp("string"),
				},
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewSlice()
			s.AppendEmpty().SetStr("val")

			_, err := GetSliceValue(s, tt.key)
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_SetSliceValue_Invalid(t *testing.T) {
	tests := []struct {
		name string
		keys ottl.Key
		err  error
	}{
		{
			name: "no key",
			keys: nil,
			err:  fmt.Errorf("cannot set slice value without key"),
		},
		{
			name: "first key not an integer",
			keys: &testKey{
				s: ottltest.Strp("key"),
			},
			err: fmt.Errorf("non-integer indexing is not supported"),
		},
		{
			name: "index too large",
			keys: &testKey{
				i: ottltest.Intp(1),
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: &testKey{
				i: ottltest.Intp(-1),
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: &testKey{
				i: ottltest.Intp(0),
				nextKey: &testKey{
					s: ottltest.Strp("string"),
				},
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewSlice()
			s.AppendEmpty().SetStr("val")

			err := SetSliceValue(s, tt.keys, "value")
			assert.Equal(t, tt.err, err)
		})
	}
}
