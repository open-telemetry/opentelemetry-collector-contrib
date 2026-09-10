// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_IsEmpty(t *testing.T) {
	nonEmptyMap := pcommon.NewMap()
	nonEmptyMap.PutStr("k", "v")

	nonEmptySlice := pcommon.NewSlice()
	nonEmptySlice.AppendEmpty().SetStr("v")

	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "nil",
			value:    nil,
			expected: true,
		},
		{
			name:     "empty string",
			value:    "",
			expected: true,
		},
		{
			name:     "non-empty string",
			value:    "hello",
			expected: false,
		},
		{
			name:     "zero int",
			value:    0,
			expected: true,
		},
		{
			name:     "non-zero int",
			value:    1,
			expected: false,
		},
		{
			name:     "zero int64",
			value:    int64(0),
			expected: true,
		},
		{
			name:     "zero float64",
			value:    float64(0),
			expected: true,
		},
		{
			name:     "non-zero float64",
			value:    1.5,
			expected: false,
		},
		{
			name:     "false bool",
			value:    false,
			expected: true,
		},
		{
			name:     "true bool",
			value:    true,
			expected: false,
		},
		{
			name:     "empty pcommon.Value",
			value:    pcommon.NewValueEmpty(),
			expected: true,
		},
		{
			name:     "non-empty pcommon.Value",
			value:    pcommon.NewValueStr("v"),
			expected: false,
		},
		{
			name:     "empty pcommon.Map",
			value:    pcommon.NewMap(),
			expected: true,
		},
		{
			name:     "non-empty pcommon.Map",
			value:    nonEmptyMap,
			expected: false,
		},
		{
			name:     "empty pcommon.Slice",
			value:    pcommon.NewSlice(),
			expected: true,
		},
		{
			name:     "non-empty pcommon.Slice",
			value:    nonEmptySlice,
			expected: false,
		},
		{
			name:     "empty []any",
			value:    []any{},
			expected: true,
		},
		{
			name:     "non-empty []any",
			value:    []any{"v"},
			expected: false,
		},
		{
			name:     "empty map[string]any",
			value:    map[string]any{},
			expected: true,
		},
		{
			name:     "non-empty map[string]any",
			value:    map[string]any{"k": "v"},
			expected: false,
		},
		{
			name:     "empty []byte",
			value:    []byte{},
			expected: true,
		},
		{
			name:     "non-empty []byte",
			value:    []byte("v"),
			expected: false,
		},
		{
			name:     "empty []string",
			value:    []string{},
			expected: true,
		},
		{
			name:     "non-empty []string",
			value:    []string{"v"},
			expected: false,
		},
		{
			name:     "empty []int64",
			value:    []int64{},
			expected: true,
		},
		{
			name:     "non-empty []int64",
			value:    []int64{1},
			expected: false,
		},
		{
			name:     "unset pcommon.TraceID",
			value:    pcommon.NewTraceIDEmpty(),
			expected: true,
		},
		{
			name:     "set pcommon.TraceID",
			value:    pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}),
			expected: false,
		},
		{
			name:     "unset pcommon.SpanID",
			value:    pcommon.NewSpanIDEmpty(),
			expected: true,
		},
		{
			name:     "set pcommon.SpanID",
			value:    pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}),
			expected: false,
		},
		{
			name:     "nil pointer",
			value:    (*string)(nil),
			expected: true,
		},
		{
			name:     "pointer to empty value",
			value:    func() *string { s := ""; return &s }(),
			expected: true,
		},
		{
			name:     "pointer to non-empty value",
			value:    func() *string { s := "v"; return &s }(),
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := isEmpty[any](&ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_IsEmpty_Error(t *testing.T) {
	exprFunc := isEmpty[any](&ottl.StandardGetSetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, errors.New("error getting value")
		},
	})
	result, err := exprFunc(t.Context(), nil)
	assert.Equal(t, false, result)
	assert.Error(t, err)
}

func BenchmarkIsEmpty(b *testing.B) {
	nonEmptyMap := pcommon.NewMap()
	nonEmptyMap.PutStr("k", "v")

	nonEmptySlice := pcommon.NewSlice()
	nonEmptySlice.AppendEmpty().SetStr("v")

	benchmarks := []struct {
		name  string
		value any
	}{
		{name: "nil", value: nil},
		{name: "empty_string", value: ""},
		{name: "non_empty_string", value: "hello"},
		{name: "empty_int64", value: int64(0)},
		{name: "non_empty_int64", value: int64(1)},
		{name: "empty_float64", value: float64(0)},
		{name: "non_empty_float64", value: 1.5},
		{name: "empty_bool", value: false},
		{name: "non_empty_bool", value: true},
		{name: "empty_pcommon.Value", value: pcommon.NewValueEmpty()},
		{name: "non_empty_pcommon.Value", value: pcommon.NewValueStr("v")},
		{name: "empty_pcommon.Map", value: pcommon.NewMap()},
		{name: "non_empty_pcommon.Map", value: nonEmptyMap},
		{name: "empty_pcommon.Slice", value: pcommon.NewSlice()},
		{name: "non_empty_pcommon.Slice", value: nonEmptySlice},
		{name: "empty_[]any", value: []any{}},
		{name: "non_empty_[]any", value: []any{"v"}},
		{name: "empty_map[string]any", value: map[string]any{}},
		{name: "non_empty_map[string]any", value: map[string]any{"k": "v"}},
		{name: "empty_[]byte", value: []byte{}},
		{name: "non_empty_[]byte", value: []byte("v")},
		{name: "empty_pcommon.TraceID", value: pcommon.NewTraceIDEmpty()},
		{name: "non_empty_pcommon.TraceID", value: pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})},
		{name: "empty_pcommon.SpanID", value: pcommon.NewSpanIDEmpty()},
		{name: "non_empty_pcommon.SpanID", value: pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})},
	}
	ctx := b.Context()
	for _, bm := range benchmarks {
		exprFunc := isEmpty[any](&ottl.StandardGetSetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return bm.value, nil
			},
		})
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, err := exprFunc(ctx, nil)
				require.NoError(b, err)
			}
		})
	}
}
