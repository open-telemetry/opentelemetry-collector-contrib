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

func Test_toSnakeCase(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StringGetter[any]
		expected any
	}{
		{
			name: "simple toSnake",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "simpleString", nil
				},
			},
			expected: "simple_string",
		},
		{
			name: "noop already snake case",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "simple_string", nil
				},
			},
			expected: "simple_string",
		},
		{
			name: "multiple uppercase",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "CPUUtilizationMetric", nil
				},
			},
			expected: "cpu_utilization_metric",
		},
		{
			name: "hyphens",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "simple-string", nil
				},
			},
			expected: "simple_string",
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
			exprFunc := toSnakeCase(tt.target)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_toSnakeCaseRuntimeError(t *testing.T) {
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
			exprFunc := toSnakeCase[any](tt.target)
			_, err := exprFunc(t.Context(), nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func Test_ToSnakeCaseFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewToSnakeCaseFactory[any]()
		assert.Equal(t, "ToSnakeCase", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewToSnakeCaseFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &ToSnakeCaseArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewToSnakeCaseFactory[any]()
		args := factory.CreateDefaultArguments()
		createToSnakeCaseArgs, ok := args.(*ToSnakeCaseArguments[any])
		require.True(t, ok)
		createToSnakeCaseArgs.Target = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "hello world", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createToSnakeCaseFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "ToSnakeCaseFactory args must be of type *ToSnakeCaseArguments[K]")
	})
}
