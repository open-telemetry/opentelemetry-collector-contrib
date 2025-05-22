// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"reflect"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ContainsArguments[K any] struct {
	Target ottl.PSliceGetter[K]
	Item   ottl.Getter[K]
}

func NewSliceContainsFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("SliceContains", &ContainsArguments[K]{}, createSliceContainsFunction[K])
}

func createSliceContainsFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ContainsArguments[K])

	if !ok {
		return nil, errors.New("SliceContainsFactory args must be of type *ContainsArguments[K]")
	}

	return sliceContains(args.Target, args.Item), nil
}

func sliceContains[K any](target ottl.PSliceGetter[K], itemGetter ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		slice, sliceErr := target.Get(ctx, tCtx)
		if sliceErr != nil {
			return nil, sliceErr
		}
		item, itemErr := itemGetter.Get(ctx, tCtx)
		if itemErr != nil {
			return nil, itemErr
		}

		for i := 0; i < slice.Len(); i++ {
			val := slice.At(i).AsRaw()
			if reflect.DeepEqual(val, item) {
				return true, nil
			}
		}
		return false, nil
	}
}
