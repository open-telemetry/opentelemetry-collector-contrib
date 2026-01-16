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

func Test_index_native_slices(t *testing.T) {
	tests := []struct {
		name     string
		source   any
		value    any
		expected int64
	}{
		// String tests
		{
			name:     "find substring in middle",
			source:   "hello world",
			value:    "world",
			expected: 6,
		},
		{
			name:     "find substring at beginning",
			source:   "hello world",
			value:    "hello",
			expected: 0,
		},
		{
			name:     "substring not found",
			source:   "hello world",
			value:    "universe",
			expected: -1,
		},
		{
			name:     "empty substring",
			source:   "hello world",
			value:    "",
			expected: 0, // Empty string is always found at index 0
		},
		{
			name:     "case sensitive search",
			source:   "Hello World",
			value:    "world",
			expected: -1, // Case-sensitive, so 'world' isn't in 'Hello World'
		},
		// pcommon.Slice tests
		{
			name: "pcommon.Slice string slice with string value",
			source: func() pcommon.Slice {
				slice := pcommon.NewSlice()
				slice.AppendEmpty().SetStr("hello")
				slice.AppendEmpty().SetStr("world")
				slice.AppendEmpty().SetStr("opentelemetry")
				return slice
			}(),
			value:    "world",
			expected: 1,
		},
		{
			name: "pcommon.Slice int slice with int value",
			source: func() pcommon.Slice {
				slice := pcommon.NewSlice()
				slice.AppendEmpty().SetInt(1)
				slice.AppendEmpty().SetInt(2)
				slice.AppendEmpty().SetInt(3)
				return slice
			}(),
			value:    int64(2),
			expected: 1,
		},
		{
			name: "pcommon.Slice mixed type slice with bool value",
			source: func() pcommon.Slice {
				slice := pcommon.NewSlice()
				slice.AppendEmpty().SetStr("hello")
				slice.AppendEmpty().SetInt(42)
				slice.AppendEmpty().SetBool(true)
				return slice
			}(),
			value:    true,
			expected: 2,
		},
		{
			name: "pcommon.Slice value not found in slice",
			source: func() pcommon.Slice {
				slice := pcommon.NewSlice()
				slice.AppendEmpty().SetInt(1)
				slice.AppendEmpty().SetInt(2)
				slice.AppendEmpty().SetInt(3)
				return slice
			}(),
			value:    int64(5),
			expected: -1,
		},
		{
			name:     "pcommon.Slice empty slice",
			source:   pcommon.NewSlice(),
			value:    "anything",
			expected: -1,
		},
		// []any slice tests
		{
			name:     "[]any slice with string value",
			source:   []any{"hello", "world", "opentelemetry"},
			value:    "world",
			expected: 1,
		},
		{
			name:     "[]any slice with int value",
			source:   []any{1, 2, 3, 4},
			value:    3,
			expected: -1, // []any gets converted to pcommon.Slice, but int vs int64 comparison fails
		},
		{
			name:     "[]any slice with int64 value",
			source:   []any{int64(1), int64(2), int64(3), int64(4)},
			value:    int64(3),
			expected: 2, // int64 values work
		},
		{
			name:     "[]any slice with mixed types",
			source:   []any{"hello", 42, true, 3.14},
			value:    true,
			expected: 2,
		},
		{
			name:     "[]any slice value not found",
			source:   []any{"hello", "world"},
			value:    "universe",
			expected: -1,
		},
		// []string slice tests
		{
			name:     "[]string slice with string value",
			source:   []string{"apple", "banana", "cherry"},
			value:    "banana",
			expected: 1,
		},
		{
			name:     "[]string slice with non-string value",
			source:   []string{"apple", "banana", "cherry"},
			value:    123,
			expected: -1,
		},
		{
			name:     "[]string slice value not found",
			source:   []string{"apple", "banana", "cherry"},
			value:    "orange",
			expected: -1,
		},
		// []int64 slice tests (int gets converted to int64, so int values work with int64 comparison)
		{
			name:     "[]int64 slice with int64 value",
			source:   []int64{1000, 2999, 3111},
			value:    int64(2999),
			expected: 1,
		},
		// []float64 slice tests
		{
			name:     "[]float64 slice with float64 value",
			source:   []float64{1.1, 2.2, 3.3},
			value:    2.2,
			expected: 1,
		},
		// []bool slice tests
		{
			name:     "[]bool slice with bool value true",
			source:   []bool{false, true, false},
			value:    true,
			expected: 1,
		},
		{
			name:     "[]bool slice with bool value false",
			source:   []bool{false, true, false},
			value:    false,
			expected: 0,
		},
		{
			name:     "[]bool slice with non-bool value",
			source:   []bool{false, true, false},
			value:    "true",
			expected: -1,
		},
		// Edge cases
		{
			name:     "empty []int slice",
			source:   []int{},
			value:    1,
			expected: -1,
		},
		{
			name:     "empty []string slice",
			source:   []string{},
			value:    "test",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceExpr := ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.source, nil
				},
			}
			valueExpr := ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			}

			indexFn := index(ottl.NewValueComparator(), sourceExpr, valueExpr)
			result, err := indexFn(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_index_error_cases(t *testing.T) {
	tests := []struct {
		name        string
		source      any
		value       any
		expected    int64
		expectedErr string
	}{
		{
			name:        "source not string or pcommon.Slice",
			source:      123,
			value:       "test",
			expectedErr: "expected pcommon.Slice but got int",
		},
		{
			name:        "string source with non-string value",
			source:      "hello world",
			value:       123,
			expectedErr: "invalid value type for Index function, value must be a string",
		},
		{
			name: "pcommon.Value with slice type",
			source: func() pcommon.Value {
				val := pcommon.NewValueSlice()
				slice := val.Slice()
				slice.AppendEmpty().SetStr("hello")
				slice.AppendEmpty().SetStr("world")
				return val
			}(),
			value:    "world",
			expected: 1,
		},
		{
			name: "pcommon.Value with non-slice type (string)",
			source: func() pcommon.Value {
				val := pcommon.NewValueStr("not a slice")
				return val
			}(),
			value:       "test",
			expectedErr: "expected pcommon.Slice but got Str",
		},
		{
			name: "pcommon.Value with non-slice type (int)",
			source: func() pcommon.Value {
				val := pcommon.NewValueInt(42)
				return val
			}(),
			value:       42,
			expectedErr: "expected pcommon.Slice but got Int",
		},
		{
			name: "pcommon.Value with non-slice type (bool)",
			source: func() pcommon.Value {
				val := pcommon.NewValueBool(true)
				return val
			}(),
			value:       true,
			expectedErr: "expected pcommon.Slice but got Bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceExpr := ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.source, nil
				},
			}
			valueExpr := ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			}

			indexFn := index(ottl.NewValueComparator(), sourceExpr, valueExpr)
			result, err := indexFn(t.Context(), nil)

			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func Test_IndexFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewIndexFactory[any]()
		assert.Equal(t, "Index", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewIndexFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &IndexArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewIndexFactory[any]()
		args := factory.CreateDefaultArguments()
		// Set up the arguments appropriately
		indexArgs, ok := args.(*IndexArguments[any])
		require.True(t, ok)
		indexArgs.Target = ottl.StandardGetSetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "test source", nil
			},
		}
		indexArgs.Value = ottl.StandardGetSetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "test", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		// This tests the error case in createIndexFunction
		_, err := createIndexFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.Error(t, err)
		assert.ErrorContains(t, err, "IndexFactory args must be of type *IndexArguments[K]")
	})
}
