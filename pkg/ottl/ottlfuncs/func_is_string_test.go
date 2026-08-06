// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_IsString(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "string",
			value:    "a string",
			expected: true,
		},
		{
			name:     "ValueTypeString",
			value:    pcommon.NewValueStr("a string"),
			expected: true,
		},
		{
			name:     "not String",
			value:    1,
			expected: false,
		},
		{
			name:     "ValueTypeSlice",
			value:    pcommon.NewValueSlice(),
			expected: false,
		},
		{
			name:     "nil",
			value:    nil,
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := isString[any](&ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return tt.value, nil
				},
			})
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

//nolint:errorlint
func Test_IsString_Error(t *testing.T) {
	exprFunc := isString[any](&ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, ottl.TypeError("")
		},
	})
	result, err := exprFunc(t.Context(), nil)
	assert.Equal(t, false, result)
	assert.Error(t, err)
	_, ok := err.(ottl.TypeError)
	assert.False(t, ok)
}

func Test_IsStringFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewIsStringFactory[any]()
		assert.Equal(t, "IsString", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewIsStringFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &IsStringArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewIsStringFactory[any]()
		args := factory.CreateDefaultArguments()
		isStringArgs, ok := args.(*IsStringArguments[any])
		require.True(t, ok)
		isStringArgs.Target = &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return "hello", nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createIsStringFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "IsStringFactory args must be of type *IsStringArguments[K]")
	})
}
