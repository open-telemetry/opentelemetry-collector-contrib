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

func Test_toLowerCase(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StringGetter[any]
		expected any
	}{
		{
			name: "simple",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "SIMPLE", nil
				},
			},
			expected: "simple",
		},
		{
			name: "already lower",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "simple", nil
				},
			},
			expected: "simple",
		},
		{
			name: "complex",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "complex_SET-of.WORDS1234", nil
				},
			},
			expected: "complex_set-of.words1234",
		},
		{
			name: "empty string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "", nil
				},
			},
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := toLowerCase(tt.target)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_toLowerCaseRuntimeError(t *testing.T) {
	tests := []struct {
		name          string
		target        ottl.StringGetter[any]
		expectedError string
	}{
		{
			name: "non-string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return 10, nil
				},
			},
			expectedError: "expected string but got int",
		},
		{
			name: "nil",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return nil, nil
				},
			},
			expectedError: "expected string but got nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := toLowerCase[any](tt.target)
			_, err := exprFunc(t.Context(), nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func Test_ToLowerCaseFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewToLowerCaseFactory[any]()
		assert.Equal(t, "ToLowerCase", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewToLowerCaseFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &ToLowerCaseArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewToLowerCaseFactory[any]()
		args := factory.CreateDefaultArguments()
		createToLowerCaseArgs, ok := args.(*ToLowerCaseArguments[any])
		require.True(t, ok)
		createToLowerCaseArgs.Target = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "hello world", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createToLowerCaseFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "ToLowerCaseFactory args must be of type *ToLowerCaseArguments[K]")
	})
}
