// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ToValuesArguments[K any] struct {
	Map   ottl.Optional[ottl.PMapGetter[K]]
	Maps  ottl.Optional[[]ottl.PMapGetter[K]]
	Depth ottl.Optional[int64]
}

func NewToValuesFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ToValues", &ToValuesArguments[K]{}, createToValuesFunction[K])
}

func createToValuesFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ToValuesArguments[K])
	if !ok {
		return nil, errors.New("ToValuesFactory args must be of type *ToValuesArguments[K]")
	}

	return toValues(args.Map, args.Maps, args.Depth)
}

func toValues[K any](aMap ottl.Optional[ottl.PMapGetter[K]], maps ottl.Optional[[]ottl.PMapGetter[K]], depth ottl.Optional[int64]) (ottl.ExprFunc[K], error) {
	if aMap.IsEmpty() && maps.IsEmpty() {
		return nil, errors.New("one of the two optional arguments ('map' or 'maps') must be provided")
	}
	if !aMap.IsEmpty() && !maps.IsEmpty() {
		return nil, errors.New("only one of the two optional arguments ('map' or 'maps') should be provided")
	}
	d := int64(math.MaxInt64)
	if !depth.IsEmpty() {
		d = depth.Get()
		if d < 0 {
			return nil, errors.New("depth must be an integer greater than or equal to 0")
		}
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		// init res with target values
		var res []any
		if !aMap.IsEmpty() {
			mGetter := aMap.Get()
			m, err := mGetter.Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
			res = values([]pcommon.Map{m}, d)
		}

		if !maps.IsEmpty() {
			arrayOfMapGetters := maps.Get()
			arrayOfMaps := []pcommon.Map{}
			for _, mapGetter := range arrayOfMapGetters {
				m, err := mapGetter.Get(ctx, tCtx)
				if err != nil {
					return nil, err
				}
				arrayOfMaps = append(arrayOfMaps, m)
			}

			res = values(arrayOfMaps, d)
		}

		resSlice := pcommon.NewSlice()
		if err := resSlice.FromRaw(res); err != nil {
			return nil, err
		}
		return resSlice, nil
	}, nil
}

func values(maps []pcommon.Map, depth int64) []any {
	rv := []any{}
	for _, m := range maps {
		for _, v := range m.All() {
			if v.Type() == pcommon.ValueTypeMap && depth > 0 {
				// If the value is a pcommon.Map and depth is greater than 0, we need to go deeper.
				rv = append(rv, values([]pcommon.Map{v.Map()}, depth-1)...)
			} else {
				rv = append(rv, v.AsRaw())
			}
		}
	}
	return rv
}
