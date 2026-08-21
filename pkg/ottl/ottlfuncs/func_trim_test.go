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

func Test_trim(t *testing.T) {
	tests := []struct {
		name        string
		target      ottl.StringGetter[any]
		replacement ottl.Optional[string]
		expected    any
		shouldError bool
	}{
		{
			name: "trim string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return " this is a test ", nil
				},
			},
			replacement: ottl.NewTestingOptional[string](" "),
			expected:    "this is a test",
			shouldError: false,
		},
		{
			name: "trim empty string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "", nil
				},
			},
			replacement: ottl.NewTestingOptional[string](" "),
			expected:    "",
			shouldError: false,
		},
		{
			name: "No replacement string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return " this is a test ", nil
				},
			},
			replacement: ottl.Optional[string]{},
			expected:    "this is a test",
			shouldError: false,
		},
		{
			name: "Set replacement string to \"\"",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return " this is a test ", nil
				},
			},
			replacement: ottl.NewTestingOptional[string](""),
			expected:    " this is a test ",
			shouldError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := trim(tt.target, tt.replacement)
			result, err := exprFunc(nil, nil)
			if tt.shouldError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_TrimFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewTrimFactory[any]()
		assert.Equal(t, "Trim", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewTrimFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &TrimArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target", "Replacement"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewTrimFactory[any]()
		args := factory.CreateDefaultArguments()
		trimArgs, ok := args.(*TrimArguments[any])
		require.True(t, ok)
		trimArgs.Target = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "  hello world  ", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createTrimFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "TrimFactory args must be of type *TrimArguments[K]")
	})
}
