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
	lb, err := expr.Activate(t.Context(), arity)
	require.NoError(t, err)
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

func TestEvaluateLambdaActivation_directType(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"v"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return int64(42), nil
	})
	lb := activateTestLambda(t, expr, 1)
	require.NoError(t, lb.SetArg(0, int64(0)))

	got, err := EvaluateLambdaActivation[any, int64](nil, lb)
	require.NoError(t, err)
	assert.Equal(t, int64(42), got)
}

func TestEvaluateLambdaActivation_evalError(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"a"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return nil, errors.New("eval failed")
	})
	lb := activateTestLambda(t, expr, 1)
	require.NoError(t, lb.SetArg(0, "x"))

	_, err := EvaluateLambdaActivation[any, bool](nil, lb)
	require.Error(t, err)
	assert.ErrorContains(t, err, "eval failed")
}

func TestEvaluateLambdaActivation_typeError(t *testing.T) {
	expr := ottl.NewTestingLambdaExpression[any]([]string{"_"}, func(_ context.Context, _ any, _ func(string) any) (any, error) {
		return 123, nil
	})
	lb := activateTestLambda(t, expr, 1)
	require.NoError(t, lb.SetArg(0, nil))

	_, err := EvaluateLambdaActivation[any, string](nil, lb)
	require.Error(t, err)
	assert.ErrorContains(t, err, "lambda expression must return a value of type string")
}
