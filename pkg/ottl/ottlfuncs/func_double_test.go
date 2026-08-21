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

func Test_Double(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
		err      bool
	}{
		{
			name:     "string",
			value:    "50",
			expected: float64(50),
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
			expected: float64(333),
		},
		{
			name:     "float64",
			value:    float64(2.7),
			expected: float64(2.7),
		},
		{
			name:     "float64 without decimal",
			value:    float64(55),
			expected: float64(55),
		},
		{
			name:     "true",
			value:    true,
			expected: float64(1),
		},
		{
			name:     "false",
			value:    false,
			expected: float64(0),
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
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exprFunc := doubleFunc[any](&ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return test.value, nil
				},
			})
			result, err := exprFunc(nil, nil)
			if test.err {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, test.expected, result)
		})
	}
}

func Test_DoubleFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewDoubleFactory[any]()
		assert.Equal(t, "Double", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewDoubleFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &DoubleArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewDoubleFactory[any]()
		args := factory.CreateDefaultArguments()
		doubleArgs, ok := args.(*DoubleArguments[any])
		require.True(t, ok)
		doubleArgs.Target = &ottl.StandardFloatLikeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "42.0", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createDoubleFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "DoubleFactory args must be of type *DoubleArguments[K]")
	})
}
