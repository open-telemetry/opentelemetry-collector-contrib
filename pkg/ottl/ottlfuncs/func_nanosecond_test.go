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

func Test_Nanosecond(t *testing.T) {
	tests := []struct {
		name     string
		time     ottl.TimeGetter[any]
		expected int64
	}{
		{
			name: "some time",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return time.Date(2006, time.January, 2, 15, 4, 5, 197382465, time.UTC), nil
				},
			},
			expected: 197382465,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Nanosecond(tt.time)
			require.NoError(t, err)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Nanosecond_Error(t *testing.T) {
	var getter ottl.TimeGetter[any] = &ottl.StandardTimeGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "not a time", nil
		},
	}
	exprFunc, err := Nanosecond(getter)
	require.NoError(t, err)
	result, err := exprFunc(t.Context(), nil)
	assert.Nil(t, result)
	assert.Error(t, err)
}

func Test_NanosecondFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewNanosecondFactory[any]()
		assert.Equal(t, "Nanosecond", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewNanosecondFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &NanosecondArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Time"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewNanosecondFactory[any]()
		args := factory.CreateDefaultArguments()
		nanoArgs, ok := args.(*NanosecondArguments[any])
		require.True(t, ok)
		nanoArgs.Time = ottl.StandardTimeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return time.Now(), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createNanosecondFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "NanosecondFactory args must be of type *NanosecondArguments[K]")
	})
}
