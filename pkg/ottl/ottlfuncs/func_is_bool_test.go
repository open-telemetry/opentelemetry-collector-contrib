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

func Test_IsBool(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "bool",
			value:    true,
			expected: true,
		},
		{
			name:     "ValueTypeBool",
			value:    pcommon.NewValueBool(false),
			expected: true,
		},
		{
			name:     "not bool",
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
			exprFunc := isBool[any](&ottl.StandardBoolGetter[any]{
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
func Test_IsBool_Error(t *testing.T) {
	exprFunc := isBool[any](&ottl.StandardBoolGetter[any]{
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

func Test_IsBoolFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewIsBoolFactory[any]()
		assert.Equal(t, "IsBool", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewIsBoolFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &IsBoolArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewIsBoolFactory[any]()
		args := factory.CreateDefaultArguments()
		isBoolArgs, ok := args.(*IsBoolArguments[any])
		require.True(t, ok)
		isBoolArgs.Target = &ottl.StandardBoolGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return true, nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createIsBoolFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "IsBoolFactory args must be of type *IsBoolArguments[K]")
	})
}
