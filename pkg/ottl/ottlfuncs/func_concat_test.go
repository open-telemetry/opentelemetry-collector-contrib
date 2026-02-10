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
		vals      any
		delimiter ottl.StringGetter[any]
		expected  string
	}{
		{
			name:      "concat strings",
			vals:      []any{"hello", "world"},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " ", nil }},
			expected:  "hello world",
		},
		{
			name:      "nil",
			vals:      []any{"hello", nil, "world"},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "hello<nil>world",
		},
		{
			name:      "integers",
			vals:      []any{"hello", int64(1)},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "hello1",
		},
		{
			name:      "floats",
			vals:      []any{"hello", 3.14159},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "hello3.14159",
		},
		{
			name:      "booleans",
			vals:      []any{"hello", true},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return " ", nil }},
			expected:  "hello true",
		},
		{
			name:      "byte slices",
			vals:      []any{[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8}},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "00000000000000000ed2e63cbe71f5a8",
		},
		{
			name: "pcommon.Slice",
			vals: func() pcommon.Slice {
				s := pcommon.NewSlice()
				_ = s.FromRaw([]any{[]any{1, 2}, []any{3, 4}})
				return s
			}(),
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			expected:  "[1,2],[3,4]",
		},
		{
			name: "maps",
			vals: func() pcommon.Slice {
				s := pcommon.NewSlice()
				s.EnsureCapacity(2)
				s.AppendEmpty().SetEmptyMap().PutStr("a", "b")
				s.AppendEmpty().SetEmptyMap().PutStr("c", "d")
				return s
			}(),
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ",", nil }},
			expected:  `{"a":"b"},{"c":"d"}`,
		},
		{
			name:      "empty string values",
			vals:      []any{"", "", ""},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "__", nil }},
			expected:  "____",
		},
		{
			name:      "single argument",
			vals:      []any{"hello"},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "-", nil }},
			expected:  "hello",
		},
		{
			name:      "no arguments",
			vals:      []any{},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "-", nil }},
			expected:  "",
		},
		{
			name:      "no arguments with an empty delimiter",
			vals:      []any{},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "", nil }},
			expected:  "",
		},
		{
			name:      "native string slice",
			vals:      []string{"hello", "world"},
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return ":", nil }},
			expected:  "hello:world",
		},
		{
			name: "pcommon.Value slice",
			vals: func() pcommon.Value {
				v := pcommon.NewValueSlice()
				v.Slice().AppendEmpty().SetStr("hello")
				v.Slice().AppendEmpty().SetInt(7)
				return v
			}(),
			delimiter: ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) { return "-", nil }},
			expected:  "hello-7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valsGetter := ottl.StandardPSliceGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.vals, nil
				},
			}
			exprFunc := concat(valsGetter, tt.delimiter)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_concat_error(t *testing.T) {
	target := &ottl.StandardPSliceGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return make(chan int), nil
		},
	}
	delimiter := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "test", nil
		},
	}
	exprFunc := concat[any](target, delimiter)
	_, err := exprFunc(t.Context(), nil)
	assert.EqualError(t, err, "expected pcommon.Slice but got chan int")
}

func Test_concat_error_delimiter(t *testing.T) {
	target := &ottl.StandardPSliceGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return []any{"a"}, nil
		},
	}
	delimiter := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return 3, nil
		},
	}
	exprFunc := concat[any](target, delimiter)
	_, err := exprFunc(t.Context(), nil)
	assert.Error(t, err)
}
