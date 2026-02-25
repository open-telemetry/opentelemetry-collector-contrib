// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IsNilArguments[K any] struct {
	Target ottl.Getter[K]
}

func NewIsNilFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsNil", &IsNilArguments[K]{}, createIsNilFunction[K])
}

func createIsNilFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsNilArguments[K])

	if !ok {
		return nil, errors.New("IsNilFactory args must be of type *IsNilArguments[K]")
	}

	return isNil(args.Target), nil
}

//nolint:errorlint
func isNil[K any](target ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		
		if err != nil {
			if _, ok := err.(ottl.TypeError); ok {
				return true, nil
			}
			return false, err
		}

		return val == nil, nil
	}
}
