// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IsStringArguments[K any] struct {
	Target ottl.StringGetter[K] `ottlarg:"0"`
}

func NewIsStringFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsString", &IsStringArguments[K]{}, createIsStringFunction[K])
}

func createIsStringFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsStringArguments[K])

	if !ok {
		return nil, fmt.Errorf("IsStringFactory args must be of type *IsStringArguments[K]")
	}

	return isStringFunc(args.Target), nil
}

func isStringFunc[K any](target ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		_, err := target.Get(ctx, tCtx)
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
