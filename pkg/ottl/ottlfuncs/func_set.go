// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type setArguments[K any] struct {
	Target ottl.Setter[K]
	Value  ottl.Getter[K]
}

func NewSetFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("set", &setArguments[K]{}, createSetFunction[K])
}

func createSetFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*setArguments[K])

	if !ok {
		return nil, errors.New("SetFactory args must be of type *setArguments[K]")
	}

	return set(args.Target, args.Value), nil
}

func set[K any](target ottl.Setter[K], value ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := value.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		err = target.Set(ctx, tCtx, val)
		if err != nil {
			return nil, err
		}

		return nil, nil
	}
}
