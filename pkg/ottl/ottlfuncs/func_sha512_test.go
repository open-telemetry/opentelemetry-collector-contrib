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

func Test_SHA512(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any
		err      bool
	}{
		{
			name:     "empty string",
			value:    "",
			expected: "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e",
		},
		{
			name:     "string",
			value:    "foo bar",
			expected: "65019286222ace418f742556366f9b9da5aaf6797527d2f0cba5bfe6b2f8ed24746542a0f2be1da8d63c2477f688b608eb53628993afa624f378b03f10090ce7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := SHA512HashString[any](&ottl.StandardStringGetter[any]{
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

func Test_SHA512Error(t *testing.T) {
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
			exprFunc, err := SHA512HashString[any](&ottl.StandardStringGetter[any]{
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

func Test_SHA512Factory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewSHA512Factory[any]()
		assert.Equal(t, "SHA512", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewSHA512Factory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &SHA512Arguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewSHA512Factory[any]()
		args := factory.CreateDefaultArguments()
		shaArgs, ok := args.(*SHA512Arguments[any])
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
		_, err := createSHA512Function[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "SHA512Factory args must be of type *SHA512Arguments[K]")
	})
}
