// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_String(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
		err      bool
	}{
		{
			name:     "string",
			value:    "test",
			expected: string("test"),
		},
		{
			name:     "empty string",
			value:    "",
			expected: string(""),
		},
		{
			name:     "a number string",
			value:    "333",
			expected: string("333"),
		},
		{
			name:     "int64",
			value:    int64(333),
			expected: string("333"),
		},
		{
			name:     "float64",
			value:    float64(2.7),
			expected: string("2.7"),
		},
		{
			name:     "float64 without decimal",
			value:    float64(55),
			expected: string("55"),
		},
		{
			name:     "true",
			value:    true,
			expected: string("true"),
		},
		{
			name:     "false",
			value:    false,
			expected: string("false"),
		},
		{
			name:     "nil",
			value:    nil,
			expected: nil,
		},
		{
			name:     "byte",
			value:    []byte{123},
			expected: string("7b"),
		},
		{
			name:     "map",
			value:    map[int]bool{1: true},
			expected: string("{\"1\":true}"),
		},
		{
			name:     "slice",
			value:    []int{1, 2, 3},
			expected: string("[1,2,3]"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := stringFunc(&ottl.StandardStringLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(nil, nil)
			if tt.err {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_StringFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewStringFactory[any]()
		assert.Equal(t, "String", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewStringFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &StringArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewStringFactory[any]()
		args := factory.CreateDefaultArguments()
		stringArgs, ok := args.(*StringArguments[any])
		require.True(t, ok)
		stringArgs.Target = &ottl.StandardStringLikeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "hello", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createStringFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "StringFactory args must be of type *StringArguments[K]")
	})
}
