// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_when(t *testing.T) {
	asLiteralGetter := func(v any) ottl.Getter[any] {
		getter, err := ottl.NewTestingLiteralGetter[any](true, ottl.StandardGetSetter[any]{
			Getter: func(context.Context, any) (any, error) {
				return v, nil
			},
		})
		require.NoError(t, err)
		return getter
	}

	tests := []struct {
		name       string
		condition  *ottl.LambdaExpression[any]
		trueValue  ottl.Getter[any]
		falseValue ottl.Getter[any]
		want       any
	}{
		{
			name: "condition true with literal values",
			condition: ottl.NewTestingLambdaExpression[any]([]string{}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return true, nil
			}),
			trueValue:  asLiteralGetter("true"),
			falseValue: asLiteralGetter("false"),
			want:       "true",
		},
		{
			name: "condition false with literal values",
			condition: ottl.NewTestingLambdaExpression[any]([]string{}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return false, nil
			}),
			trueValue:  asLiteralGetter("true"),
			falseValue: asLiteralGetter("false"),
			want:       "false",
		},
		{
			name: "condition true with dynamic values",
			condition: ottl.NewTestingLambdaExpression[any]([]string{}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return true, nil
			}),
			trueValue: &ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
				return int64(1), nil
			}},
			falseValue: &ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
				return int64(0), nil
			}},
			want: int64(1),
		},
		{
			name: "condition false with dynamic values",
			condition: ottl.NewTestingLambdaExpression[any]([]string{}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
				return false, nil
			}),
			trueValue: &ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
				return int64(1), nil
			}},
			falseValue: &ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
				return int64(0), nil
			}},
			want: int64(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := whenFunction(tt.condition, tt.trueValue, tt.falseValue)
			got, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_when_unused_branch(t *testing.T) {
	condition := ottl.NewTestingLambdaExpression[any]([]string{}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return true, nil
	})
	trueValue := &ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
		return "true", nil
	}}
	falseValue := &ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
		return nil, errors.New("should not be reached")
	}}

	exprFunc := whenFunction(condition, trueValue, falseValue)
	got, err := exprFunc(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, "true", got)
}

func Test_when_error(t *testing.T) {
	trueValue := &ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
		return "true", nil
	}}
	falseValue := &ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
		return "false", nil
	}}

	t.Run("non-boolean condition", func(t *testing.T) {
		condition := ottl.NewTestingLambdaExpression[any]([]string{}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
			return "not a bool", nil
		})

		exprFunc := whenFunction(condition, trueValue, falseValue)
		_, err := exprFunc(t.Context(), nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error while evaluating lambda function")
		assert.ErrorContains(t, err, "lambda expression must return a value of type bool")
	})

	t.Run("condition eval error", func(t *testing.T) {
		condition := ottl.NewTestingLambdaExpression[any]([]string{}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
			return nil, errors.New("eval failed")
		})

		exprFunc := whenFunction(condition, trueValue, falseValue)
		_, err := exprFunc(t.Context(), nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "error while evaluating lambda function")
		assert.ErrorContains(t, err, "eval failed")
	})
}

func Test_createWhenFunction(t *testing.T) {
	fCtx := ottl.FunctionContext{}
	condition := ottl.NewTestingLambdaExpression[any]([]string{}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return true, nil
	})
	trueValue := &ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
		return "true", nil
	}}
	falseValue := &ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
		return "false", nil
	}}

	t.Run("valid args", func(t *testing.T) {
		fn, err := createWhenFunction[any](fCtx, &WhenArguments[any]{
			Condition:  condition,
			TrueValue:  trueValue,
			FalseValue: falseValue,
		})
		require.NoError(t, err)
		require.NotNil(t, fn)
	})

	t.Run("invalid args type", func(t *testing.T) {
		_, err := createWhenFunction[any](fCtx, &struct{}{})
		assert.EqualError(t, err, "WhenFactory args must be of type *WhenArguments[K]")
	})
}
