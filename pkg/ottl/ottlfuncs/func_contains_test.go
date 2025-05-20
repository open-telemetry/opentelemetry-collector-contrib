// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Contains(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StandardPSliceGetter[any]
		item     ottl.Getter[any]
		expected bool
	}{
		{
			name: "find item in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{"hello", "world"}, nil
				},
			},
			item: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "hello", nil
				},
			},
			expected: true,
		},
		{
			name: "not find item in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{"hello", "world"}, nil
				},
			},
			item: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "unknown", nil
				},
			},
			expected: false,
		},
		{
			name: "find integers in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{0, 1}, nil
				},
			},
			item: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(1), nil
				},
			},
			expected: true,
		},
		{
			name: "find floats in taget",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{0, 3.14159}, nil
				},
			},
			item: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 3.14159, nil
				},
			},
			expected: true,
		},
		{
			name: "find booleans in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{true, false}, nil
				},
			},
			item: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return true, nil
				},
			},
			expected: true,
		},
		{
			name: "find pcommon.Slice in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewSlice()
					_ = s.FromRaw([]any{1, 2})
					return s, nil
				},
			},
			item: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(1), nil
				},
			},
			expected: true,
		},
		{
			name: "not find pcommon.Slice in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewSlice()
					_ = s.FromRaw([]any{1, 2})
					return s, nil
				},
			},
			item: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(4), nil
				},
			},
			expected: false,
		},
		{
			name: "not find pcommon.Slice in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewSlice()
					_ = s.FromRaw([]any{1, 2})
					return s, nil
				},
			},
			item: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(4), nil
				},
			},
			expected: false,
		},
		{
			name: "not find pcommon.Value in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewSlice()
					_ = s.FromRaw([]any{1, 4})
					return s, nil
				},
			},
			item: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewValueInt(4), nil
				},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			exprFunc := contains(tt.target, tt.item)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Contains_Error(t *testing.T) {
	target := &ottl.StandardPSliceGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return make(chan int), nil
		},
	}
	item := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "test", nil
		},
	}

	exprFunc := contains(target, item)
	_, err := exprFunc(context.Background(), nil)
	assert.Error(t, err)
}
