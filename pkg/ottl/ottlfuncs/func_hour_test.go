// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Hour(t *testing.T) {
	tests := []struct {
		name     string
		time     ottl.TimeGetter[any]
		expected int64
	}{
		{
			name: "some time",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 15,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Hour(tt.time)
			require.NoError(t, err)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Hour_Error(t *testing.T) {
	var getter ottl.TimeGetter[any] = &ottl.StandardTimeGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "not a time", nil
		},
	}
	exprFunc, err := Hour(getter)
	require.NoError(t, err)
	result, err := exprFunc(t.Context(), nil)
	assert.Nil(t, result)
	assert.Error(t, err)
}

func Test_HourFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewHourFactory[any]()
		assert.Equal(t, "Hour", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewHourFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &HourArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewHourFactory[any]()
		args := factory.CreateDefaultArguments()
		hourArgs, ok := args.(*HourArguments[any])
		require.True(t, ok)
		hourArgs.Time = &ottl.StandardTimeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return time.Now(), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createHourFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "HourFactory args must be of type *HourArguments[K]")
	})
}
