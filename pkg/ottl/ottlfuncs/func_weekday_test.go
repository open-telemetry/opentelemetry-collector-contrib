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

func Test_Weekday(t *testing.T) {
	tests := []struct {
		name     string
		time     ottl.TimeGetter[any]
		expected int64
	}{
		{
			name: "Mon",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2025, time.February, 24, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 1,
		},
		{
			name: "Tue",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2025, time.February, 25, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 2,
		},
		{
			name: "Wed",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2025, time.February, 26, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 3,
		},
		{
			name: "Thu",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2025, time.February, 27, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 4,
		},
		{
			name: "Fri",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2025, time.February, 28, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 5,
		},
		{
			name: "Sat",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2025, time.February, 22, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 6,
		},
		{
			name: "Sun",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2025, time.February, 23, 15, 4, 5, 0, time.UTC), nil
				},
			},
			expected: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Weekday(tt.time)
			require.NoError(t, err)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Weekday_Error(t *testing.T) {
	var getter ottl.TimeGetter[any] = &ottl.StandardTimeGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "not a time", nil
		},
	}
	exprFunc, err := Weekday(getter)
	require.NoError(t, err)
	result, err := exprFunc(t.Context(), nil)
	assert.Nil(t, result)
	assert.Error(t, err)
}

func Test_WeekdayFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewWeekdayFactory[any]()
		assert.Equal(t, "Weekday", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewWeekdayFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &WeekdayArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Time"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewWeekdayFactory[any]()
		args := factory.CreateDefaultArguments()
		timeArgs, ok := args.(*WeekdayArguments[any])
		require.True(t, ok)
		timeArgs.Time = &ottl.StandardTimeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return time.Now(), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createWeekdayFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "WeekdayFactory args must be of type *WeekdayArguments[K]")
	})
}
