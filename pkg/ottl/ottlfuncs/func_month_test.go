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

func Test_Month(t *testing.T) {
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
			expected: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := month(tt.time)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Month_Error(t *testing.T) {
	var getter ottl.TimeGetter[any] = &ottl.StandardTimeGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "not a time", nil
		},
	}
	exprFunc := month(getter)
	result, err := exprFunc(t.Context(), nil)
	assert.Nil(t, result)
	assert.Error(t, err)
}

func Test_MonthFactory(t *testing.T) {
	t.Run("factory creation", func(t *testing.T) {
		factory := NewMonthFactory[any]()
		assert.Equal(t, "Month", factory.Name())
	})

	t.Run("default arguments", func(t *testing.T) {
		factory := NewMonthFactory[any]()
		args := factory.CreateDefaultArguments()

		assert.IsType(t, &monthArguments[any]{}, args)
		assertArgumentFieldNames(t, args, []string{"Time"})
	})

	t.Run("function creation", func(t *testing.T) {
		factory := NewMonthFactory[any]()
		args := factory.CreateDefaultArguments()
		monthArgs, ok := args.(*monthArguments[any])
		require.True(t, ok)
		monthArgs.Time = ottl.StandardTimeGetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return time.Now(), nil
			},
		}

		fn, err := factory.CreateFunction(ottl.FunctionContext{}, args)
		require.NoError(t, err)
		assert.NotNil(t, fn)
	})

	t.Run("invalid arguments type", func(t *testing.T) {
		_, err := createMonthFunction[any](ottl.FunctionContext{}, "invalid args")
		assert.ErrorContains(t, err, "MonthFactory args must be of type *monthArguments[K]")
	})
}

func BenchmarkMonth(b *testing.B) {
	exprFunc := month[any](&ottl.StandardTimeGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC), nil
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
