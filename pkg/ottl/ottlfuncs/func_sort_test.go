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

func Test_Sort(t *testing.T) {

	pMap := pcommon.NewValueMap().SetEmptyMap()
	pMap.PutStr("k", "v")

	tests := []struct {
		name     string
		getter   ottl.Getter[any]
		order    string
		expected any
		err      bool
	}{
		{
			name: "int slice",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewValueSlice().SetEmptySlice()
					_ = s.FromRaw([]any{9, 6, 3})
					return s, nil
				},
			},
			order:    sortAsc,
			expected: []int64{3, 6, 9},
		},
		{
			name: "int slice desc",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewValueSlice().SetEmptySlice()
					_ = s.FromRaw([]any{3, 6, 9})
					return s, nil
				},
			},
			order:    sortDesc,
			expected: []int64{9, 6, 3},
		},
		{
			name: "string slice",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewValueSlice().SetEmptySlice()
					_ = s.FromRaw([]any{"i", "am", "awesome", "slice"})
					return s, nil
				},
			},
			order:    sortAsc,
			expected: []string{"am", "awesome", "i", "slice"},
		},
		{
			name: "double slice desc",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewValueSlice().SetEmptySlice()
					_ = s.FromRaw([]any{0.1829374652374, -3.4029435828374, 9.7425639845731})
					return s, nil
				},
			},
			order:    sortDesc,
			expected: []float64{9.7425639845731, 0.1829374652374, -3.4029435828374},
		},
		{
			name: "bool slice compares as string",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewValueSlice().SetEmptySlice()
					_ = s.FromRaw([]any{true, false, true, false})
					return s, nil
				},
			},
			order:    sortAsc,
			expected: []string{"false", "false", "true", "true"},
		},
		{
			name: "mixed types slice compares as string",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewValueSlice().SetEmptySlice()
					_ = s.FromRaw([]any{1, "two", 3.33, false})
					return s, nil
				},
			},
			order:    sortAsc,
			expected: []string{"1", "3.33", "false", "two"},
		},
		{
			name: "mixed numeric types slice compares as double",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewValueSlice().SetEmptySlice()
					_ = s.FromRaw([]any{0, 2, 3.33, 0})
					return s, nil
				},
			},
			order:    sortAsc,
			expected: []float64{0, 0, 2, 3.33},
		},
		{
			name: "mixed numeric types slice compares as double desc",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					s := pcommon.NewValueSlice().SetEmptySlice()
					_ = s.FromRaw([]any{3.14, 2, 3.33, 0})
					return s, nil
				},
			},
			order:    sortDesc,
			expected: []float64{3.33, 3.14, 2, 0},
		},
		{
			name: "[]any compares as string",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{1, "two", 3.33, false}, nil
				},
			},
			order:    sortAsc,
			expected: []string{"1", "3.33", "false", "two"},
		},
		{
			name: "[]string",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []string{"A", "a", "aa"}, nil
				},
			},
			order:    sortAsc,
			expected: []string{"A", "a", "aa"},
		},
		{
			name: "[]bool compares as string",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []bool{true, false}, nil
				},
			},
			order:    sortAsc,
			expected: []string{"false", "true"},
		},
		{
			name: "[]int64",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []int64{6, 3, 9}, nil
				},
			},
			order:    sortAsc,
			expected: []int64{3, 6, 9},
		},
		{
			name: "[]float64",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []float64{0.1829374652374, -3.4029435828374, 9.7425639845731}, nil
				},
			},
			order:    sortAsc,
			expected: []float64{-3.4029435828374, 0.1829374652374, 9.7425639845731},
		},
		{
			name: "pcommon.Value is a slice",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					pv := pcommon.NewValueEmpty()
					s := pv.SetEmptySlice()
					_ = s.FromRaw([]any{"a", "slice", "a"})
					return pv, nil
				},
			},
			order:    sortAsc,
			expected: []string{"a", "a", "slice"},
		},
		{
			name: "pcommon.Value is empty",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					pv := pcommon.NewValueEmpty()
					return pv, nil
				},
			},
			order:    sortAsc,
			expected: nil,
			err:      true,
		},
		{
			name: "unsupported ValueTypeMap",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pMap, nil
				},
			},
			order:    sortAsc,
			expected: nil,
			err:      true,
		},
		{
			name: "unsupported bytes",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []byte("still fine"), nil
				},
			},
			order:    sortAsc,
			expected: nil,
			err:      true,
		},
		{
			name: "unsupported string",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "no change", nil
				},
			},
			order:    sortAsc,
			expected: nil,
			err:      true,
		},
		{
			name: "[]any with a map",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{map[string]string{"some": "invalid kv"}}, nil
				},
			},
			order:    sortAsc,
			expected: nil,
			err:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := sort(tt.getter, tt.order)
			result, err := exprFunc(nil, nil)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}
