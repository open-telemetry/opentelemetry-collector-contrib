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

func Test_contains(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StandardPSliceGetter[any]
		item     string
		expected bool
	}{
		{
			name: "find item in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []string{"hello", "world"}, nil
				},
			},
			item:     "hello",
			expected: true,
		},
		{
			name: "not find item in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []string{"hello", "world"}, nil
				},
			},
			item:     "unknow",
			expected: false,
		},
		{
			name: "find integers in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []int{0, 1}, nil
				},
			},
			item:     "1",
			expected: true,
		},
		{
			name: "find floats in taget",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []float64{0, 3.14159}, nil
				},
			},
			item:     "3.14159",
			expected: true,
		},
		{
			name: "find booleans in target",
			target: ottl.StandardPSliceGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []bool{true, false}, nil
				},
			},
			item:     "true",
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
			item:     "1",
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
			item:     "4",
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

func Test_contains_error(t *testing.T) {
	target := &ottl.StandardPSliceGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return make(chan int), nil
		},
	}
	exprFunc := contains(target, "test")
	_, err := exprFunc(context.Background(), nil)
	assert.Error(t, err)
}
