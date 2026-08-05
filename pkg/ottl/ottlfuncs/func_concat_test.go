// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_concat(t *testing.T) {
	tests := []struct {
		name      string
		vals      ottl.StandardStringLikeSliceGetter[any]
		delimiter ottl.StringGetter[any]
		expected  string
	}{
		{
			name: "concat strings",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"hello", "world"}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " ", nil }},
			expected:  "hello world",
		},
		{
			name: "concat []string (e.g. output of Split)",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []string{"a", "b", "c"}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "abc",
		},
		{
			name: "nil element",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"hello", nil, "world"}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "helloworld",
		},
		{
			name: "nil value",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return nil, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "-", nil }},
			expected:  "",
		},
		{
			name: "integers",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"hello", int64(1)}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "hello1",
		},
		{
			name: "floats",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"hello", 3.14159}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "hello3.14159",
		},
		{
			name: "booleans",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"hello", true}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " ", nil }},
			expected:  "hello true",
		},
		{
			name: "byte slices",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8}}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "00000000000000000ed2e63cbe71f5a8",
		},
		{
			name: "nested pcommon.Slice elements",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					s1 := pcommon.NewSlice()
					_ = s1.FromRaw([]any{1, 2})
					s2 := pcommon.NewSlice()
					_ = s2.FromRaw([]any{3, 4})
					return []any{s1, s2}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			expected:  "[1,2],[3,4]",
		},
		{
			name: "maps",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					m1 := pcommon.NewMap()
					m1.PutStr("a", "b")
					m2 := pcommon.NewMap()
					m2.PutStr("c", "d")
					return []any{m1, m2}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			expected:  `{"a":"b"},{"c":"d"}`,
		},
		{
			name: "pcommon.Slice value",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					s := pcommon.NewSlice()
					_ = s.FromRaw([]any{"a", "b", 1})
					return s, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "-", nil }},
			expected:  "a-b-1",
		},
		{
			name: "pcommon.Value slice",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					v := pcommon.NewValueSlice()
					_ = v.Slice().FromRaw([]any{"a", "b"})
					return v, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "-", nil }},
			expected:  "a-b",
		},
		{
			name: "empty string values",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"", "", ""}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "__", nil }},
			expected:  "____",
		},
		{
			name: "single argument",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{"hello"}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "-", nil }},
			expected:  "hello",
		},
		{
			name: "no arguments",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "-", nil }},
			expected:  "",
		},
		{
			name: "no arguments with an empty delimiter",
			vals: ottl.StandardStringLikeSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return []any{}, nil
				},
			},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := concat[any](tt.vals, tt.delimiter)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_concat_error(t *testing.T) {
	vals := &ottl.StandardStringLikeSliceGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return []any{make(chan int)}, nil
		},
	}
	delimiter := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "test", nil
		},
	}
	exprFunc := concat[any](vals, delimiter)
	_, err := exprFunc(t.Context(), nil)
	assert.Error(t, err)
}

func Test_concat_error_delimiter(t *testing.T) {
	vals := &ottl.StandardStringLikeSliceGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return []any{"a"}, nil
		},
	}
	delimiter := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return 3, nil
		},
	}
	exprFunc := concat[any](vals, delimiter)
	_, err := exprFunc(t.Context(), nil)
	assert.Error(t, err)
}
