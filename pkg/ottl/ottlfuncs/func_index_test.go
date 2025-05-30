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

func Test_index_string(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		value    string
		expected int64
	}{
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

			indexFn := index(sourceExpr, valueExpr)
			result, err := indexFn(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_index_pcommon_slice(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() pcommon.Slice
		value    any
		expected int64
	}{
		{
			name: "string slice with string value",
			setup: func() pcommon.Slice {
				slice := pcommon.NewSlice()
				slice.AppendEmpty().SetStr("hello")
				slice.AppendEmpty().SetStr("world")
				slice.AppendEmpty().SetStr("opentelemetry")
				return slice
			},
			value:    "world",
			expected: 1,
		},
		{
			name: "int slice with int value",
			setup: func() pcommon.Slice {
				slice := pcommon.NewSlice()
				slice.AppendEmpty().SetInt(1)
				slice.AppendEmpty().SetInt(2)
				slice.AppendEmpty().SetInt(3)
				return slice
			},
			value:    int64(2),
			expected: 1,
		},
		{
			name: "mixed type slice with bool value",
			setup: func() pcommon.Slice {
				slice := pcommon.NewSlice()
				slice.AppendEmpty().SetStr("hello")
				slice.AppendEmpty().SetInt(42)
				slice.AppendEmpty().SetBool(true)
				return slice
			},
			value:    true,
			expected: 2,
		},
		{
			name: "value not found in slice",
			setup: func() pcommon.Slice {
				slice := pcommon.NewSlice()
				slice.AppendEmpty().SetInt(1)
				slice.AppendEmpty().SetInt(2)
				slice.AppendEmpty().SetInt(3)
				return slice
			},
			value:    int64(5),
			expected: -1,
		},
		{
			name:     "empty slice",
			setup:    pcommon.NewSlice,
			value:    "anything",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slice := tt.setup()

			sourceExpr := ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return slice, nil
				},
			}
			valueExpr := ottl.StandardGetSetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			}

			indexFn := index(sourceExpr, valueExpr)
			result, err := indexFn(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_index_error_cases(t *testing.T) {
	tests := []struct {
		name        string
		source      any
		value       any
		expectedErr string
	}{
		{
			name:        "source not string or pcommon.Slice",
			source:      123,
			value:       "test",
			expectedErr: "source must be string or slice type",
		},
		{
			name:        "string source with non-string value",
			source:      "hello world",
			value:       123,
			expectedErr: "when source is string, value must also be string",
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

			indexFn := index(sourceExpr, valueExpr)
			_, err := indexFn(context.Background(), nil)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
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
		indexArgs.Source = ottl.StandardGetSetter[any]{
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
		assert.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		// This tests the error case in createIndexFunction
		_, err := createIndexFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "IndexFactory args must be of type *IndexArguments[K]")
	})
}
