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

func Test_Microseconds(t *testing.T) {
	tests := []struct {
		name     string
		duration ottl.DurationGetter[any]
		expected int64
	}{
		{
			name: "100 microseconds",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.ParseDuration("100us")
				},
			},
			expected: 100,
		},
		{
			name: "1000 hour",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.ParseDuration("100h")
				},
			},
			expected: 360000000000,
		},
		{
			name: "50 mins",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.ParseDuration("50m")
				},
			},
			expected: 3000000000,
		},
		{
			name: "1 hour 40 mins 3 seconds 30 milliseconds 100 microseconds",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.ParseDuration("1h40m3s30ms100us")
				},
			},
			expected: 6003030100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Microseconds(tt.duration)
			require.NoError(t, err)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_MicrosecondsFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewMicrosecondsFactory[any]()
		assert.Equal(t, "Microseconds", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewMicrosecondsFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &MicrosecondsArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Duration"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewMicrosecondsFactory[any]()
		args := factory.CreateDefaultArguments()
		microsecondsArgs, ok := args.(*MicrosecondsArguments[any])
		require.True(t, ok)
		microsecondsArgs.Duration = ottl.StandardDurationGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return time.Duration(100), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createMicrosecondsFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "MicrosecondsFactory args must be of type *MicrosecondsArguments[K]")
	})
}
