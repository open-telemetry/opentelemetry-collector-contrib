// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/xpdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs/internal/funcutil"
)

type MapKeysArguments[K any] struct {
	Source    ottl.PMapGetter[K]
	KeyMapper ottl.LambdaExpression[K]
}

func NewMapKeysFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("MapKeys", &MapKeysArguments[K]{}, createMapKeysFunction[K])
}

func createMapKeysFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*MapKeysArguments[K])
	if !ok {
		return nil, errors.New("MapKeysFactory args must be of type *MapKeysArguments[K]")
	}
	return mapKeys(args.Source, &args.KeyMapper), nil
}

func mapKeys[K any](source ottl.PMapGetter[K], keyMapper *ottl.LambdaExpression[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		sourceVal, err := source.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		lb, err := keyMapper.Activate(ctx, 2)
		if err != nil {
			return nil, err
		}
		defer lb.Close()

		var builder xpdata.MapBuilder
		builder.EnsureCapacity(sourceVal.Len())
		for k, v := range sourceVal.All() {
			newKey, err := funcutil.EvaluateBiFunction[K, string](tCtx, lb, k, v)
			if err != nil {
				return pcommon.Map{}, err
			}
			v.CopyTo(builder.AppendEmpty(newKey))
		}

		res := pcommon.NewMap()
		builder.UnsafeIntoMap(res)
		return res, nil
	}
}
