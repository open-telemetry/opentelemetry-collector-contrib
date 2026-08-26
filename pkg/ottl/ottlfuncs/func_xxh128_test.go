// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_XXH128(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
		err      bool
	}{
		{
			name:     "string",
			value:    "hello world",
			expected: "df8d09e93f874900a99b8775cc15b6c7",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "99aa06d3014798d86001c324468d497f",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := xxh128HashString[any](&ottl.StandardStringGetter[any]{
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

func Test_XXH128Error(t *testing.T) {
	tests := []struct {
		name          string
		value         any
		err           bool
		expectedError string
	}{
		{
			name:          "non-string",
			value:         10,
			expectedError: "expected string but got int",
		},
		{
			name:          "nil",
			value:         nil,
			expectedError: "expected string but got nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := xxh128HashString[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			_, err := exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func Test_XXH128Factory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewXXH128Factory[any]()
		assert.Equal(t, "XXH128", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewXXH128Factory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &XXH128Arguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewXXH128Factory[any]()
		args := factory.CreateDefaultArguments()
		XXH128Args, ok := args.(*XXH128Arguments[any])
		require.True(t, ok)
		XXH128Args.Target = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "hello", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createXXH128Function[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "XXH128Factory args must be of type *XXH128Arguments[K]")
	})
}
