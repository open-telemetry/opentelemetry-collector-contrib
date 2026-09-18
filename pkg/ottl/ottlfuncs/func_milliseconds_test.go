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

func Test_Milliseconds(t *testing.T) {
	tests := []struct {
		name     string
		duration ottl.DurationGetter[any]
		expected int64
	}{
		{
			name: "100 Milliseconds",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.ParseDuration("100ms")
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
			expected: 360000000,
		},
		{
			name: "47 mins",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.ParseDuration("47m")
				},
			},
			expected: 2820000,
		},
		{
			name: "1 hour 40 mins 3 seconds 30 milliseconds",
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.ParseDuration("1h40m3s30ms")
				},
			},
			expected: 6003030,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := milliseconds(tt.duration)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_MillisecondsFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewMillisecondsFactory[any]()
		assert.Equal(t, "Milliseconds", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewMillisecondsFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &millisecondsArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Duration"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewMillisecondsFactory[any]()
		args := factory.CreateDefaultArguments()
		millisecondsArgs, ok := args.(*millisecondsArguments[any])
		require.True(t, ok)
		millisecondsArgs.Duration = ottl.StandardDurationGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return time.Duration(100), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createMillisecondsFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "MillisecondsFactory args must be of type *millisecondsArguments[K]")
	})
}

func BenchmarkMilliseconds(b *testing.B) {
	exprFunc := milliseconds[any](&ottl.StandardDurationGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return time.ParseDuration("1h40m3s30ms")
		},
	})
	ctx := b.Context()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := exprFunc(ctx, nil); err != nil {
			b.Fatal(err)
		}
	}
}
