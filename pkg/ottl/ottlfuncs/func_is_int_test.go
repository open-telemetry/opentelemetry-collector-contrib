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

func Test_IsInt(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "int",
			value:    int64(0),
			expected: true,
		},
		{
			name:     "ValueTypeInt",
			value:    pcommon.NewValueInt(0),
			expected: true,
		},
		{
			name:     "float64",
			value:    float64(2.7),
			expected: false,
		},
		{
			name:     "ValueTypeString",
			value:    pcommon.NewValueStr("a string"),
			expected: false,
		},
		{
			name:     "not Int",
			value:    "string",
			expected: false,
		},
		{
			name:     "string number",
			value:    "0",
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
			exprFunc := isInt[any](&ottl.StandardIntGetter[any]{
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
func Test_IsInt_Error(t *testing.T) {
	exprFunc := isInt[any](&ottl.StandardIntGetter[any]{
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

func Test_IsIntFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewIsIntFactory[any]()
		assert.Equal(t, "IsInt", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewIsIntFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &IsIntArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Target"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewIsIntFactory[any]()
		args := factory.CreateDefaultArguments()
		isIntArgs, ok := args.(*IsIntArguments[any])
		require.True(t, ok)
		isIntArgs.Target = &ottl.StandardIntGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return int64(1), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createIsIntFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "IsIntFactory args must be of type *IsIntArguments[K]")
	})
}
