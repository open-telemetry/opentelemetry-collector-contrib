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

func Test_Murmur3Hash128(t *testing.T) {
	tests := []struct {
		name          string
		value         any
		expected      any
		err           bool
		expectedError string
	}{
		{
			name:     "string",
			value:    "Hello World",
			expected: "dbc2a0c1ab26631a27b4c09fcf1fe683",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "00000000000000000000000000000000",
		},
		{
			name:          "non-string",
			value:         123,
			err:           true,
			expectedError: "expected string but got int",
		},
		{
			name:          "nil",
			value:         nil,
			err:           true,
			expectedError: "expected string but got nil",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := murmur3Hash128[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(nil, nil)
			if tt.err {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func Test_CreateMurmur3Hash128Func(t *testing.T) {
	factory := NewMurmur3Hash128Factory[any]()
	fCtx := ottl.FunctionContext{}

	// invalid args
	exprFunc, err := factory.CreateFunction(fCtx, nil)
	assert.Error(t, err)
	assert.Nil(t, exprFunc)

	// valid args
	exprFunc, err = factory.CreateFunction(
		fCtx, &Murmur3Hash128Arguments[any]{
			Target: ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "Hello World", nil
				},
			},
		},
	)
	require.NoError(t, err)
	assert.NotNil(t, exprFunc)
}

func Test_Murmur3Hash128Factory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewMurmur3Hash128Factory[any]()
		assert.Equal(t, "Murmur3Hash128", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewMurmur3Hash128Factory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &Murmur3Hash128Arguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewMurmur3Hash128Factory[any]()
		args := factory.CreateDefaultArguments()
		murmurArgs, ok := args.(*Murmur3Hash128Arguments[any])
		require.True(t, ok)
		murmurArgs.Target = ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "Hello World", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createMurmur3Hash128Function[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "Murmur3Hash128Factory args must be of type *Murmur3Hash128Arguments[K]")
	})
}
