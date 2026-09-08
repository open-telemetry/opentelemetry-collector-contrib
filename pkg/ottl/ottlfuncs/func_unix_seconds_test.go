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

func Test_TimeUnixSeconds(t *testing.T) {
	tests := []struct {
		name     string
		time     ottl.TimeGetter[any]
		expected time.Time
	}{
		{
			name: "January 1, 2023",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2023, 1, 1, 0, 0, 0, 0, time.Local), nil
				},
			},
			expected: time.Date(2023, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			name: "March 31, 2000, 4pm",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2000, 3, 31, 16, 0, 0, 0, time.Local), nil
				},
			},
			expected: time.Date(2000, 3, 31, 16, 0, 0, 0, time.Local),
		},
		{
			name: "December 12, 1980, 4:35:01am",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(1980, 12, 12, 4, 35, 1, 0, time.Local), nil
				},
			},
			expected: time.Date(1980, 12, 12, 4, 35, 1, 0, time.Local),
		},
		{
			name: "October 4, 2020, 5:05 5 microseconds 5 nanosecs",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2020, 10, 4, 5, 5, 5, 5, time.Local), nil
				},
			},
			expected: time.Date(2020, 10, 4, 5, 5, 5, 5, time.Local),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := UnixSeconds(tt.time)
			require.NoError(t, err)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			want := tt.expected.Unix()
			assert.Equal(t, want, result)
		})
	}
}

func Test_UnixSecondsFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewUnixSecondsFactory[any]()
		assert.Equal(t, "UnixSeconds", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewUnixSecondsFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &UnixSecondsArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Time"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewUnixSecondsFactory[any]()
		args := factory.CreateDefaultArguments()
		timeArgs, ok := args.(*UnixSecondsArguments[any])
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
		_, err := createUnixSecondsFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "UnixSecondsFactory args must be of type *UnixSecondsArguments[K]")
	})
}
