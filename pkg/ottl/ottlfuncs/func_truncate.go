// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type TruncateArguments[K any] struct {
	Target ottl.GetSetter[K] `ottlarg:"0"`
	Limit  int64             `ottlarg:"1"`
}

func NewTruncateFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("truncate", &TruncateArguments[K]{}, createTruncateFunction[K])
}

func createTruncateFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*TruncateArguments[K])

	if !ok {
		return nil, fmt.Errorf("TruncateFactory args must be of type *TruncateArguments[K]")
	}

	return truncate(args.Target, args.Limit)
}

func truncate[K any](target ottl.GetSetter[K], limit int64) (ottl.ExprFunc[K], error) {
	if limit < 0 {
		return nil, fmt.Errorf("invalid limit for truncate function, %d cannot be negative", limit)
	}
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		if limit < 0 {
			return nil, nil
		}

		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if val == nil {
			return nil, nil
		}
		if valStr, ok := val.(string); ok {
			if int64(len(valStr)) > limit {
				err = target.Set(ctx, tCtx, valStr[:limit])
				if err != nil {
					return nil, err
				}
			}
		}
		return nil, nil
	}, nil
}
