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

func Test_Luhn(t *testing.T) {
	noErrorTests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "valid number string",
			value:    "17893729974",
			expected: true,
		},
		{
			name:     "valid number string with spaces",
			value:    "1789 3729 974",
			expected: true,
		},
		{
			name:     "empty string",
			value:    "",
			expected: false,
		},
		{
			name:     "single digit",
			value:    "0",
			expected: true,
		},
		{
			name:     "valid number",
			value:    17893729974,
			expected: true,
		},
		{
			name:     "invalid number string",
			value:    "17893729975",
			expected: false,
		},
	}

	for _, tt := range noErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := isValidLuhnFunc[any](&ottl.StandardStringLikeGetter[any]{
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
			name:     "float values are not allowed",
			value:    30.3,
			errorStr: "invalid",
		},
		{
			name:     "nil",
			value:    nil,
			errorStr: "invalid",
		},
		{
			name:     "some struct",
			value:    struct{}{},
			errorStr: "invalid",
		},
	}
	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := isValidLuhnFunc[any](&ottl.StandardStringLikeGetter[any]{
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

func Test_IsValidLuhnFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewIsValidLuhnFactory[any]()
		assert.Equal(t, "IsValidLuhn", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewIsValidLuhnFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &IsValidLuhnArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewIsValidLuhnFactory[any]()
		args := factory.CreateDefaultArguments()
		isValidLuhnArgs, ok := args.(*IsValidLuhnArguments[any])
		require.True(t, ok)
		isValidLuhnArgs.Target = &ottl.StandardStringLikeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "18", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createIsValidLuhnFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "IsValidLuhnFactory args must be of type *IsValidLuhnArguments[K]")
	})
}
