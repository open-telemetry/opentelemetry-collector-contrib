// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs/internal/funcutil"
)

type ReduceArguments[K any] struct {
	Source      ottl.Getter[K]
	Seed        ottl.Getter[K]
	Accumulator ottl.LambdaExpression[K]
}

func NewReduceFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Reduce", &ReduceArguments[K]{}, createReduceFunction[K])
}

func createReduceFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ReduceArguments[K])
	if !ok {
		return nil, errors.New("ReduceFactory args must be of type *ReduceArguments[K]")
	}
	return reduce(args.Source, args.Seed, &args.Accumulator), nil
}

func reduce[K any](source, seed ottl.Getter[K], accumulator *ottl.LambdaExpression[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		sourceVal, err := funcutil.GetSliceOrMapValue(ctx, tCtx, source)
		if err != nil {
			return nil, err
		}

		seedVal, err := seed.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		lb, err := accumulator.Activate(ctx, 3)
		if err != nil {
			return nil, err
		}
		defer lb.Close()

		switch typedVal := sourceVal.(type) {
		case pcommon.Map:
			return reduceMapValues(tCtx, typedVal, lb, seedVal)
		case pcommon.Slice:
			return reduceSliceValues(tCtx, typedVal, lb, seedVal)
		default:
			return nil, fmt.Errorf("unsupported type: %T", typedVal)
		}
	}
}

func reduceMapValues[K any](tCtx K, source pcommon.Map, lb *ottl.LambdaActivation[K], seedVal any) (any, error) {
	if source.Len() == 0 {
		return seedVal, nil
	}
	var err error
	acc := seedVal
	for k, v := range source.All() {
		if acc, err = accumulateValue(tCtx, lb, acc, k, v); err != nil {
			return nil, err
		}
	}
	return acc, nil
}

func reduceSliceValues[K any](tCtx K, source pcommon.Slice, lb *ottl.LambdaActivation[K], seedVal any) (any, error) {
	if source.Len() == 0 {
		return seedVal, nil
	}
	var err error
	acc := seedVal
	for i, v := range source.All() {
		if acc, err = accumulateValue(tCtx, lb, acc, int64(i), v); err != nil {
			return nil, err
		}
	}
	return acc, nil
}

func accumulateValue[K any](tCtx K, lb *ottl.LambdaActivation[K], acc, v1, v2 any) (any, error) {
	err := lb.SetArg(0, acc)
	if err != nil {
		return nil, err
	}
	err = lb.SetArg(1, ottlcommon.NormalizeValue(v1))
	if err != nil {
		return nil, err
	}
	err = lb.SetArg(2, ottlcommon.NormalizeValue(v2))
	if err != nil {
		return nil, err
	}
	return lb.Eval(tCtx)
}
