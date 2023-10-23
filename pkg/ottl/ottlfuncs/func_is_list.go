// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IsListArguments[K any] struct {
	Target ottl.PSliceGetter[K]
}

func NewIsListFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsList", &IsListArguments[K]{}, createIsListFunction[K])
}

func createIsListFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsListArguments[K])

	if !ok {
		return nil, fmt.Errorf("IsListFactory args must be of type *IsListArguments[K]")
	}

	return isList(args.Target), nil
}

// nolint:errorlint
func isList[K any](target ottl.PSliceGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		_, err := target.Get(ctx, tCtx)
		// Use type assertion because we don't want to check wrapped errors
		switch err.(type) {
		case ottl.TypeError:
			return false, nil
		case nil:
			return true, nil
		default:
			return false, err
		}
	}
}
