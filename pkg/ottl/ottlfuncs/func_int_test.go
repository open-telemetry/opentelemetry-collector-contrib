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

func Test_Int(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
		err      bool
	}{
		{
			name:     "string",
			value:    "50",
			expected: int64(50),
		},
		{
			name:     "empty string",
			value:    "",
			expected: nil,
			err:      true,
		},
		{
			name:     "not a number string",
			value:    "test",
			expected: nil,
			err:      true,
		},
		{
			name:     "int64",
			value:    int64(333),
			expected: int64(333),
		},
		{
			name:     "float64",
			value:    float64(2.7),
			expected: int64(2),
		},
		{
			name:     "float64 without decimal",
			value:    float64(55),
			expected: int64(55),
		},
		{
			name:     "true",
			value:    true,
			expected: int64(1),
		},
		{
			name:     "false",
			value:    false,
			expected: int64(0),
		},
		{
			name:     "nil",
			value:    nil,
			expected: nil,
		},
		{
			name:     "some struct",
			value:    struct{}{},
			expected: nil,
			err:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := intFunc[any](&ottl.StandardIntLikeGetter[any]{
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

func Test_IntFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewIntFactory[any]()
		assert.Equal(t, "Int", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewIntFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &IntArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewIntFactory[any]()
		args := factory.CreateDefaultArguments()
		intArgs, ok := args.(*IntArguments[any])
		require.True(t, ok)
		intArgs.Target = &ottl.StandardIntLikeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "42", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createIntFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "IntFactory args must be of type *IntArguments[K]")
	})
}
