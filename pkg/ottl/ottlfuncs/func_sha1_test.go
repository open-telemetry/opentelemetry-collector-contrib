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

func Test_SHA1(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
		err      bool
	}{
		{
			name:     "string",
			value:    "hello world",
			expected: "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := SHA1HashString[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			require.NoError(t, err)
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

func Test_SHA1Error(t *testing.T) {
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
			exprFunc, err := SHA1HashString[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			require.NoError(t, err)
			_, err = exprFunc(nil, nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}

func Test_SHA1Factory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewSHA1Factory[any]()
		assert.Equal(t, "SHA1", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewSHA1Factory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &SHA1Arguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewSHA1Factory[any]()
		args := factory.CreateDefaultArguments()
		shaArgs, ok := args.(*SHA1Arguments[any])
		require.True(t, ok)
		shaArgs.Target = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "hello", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createSHA1Function[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "SHA1Factory args must be of type *SHA1Arguments[K]")
	})
}
