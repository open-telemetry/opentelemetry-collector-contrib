// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package funcutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs/internal/funcutil"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"
)

// EvaluateBiFunction executes a lambda with two positional arguments and returns its result.
// R is the type of lambda's evaluation result.
func EvaluateBiFunction[K, R any](
	tCtx K,
	lambda *ottl.LambdaActivation[K],
	v1 any,
	v2 any,
) (R, error) {
	err := lambda.SetArg(0, ottlcommon.NormalizeValue(v1))
	if err != nil {
		return *new(R), err
	}
	err = lambda.SetArg(1, ottlcommon.NormalizeValue(v2))
	if err != nil {
		return *new(R), err
	}
	return EvaluateLambdaActivation[K, R](tCtx, lambda)
}

// EvaluateBiPredicate evaluates a lambda bi-predicate with two arguments and returns the result.
func EvaluateBiPredicate[K any](
	tCtx K,
	lambda *ottl.LambdaActivation[K],
	v1 any,
	v2 any,
) (bool, error) {
	return EvaluateBiFunction[K, bool](tCtx, lambda, v1, v2)
}

// EvaluateLambdaActivation evaluates a lambda activation with the given context and returns the result.
// R is the type of lambda's evaluation result.
// All lambda arguments must be set before calling this function.
func EvaluateLambdaActivation[K, R any](
	tCtx K,
	lambda *ottl.LambdaActivation[K],
) (R, error) {
	eval, err := lambda.Eval(tCtx)
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
