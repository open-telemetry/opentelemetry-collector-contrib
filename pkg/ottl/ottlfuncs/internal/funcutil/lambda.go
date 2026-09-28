// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package funcutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs/internal/funcutil"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// EvaluateBiFunction executes a lambda with two positional arguments and returns its result.
// R is the type of lambda's evaluation result.
func EvaluateBiFunction[K, R any](
	tCtx K,
	lambda *ottl.LambdaActivation[K],
	v1 any,
	v2 any,
) (R, error) {
	return EvaluateFunction[K, R](tCtx, lambda, v1, v2)
}

// EvaluateBiPredicate evaluates a lambda bi-predicate with two arguments and returns the result.
func EvaluateBiPredicate[K any](
	tCtx K,
	lambda *ottl.LambdaActivation[K],
	v1 any,
	v2 any,
) (bool, error) {
	return EvaluateFunction[K, bool](tCtx, lambda, v1, v2)
}

// EvaluateFunction calls a lambda with the given arguments and returns the result.
// R is the type of lambda's evaluation result.
func EvaluateFunction[K, R any](tCtx K, lambda *ottl.LambdaActivation[K], args ...any) (R, error) {
	eval, err := lambda.Call(tCtx, args...)
	if err != nil {
		return *new(R), err
	}
	//nolint:gocritic // we want R to be returned as-is even if it's a pcommon.Value
	switch typedVal := eval.(type) {
	case R:
		return typedVal, nil
	case pcommon.Value:
		if res, ok := typedVal.AsRaw().(R); ok {
			return res, nil
		}
	}
	return *new(R), fmt.Errorf("lambda expression must return a value of type %T, got %T", *new(R), eval)
}
