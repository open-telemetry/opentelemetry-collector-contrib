// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/xpdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs/internal/funcutil"
)

type MapEachArguments[K any] struct {
	Source ottl.Getter[K]
	Mapper ottl.LambdaExpression[K]
}

func NewMapEachFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("MapEach", &MapEachArguments[K]{}, createMapEachFunction[K])
}

func createMapEachFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*MapEachArguments[K])
	if !ok {
		return nil, errors.New("MapEachFactory args must be of type *MapEachArguments[K]")
	}
	return mapEach(args.Source, &args.Mapper), nil
}

func mapEach[K any](source ottl.Getter[K], mapper *ottl.LambdaExpression[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		sourceVal, err := funcutil.GetSliceOrMapValue(ctx, tCtx, source)
		if err != nil {
			return nil, err
		}

		lb, err := mapper.Activate(ctx, 2)
		if err != nil {
			return nil, err
		}
		defer lb.Close()

		switch typedVal := sourceVal.(type) {
		case pcommon.Map:
			return mapMapValues(tCtx, typedVal, lb)
		case pcommon.Slice:
			return mapSliceValues(tCtx, typedVal, lb)
		default:
			return nil, fmt.Errorf("unsupported type: %T", typedVal)
		}
	}
}

func mapMapValues[K any](tCtx K, source pcommon.Map, lb *ottl.LambdaActivation[K]) (pcommon.Map, error) {
	var builder xpdata.MapBuilder
	builder.EnsureCapacity(source.Len())
	for k, v := range source.All() {
		newVal, err := funcutil.EvaluateBiFunction[K, any](tCtx, lb, k, v)
		if err != nil {
			return pcommon.Map{}, err
		}
		err = builder.AppendEmpty(k).FromRaw(newVal)
		if err != nil {
			return pcommon.Map{}, err
		}
	}
	res := pcommon.NewMap()
	builder.UnsafeIntoMap(res)
	return res, nil
}

func mapSliceValues[K any](tCtx K, source pcommon.Slice, lb *ottl.LambdaActivation[K]) (pcommon.Slice, error) {
	res := pcommon.NewSlice()
	res.EnsureCapacity(source.Len())
	for i, v := range source.All() {
		val, err := funcutil.EvaluateBiFunction[K, any](tCtx, lb, int64(i), v)
		if err != nil {
			return pcommon.Slice{}, err
		}
		err = res.AppendEmpty().FromRaw(val)
		if err != nil {
			return pcommon.Slice{}, err
		}
	}
	return res, nil
}
