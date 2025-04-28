// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxutil_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_GetSliceValue_Valid(t *testing.T) {
	s := pcommon.NewSlice()
	s.AppendEmpty().SetStr("val")

	value, err := ctxutil.GetSliceValue[any](context.Background(), nil, s, []ottl.Key[any]{
		&pathtest.Key[any]{
			I: ottltest.Intp(0),
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, "val", value)
}

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
			err: errors.New(`unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'`),
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
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
					I: ottltest.Intp(0),
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

func Test_SetSliceValue_Valid(t *testing.T) {
	s := pcommon.NewSlice()
	s.AppendEmpty().SetStr("val")

	err := ctxutil.SetSliceValue[any](context.Background(), nil, s, []ottl.Key[any]{
		&pathtest.Key[any]{I: ottltest.Intp(0)},
	}, "value")
	assert.NoError(t, err)
	assert.Equal(t, "value", s.At(0).AsRaw())
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
			err: errors.New(`unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'`),
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
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
					I: ottltest.Intp(0),
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

func Test_GetCommonTypedSliceValue_Valid(t *testing.T) {
	s := pcommon.NewStringSlice()
	s.Append("one", "two", "three")

	value, err := ctxutil.GetCommonTypedSliceValue[any, string](context.Background(), nil, s, []ottl.Key[any]{
		&pathtest.Key[any]{
			I: ottltest.Intp(1),
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, s.At(1), value)
}

func Test_GetCommonTypedSliceValue_Invalid(t *testing.T) {
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
			err: errors.New(`unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'`),
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
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
					I: ottltest.Intp(-1),
					G: getSetter,
				},
			},
			err: errors.New("index -1 out of bounds"),
		},
		{
			name: "invalid key type",
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
			err: errors.New("type pcommon.StringSlice does not support indexing"),
		},
		{
			name: "nil key",
			keys: nil,
			err:  errors.New("cannot get slice value without key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewStringSlice()
			s.Append("val")

			_, err := ctxutil.GetCommonTypedSliceValue[any, string](context.Background(), nil, s, tt.keys)
			assert.Equal(t, tt.err.Error(), err.Error())
		})
	}
}

func Test_SetCommonTypedSliceValue_Valid(t *testing.T) {
	s := pcommon.NewStringSlice()
	s.Append("1", "2", "3")

	err := ctxutil.SetCommonTypedSliceValue[any, string](context.Background(), nil, s, []ottl.Key[any]{
		&pathtest.Key[any]{I: ottltest.Intp(1)},
	}, "two")
	assert.NoError(t, err)
	assert.Equal(t, "two", s.At(1))
}

func Test_SetCommonTypedSliceValue_Invalid(t *testing.T) {
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
		val  any
	}{
		{
			name: "first key not an integer",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("key"),
					G: getSetter,
				},
			},
			err: errors.New(`unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'`),
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
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
					I: ottltest.Intp(-1),
					G: getSetter,
				},
			},
			err: errors.New("index -1 out of bounds"),
		},
		{
			name: "invalid key type",
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
			err: errors.New("type pcommon.StringSlice does not support indexing"),
		},
		{
			name: "invalid value type",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: ottltest.Intp(0),
				},
			},
			val: 1,
			err: errors.New("invalid value type provided for a slice of string: int"),
		},
		{
			name: "nil key",
			keys: nil,
			err:  errors.New("cannot set slice value without key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewStringSlice()
			s.Append("one")

			var val any
			if tt.val != nil {
				val = tt.val
			} else {
				val = 1
			}

			err := ctxutil.SetCommonTypedSliceValue[any, string](context.Background(), nil, s, tt.keys, val)
			assert.Equal(t, tt.err.Error(), err.Error())
		})
	}
}

func Test_SetCommonTypedSliceValues(t *testing.T) {
	pss := pcommon.NewStringSlice()
	pss.FromRaw([]string{"one", "two", "three"})

	ps := pcommon.NewSlice()
	err := ps.FromRaw([]any{"one", "two", "three"})
	assert.NoError(t, err)

	invalid := pcommon.NewSlice()
	err = invalid.FromRaw([]any{"one", 1, "three"})
	assert.NoError(t, err)

	tests := []struct {
		name      string
		val       any
		want      []string
		wantError string
	}{
		{
			name: "from any slice",
			val:  []any{"one", "two", "three"},
			want: []string{"one", "two", "three"},
		},
		{
			name: "from typed value",
			val:  []string{"one", "two", "three"},
			want: []string{"one", "two", "three"},
		},
		{
			name: "from the same slice type",
			val:  pss,
			want: pss.AsRaw(),
		},
		{
			name: "from pcommon.Slice",
			val:  ps,
			want: []string{"one", "two", "three"},
		},
		{
			name:      "from invalid type",
			val:       1,
			wantError: "invalid type provided for setting a slice of int: string",
		},
		{
			name:      "from slice with invalid value",
			val:       invalid,
			wantError: "invalid value type provided for a slice of []string: int64",
		},
		{
			name:      "from any slice with invalid value",
			val:       []any{"1", int64(2), "3"},
			wantError: "invalid value type provided for a slice of string: int64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewStringSlice()
			err := ctxutil.SetCommonTypedSliceValues[string](s, tt.val)
			if tt.wantError != "" {
				assert.ErrorContains(t, err, tt.wantError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, s.AsRaw())
			}
		})
	}
}

func Test_GetCommonIntSliceValue_Valid(t *testing.T) {
	s := pcommon.NewIntSlice()
	s.Append(1, 2, 3)

	value, err := ctxutil.GetCommonIntSliceValue[any, int](context.Background(), nil, s, []ottl.Key[any]{
		&pathtest.Key[any]{
			I: ottltest.Intp(1),
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(s.At(1)), value)
}

func Test_GetCommonIntSliceValue_Invalid(t *testing.T) {
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
			err: errors.New(`unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'`),
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
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
					I: ottltest.Intp(-1),
					G: getSetter,
				},
			},
			err: errors.New("index -1 out of bounds"),
		},
		{
			name: "invalid key type",
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
			err: errors.New("type pcommon.IntSlice does not support indexing"),
		},
		{
			name: "nil key",
			keys: nil,
			err:  errors.New("cannot get slice value without key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewIntSlice()
			s.Append(1)

			_, err := ctxutil.GetCommonIntSliceValue[any, int](context.Background(), nil, s, tt.keys)
			assert.Equal(t, tt.err.Error(), err.Error())
		})
	}
}

func Test_SetCommonIntSliceValue_Valid(t *testing.T) {
	s := pcommon.NewInt32Slice()
	s.Append(1, 2, 3)

	for _, val := range []any{
		1, int8(1), int16(1), int32(1), int64(1), uint(1), uint8(1), uint16(1), uint32(1), uint64(1),
	} {
		t.Run(fmt.Sprintf("from %T", val), func(t *testing.T) {
			err := ctxutil.SetCommonIntSliceValue[any, int32](context.Background(), nil, s, []ottl.Key[any]{
				&pathtest.Key[any]{I: ottltest.Intp(1)},
			}, val)
			assert.NoError(t, err)
			assert.Equal(t, int32(1), s.At(1))
		})
	}
}

func Test_SetCommonIntSliceValue_Invalid(t *testing.T) {
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
		val  any
	}{
		{
			name: "first key not an integer",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					S: ottltest.Strp("key"),
					G: getSetter,
				},
			},
			err: errors.New(`unable to resolve an integer index in slice: could not resolve key for map/slice, expecting 'int64' but got '<nil>'`),
		},
		{
			name: "index too large",
			keys: []ottl.Key[any]{
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
					I: ottltest.Intp(-1),
					G: getSetter,
				},
			},
			err: errors.New("index -1 out of bounds"),
		},
		{
			name: "invalid key type",
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
			err: errors.New("type pcommon.Int32Slice does not support indexing"),
		},
		{
			name: "invalid value type",
			keys: []ottl.Key[any]{
				&pathtest.Key[any]{
					I: ottltest.Intp(0),
				},
			},
			val: "one",
			err: errors.New("invalid type provided for setting a slice of int32: string"),
		},
		{
			name: "nil key",
			keys: nil,
			err:  errors.New("cannot set slice value without key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewInt32Slice()
			s.Append(1)

			var val any
			if tt.val != nil {
				val = tt.val
			} else {
				val = 1
			}

			err := ctxutil.SetCommonIntSliceValue[any, int32](context.Background(), nil, s, tt.keys, val)
			assert.Equal(t, tt.err.Error(), err.Error())
		})
	}
}

func Test_SetCommonIntSliceValues(t *testing.T) {
	st := pcommon.NewInt32Slice()
	st.FromRaw([]int32{1, 2, 3})

	ps := pcommon.NewSlice()
	err := ps.FromRaw([]any{1, 2, 3})
	assert.NoError(t, err)

	invalid := pcommon.NewSlice()
	err = invalid.FromRaw([]any{"one", 1, "three"})
	assert.NoError(t, err)

	tests := []struct {
		name      string
		val       any
		want      []int32
		wantError string
	}{
		{
			name: "from any slice",
			val:  []any{int64(1), int64(2), int64(3)},
			want: []int32{1, 2, 3},
		},
		{
			name: "from int64 slice",
			val:  []int64{1, 2, 3},
			want: []int32{1, 2, 3},
		},
		{
			name: "from typed value",
			val:  []int32{1, 2, 3},
			want: []int32{1, 2, 3},
		},
		{
			name: "from the same slice type",
			val:  st,
			want: []int32{1, 2, 3},
		},
		{
			name: "from pcommon.Slice",
			val:  ps,
			want: []int32{1, 2, 3},
		},
		{
			name:      "from invalid type",
			val:       "one",
			wantError: "cannot set a slice of string from a value type: int32",
		},
		{
			name:      "from slice with invalid value",
			val:       invalid,
			wantError: "invalid value type provided for slice of int32: string",
		},
		{
			name:      "from any slice with invalid value",
			val:       []any{int64(1), "2", int64(3)},
			wantError: "invalid value type provided for a slice of int32: string, expected int64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := pcommon.NewInt32Slice()
			err := ctxutil.SetCommonIntSliceValues[int32](s, tt.val)
			if tt.wantError != "" {
				assert.ErrorContains(t, err, tt.wantError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, s.AsRaw())
			}
		})
	}
}

func Test_GetCommonIntSliceValues(t *testing.T) {
	st := pcommon.NewInt32Slice()
	st.FromRaw([]int32{1, 2, 3})
	assert.Equal(t, []int64{1, 2, 3}, ctxutil.GetCommonIntSliceValues[int32](st))
}
