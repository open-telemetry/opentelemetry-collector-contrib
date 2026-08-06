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

func Test_Unix(t *testing.T) {
	tests := []struct {
		name        string
		seconds     ottl.IntGetter[any]
		nanoseconds ottl.Optional[ottl.IntGetter[any]]
		expected    int64
	}{
		{
			name: "January 1, 2023",
			seconds: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(1672527600), nil
				},
			},
			nanoseconds: ottl.Optional[ottl.IntGetter[any]]{},
			expected:    int64(1672527600),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Unix(tt.seconds, tt.nanoseconds)
			require.NoError(t, err)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			want := time.Unix(tt.expected, 0)
			assert.Equal(t, want, result)
		})
	}
}

func Test_UnixFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewUnixFactory[any]()
		assert.Equal(t, "Unix", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewUnixFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &UnixArguments[any]{}, args)
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewUnixFactory[any]()
		args := factory.CreateDefaultArguments()
		unixArgs, ok := args.(*UnixArguments[any])
		require.True(t, ok)
		unixArgs.Seconds = &ottl.StandardIntGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return int64(1672531200), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createUnixFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "UnixFactory args must be of type *UnixArguments[K]")
	})
}
