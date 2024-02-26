// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"math"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IsNaNArguments[K any] struct {
	Target ottl.FloatGetter[K]
}

func NewIsNaNFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsNaN", &IsNaNArguments[K]{}, createIsNaNFunction[K])
}

func createIsNaNFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsNaNArguments[K])

	if !ok {
		return nil, fmt.Errorf("IsNaNFactory args must be of type *IsNaNArguments[K]")
	}

	return isNaN(args.Target), nil
}

// nolint:errorlint
func isNaN[K any](target ottl.FloatGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		// Use type assertion because we don't want to check wrapped errors
		switch err.(type) {
		case ottl.TypeError:
			return false, nil
		case nil:
			return math.IsNaN(val), nil
		default:
			return false, err
		}
	}
}
