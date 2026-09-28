// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package funcutil

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func activateTestLambda(t *testing.T, expr *ottl.LambdaExpression[any], arity int) *ottl.LambdaActivation[any] {
	t.Helper()
	require.NoError(t, expr.ValidateArity(arity))
	lb := expr.Activate(t.Context())
	t.Cleanup(lb.Close)
	return lb
}

func TestEvaluateBiPredicate(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
		k := resolveBinding("k")
		v := resolveBinding("v")
		return k.(string) == "match" && v.(int64) > 0, nil
	})
	lb := activateTestLambda(t, expr, 2)

	got, err := EvaluateBiPredicate[any](nil, lb, "match", int64(1))
	require.NoError(t, err)
	assert.True(t, got)

	got, err = EvaluateBiPredicate[any](nil, lb, "other", int64(1))
	require.NoError(t, err)
	assert.False(t, got)
}

func TestEvaluateBiPredicate_normalizesPcommonValue(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
		k := resolveBinding("k")
		v := resolveBinding("v")
		return k.(string) == "key" && v.(int64) == 7, nil
	})
	lb := activateTestLambda(t, expr, 2)

	got, err := EvaluateBiPredicate[any](nil, lb, pcommon.NewValueStr("key"), pcommon.NewValueInt(7))
	require.NoError(t, err)
	assert.True(t, got)
}

func TestEvaluateBiFunction(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
		k := resolveBinding("k")
		v := resolveBinding("v")
		return k.(string) + v.(string), nil
	})
	lb := activateTestLambda(t, expr, 2)

	got, err := EvaluateBiFunction[any, string](nil, lb, "hello", " world")
	require.NoError(t, err)
	assert.Equal(t, "hello world", got)
}

func TestEvaluateBiFunction_unwrapsPcommonValueResult(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"_", "_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return pcommon.NewValueStr("from-value"), nil
	})
	lb := activateTestLambda(t, expr, 2)

	got, err := EvaluateBiFunction[any, string](nil, lb, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "from-value", got)
}

func TestEvaluateFunction_directType(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return int64(42), nil
	})
	lb := activateTestLambda(t, expr, 1)

	got, err := EvaluateFunction[any, int64](nil, lb, int64(0))
	require.NoError(t, err)
	assert.Equal(t, int64(42), got)
}

func TestEvaluateFunction_typeError(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return 123, nil
	})
	lb := activateTestLambda(t, expr, 1)

	_, err := EvaluateFunction[any, string](nil, lb, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "lambda expression must return a value of type string")
}

func TestEvaluateFunction_wrongArgumentCount(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"a", "b"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return nil, nil
	})
	lb := activateTestLambda(t, expr, 2)

	_, err := EvaluateFunction[any, any](nil, lb, "one", "two", "three")
	require.EqualError(t, err, "lambda should be defined with exactly 3 formal(s), but has 2")
}

func TestEvaluateFunction(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
		k := resolveBinding("k")
		v := resolveBinding("v")
		return k.(int64) + v.(int64), nil
	})
	lb := activateTestLambda(t, expr, 2)

	got, err := EvaluateFunction[any, int64](nil, lb, int64(1), int64(2))
	require.NoError(t, err)
	assert.Equal(t, int64(3), got)
}

func TestEvaluateFunction_withUnboundArgument(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"k", "_", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
		acc := resolveBinding("k")
		v := resolveBinding("v")
		return acc.(string) + v.(string), nil
	})
	lb := activateTestLambda(t, expr, 3)

	got, err := EvaluateFunction[any, string](nil, lb, "seed", "ignored", "suffix")
	require.NoError(t, err)
	assert.Equal(t, "seedsuffix", got)
}

func TestEvaluateFunction_normalizesArguments(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"k", "v"}, func(_ context.Context, _ any, resolveBinding func(string) any) (any, error) {
		k := resolveBinding("k")
		v := resolveBinding("v")
		return k.(int64) + v.(int64), nil
	})
	lb := activateTestLambda(t, expr, 2)

	got, err := EvaluateFunction[any, int64](nil, lb, pcommon.NewValueInt(1), pcommon.NewValueInt(2))
	require.NoError(t, err)
	assert.Equal(t, int64(3), got)
}

func TestEvaluateFunction_evalError(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"a"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return nil, errors.New("eval failed")
	})
	lb := activateTestLambda(t, expr, 1)

	_, err := EvaluateFunction[any, bool](nil, lb, "x")
	require.Error(t, err)
	assert.ErrorContains(t, err, "eval failed")
}
