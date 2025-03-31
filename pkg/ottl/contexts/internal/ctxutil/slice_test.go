// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxutil_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_GetSliceValue_Invalid(t *testing.T) {
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
			name: "first key not an integer",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("key"),
					G: getSetter,
				},
			},
			err: fmt.Errorf(`unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'`),
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: ottltest.Intp(1),
					G: getSetter,
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: ottltest.Intp(-1),
					G: getSetter,
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: ottltest.Intp(0),
					G: getSetter,
				},
				&pathtest.Key[any]{
					S: ottltest.Strp("string"),
					G: getSetter,
				},
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewSlice()
			s.AppendEmpty().SetStr("val")

			_, err := ctxutil.GetSliceValue[any](context.Background(), nil, s, tt.keys)
			assert.Equal(t, tt.err.Error(), err.Error())
		})
	}
}

func Test_GetSliceValue_NilKey(t *testing.T) {
	_, err := ctxutil.GetSliceValue[any](context.Background(), nil, pcommon.NewSlice(), nil)
	assert.Error(t, err)
}

func Test_SetSliceValue_Invalid(t *testing.T) {
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
			name: "first key not an integer",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("key"),
					G: getSetter,
				},
			},
			err: fmt.Errorf(`unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'`),
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: ottltest.Intp(1),
					G: getSetter,
				},
			},
			err: fmt.Errorf("index 1 out of bounds"),
		},
		{
			name: "index too small",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: ottltest.Intp(-1),
					G: getSetter,
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid type",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: ottltest.Intp(0),
					G: getSetter,
				},
				&pathtest.Key[any]{
					S: ottltest.Strp("string"),
					G: getSetter,
				},
			},
			err: fmt.Errorf("type Str does not support string indexing"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewSlice()
			s.AppendEmpty().SetStr("val")

			err := ctxutil.SetSliceValue[any](context.Background(), nil, s, tt.keys, "value")
			assert.Equal(t, tt.err.Error(), err.Error())
		})
	}
}

func Test_SetSliceValue_NilKey(t *testing.T) {
	err := ctxutil.SetSliceValue[any](context.Background(), nil, pcommon.NewSlice(), nil, "value")
	assert.Error(t, err)
}

func TestAsIntegerRawSlice(t *testing.T) {
	expected := []int32{1, 2, 3}

	testsInt32 := []struct {
		name string
		val  any
	}{
		{
			name: "Int32Slice",
			val: func() pcommon.Int32Slice {
				sl := pcommon.NewInt32Slice()
				sl.FromRaw([]int32{1, 2, 3})
				return sl
			}(),
		},
		{
			name: "slice of any (int)",
			val:  []any{1, 2, 3},
		},
		{
			name: "slice of int32",
			val:  []int32{1, 2, 3},
		},
		{
			name: "slice of int34",
			val:  []int64{1, 2, 3},
		},
	}
	for _, tt := range testsInt32 {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctxutil.AsIntegerRawSlice[int32](tt.val)
			assert.NoError(t, err, "AsIntegerRawSlice(%v)", tt.val)
			assert.Equal(t, expected, got, "AsIntegerRawSlice(%v)", tt.val)
		})
	}
}

func TestAsRawSlice_string(t *testing.T) {
	expected := []string{"a", "b", "c"}

	testsInt32 := []struct {
		name string
		val  any
	}{
		{
			name: "Slice",
			val: func() pcommon.Slice {
				sl := pcommon.NewSlice()
				assert.NoError(t, sl.FromRaw([]any{"a", "b", "c"}))
				return sl
			}(),
		},
		{
			name: "slice of any",
			val:  []any{"a", "b", "c"},
		},
		{
			name: "slice of string",
			val:  []string{"a", "b", "c"},
		},
	}
	for _, tt := range testsInt32 {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ctxutil.AsRawSlice[string](tt.val)
			require.NoError(t, err, "AsIntegerRawSlice(%v)", tt.val)
			assert.Equal(t, expected, got, "AsIntegerRawSlice(%v)", tt.val)
		})
	}
}
