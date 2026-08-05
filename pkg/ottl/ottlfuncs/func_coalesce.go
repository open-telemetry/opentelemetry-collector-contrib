// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type CoalesceArguments[K any] struct {
	Values ottl.SliceGetter[K]
}

func NewCoalesceFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Coalesce", &CoalesceArguments[K]{}, createCoalesceFunction[K])
}

func createCoalesceFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*CoalesceArguments[K])
	if !ok {
		return nil, errors.New("CoalesceFactory args must be of type *CoalesceArguments[K]")
	}

	return coalesce(args.Values), nil
}

var errCoalesceEmpty = errors.New("Coalesce requires at least one value")

func coalesce[K any](values ottl.SliceGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		// When backed by a literal list, evaluate the element Getters one at a time so evaluation
		// stops at the first non-nil value.
		if getters := values.Getters(); getters != nil {
			if len(getters) == 0 {
				return nil, errCoalesceEmpty
			}
			for _, g := range getters {
				v, err := g.Get(ctx, tCtx)
				if err != nil {
					return nil, err
				}
				if v != nil {
					return v, nil
				}
			}
			return nil, nil
		}

		// Otherwise the value is a single list-valued expression that is evaluated all at once.
		vals, err := values.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		// A non-nil but empty list (such as the literal `[]`) has no values to coalesce. A nil
		// value falls through to return nil, matching an all-nil list.
		if vals != nil && len(vals) == 0 {
			return nil, errCoalesceEmpty
		}
		for _, v := range vals {
			if v != nil {
				return v, nil
			}
		}
		return nil, nil
	}
}
