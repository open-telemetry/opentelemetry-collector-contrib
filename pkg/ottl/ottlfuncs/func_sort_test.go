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
			name: "unsupported ValueTypeMap remains unchanged",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pMap, nil
				},
			},
			order:    sortAsc,
			expected: pMap,
		},
		{
			name: "unsupported bytes remains unchanged",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []byte("still fine"), nil
				},
			},
			order:    sortAsc,
			expected: []byte("still fine"),
		},
		{
			name: "invalid slice remains unchanged",
			getter: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []any{map[string]string{"some": "invalid kv"}}, nil
				},
			},
			order:    sortAsc,
			expected: []any{map[string]string{"some": "invalid kv"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Sort(tt.getter, tt.order)
			assert.NoError(t, err)
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
