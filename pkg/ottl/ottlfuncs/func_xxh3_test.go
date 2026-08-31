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

func Test_XXH3(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
		err      bool
	}{
		{
			name:     "string",
			value:    "hello world",
			expected: "d447b1ea40e6988b",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "2d06800538d394c2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := xxh3HashString[any](&ottl.StandardStringGetter[any]{
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

func Test_XXH3Error(t *testing.T) {
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
			exprFunc := xxh3HashString[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			_, err := exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func Test_XXH3Factory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewXXH3Factory[any]()
		assert.Equal(t, "XXH3", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewXXH3Factory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &XXH3Arguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewXXH3Factory[any]()
		args := factory.CreateDefaultArguments()
		XXH3Args, ok := args.(*XXH3Arguments[any])
		require.True(t, ok)
		XXH3Args.Target = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "hello", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createXXH3Function[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "XXH3Factory args must be of type *XXH3Arguments[K]")
	})
}
