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

func Test_IsMap(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{
			name:     "map",
			value:    make(map[string]any, 0),
			expected: true,
		},
		{
			name:     "ValueTypeMap",
			value:    pcommon.NewValueMap(),
			expected: true,
		},
		{
			name:     "not map",
			value:    "not a map",
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
			exprFunc := isMap[any](&ottl.StandardPMapGetter[any]{
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
func Test_IsMap_Error(t *testing.T) {
	exprFunc := isMap[any](&ottl.StandardPMapGetter[any]{
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

func Test_IsMapFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewIsMapFactory[any]()
		assert.Equal(t, "IsMap", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewIsMapFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &IsMapArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewIsMapFactory[any]()
		args := factory.CreateDefaultArguments()
		isMapArgs, ok := args.(*IsMapArguments[any])
		require.True(t, ok)
		isMapArgs.Target = &ottl.StandardPMapGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return pcommon.NewMap(), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createIsMapFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "IsMapFactory args must be of type *IsMapArguments[K]")
	})
}
