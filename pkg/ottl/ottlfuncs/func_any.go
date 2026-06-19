// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs/internal/funcutil"
)

type AnyArguments[K any] struct {
	Source    ottl.Getter[K]
	Predicate ottl.LambdaExpression[K]
}

func NewAnyFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Any", &AnyArguments[K]{}, createAnyFunction[K])
}

func createAnyFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*AnyArguments[K])
	if !ok {
		return nil, errors.New("AnyFactory args must be of type *AnyArguments[K]")
	}
	return anyMatch(args.Source, &args.Predicate), nil
}

func anyMatch[K any](source ottl.Getter[K], predicate *ottl.LambdaExpression[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		sourceVal, err := funcutil.GetSliceOrMapValue(ctx, tCtx, source)
		if err != nil {
			return nil, err
		}

		lb, err := predicate.Activate(ctx, 2)
		if err != nil {
			return nil, err
		}
		defer lb.Close()

		switch typedVal := sourceVal.(type) {
		case pcommon.Map:
			return anyMapValueMatch(tCtx, typedVal, lb)
		case pcommon.Slice:
			return anySliceValueMatch(tCtx, typedVal, lb)
		default:
			return nil, fmt.Errorf("unsupported type: %T", typedVal)
		}
	}
}

func anySliceValueMatch[K any](tCtx K, source pcommon.Slice, lambda *ottl.LambdaActivation[K]) (bool, error) {
	for i, v := range source.All() {
		match, err := funcutil.EvaluateBiPredicate(tCtx, lambda, int64(i), v)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

func anyMapValueMatch[K any](tCtx K, source pcommon.Map, lambda *ottl.LambdaActivation[K]) (bool, error) {
	for k, v := range source.All() {
		match, err := funcutil.EvaluateBiPredicate(tCtx, lambda, k, v)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}
