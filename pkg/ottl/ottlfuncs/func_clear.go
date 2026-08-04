// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ClearArguments[K any] struct {
	Target ottl.Setter[K]
}

func NewClearFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("clear", &ClearArguments[K]{}, createClearFunction[K])
}

func createClearFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ClearArguments[K])

	if !ok {
		return nil, errors.New("ClearFactory args must be of type *ClearArguments[K]")
	}

	return clearFunc(args.Target), nil
}

func clearFunc[K any](target ottl.Setter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		// Pass nil to the target setter. The underlying path setter will translate
		// this nil into the appropriate zero-value or empty state for its specific type.
		err := target.Set(ctx, tCtx, nil)
		if err != nil {
			return nil, err
		}

		return nil, nil
	}
}
