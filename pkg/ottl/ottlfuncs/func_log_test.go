// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Log(t *testing.T) {
	noErrorTests := []struct {
		name     string
		value    any
		expected any
	}{
		{
			name:     "string",
			value:    "50",
			expected: math.Log(50),
		},
		{
			name:     "int64",
			value:    int64(333),
			expected: math.Log(333),
		},
		{
			name:     "float64",
			value:    float64(2.7),
			expected: math.Log(2.7),
		},
		{
			name:     "float64 without decimal",
			value:    float64(55),
			expected: math.Log(55),
		},
		{
			name:     "true",
			value:    true, // casts to 1 which Log(1) is 0 so it works.
			expected: float64(0),
		},
	}
	for _, tt := range noErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := logFunc[any](&ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
	errorTests := []struct {
		name     string
		value    any
		errorStr string
	}{
		{
			name:     "not a number string",
			value:    "test",
			errorStr: "invalid",
		},
		{
			name:     "false",
			value:    false,
			errorStr: "invalid",
		},
		{
			name:     "zero is undefined",
			value:    0.0,
			errorStr: "greater than zero",
		},
		{
			name:     "negative is undefined",
			value:    -30.3,
			errorStr: "greater than zero",
		},
		{
			name:     "nil",
			value:    nil,
			errorStr: "invalid",
		},
		{
			name:     "some struct",
			value:    struct{}{},
			errorStr: "unsupported",
		},
	}
	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := logFunc[any](&ottl.StandardFloatLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.errorStr)
			assert.Nil(t, result)
		})
	}
}

func Test_LogFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewLogFactory[any]()
		assert.Equal(t, "Log", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewLogFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &LogArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewLogFactory[any]()
		args := factory.CreateDefaultArguments()
		logArgs, ok := args.(*LogArguments[any])
		require.True(t, ok)
		logArgs.Target = ottl.StandardFloatLikeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return float64(10), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createLogFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "LogFactory args must be of type *LogArguments[K]")
	})
}
