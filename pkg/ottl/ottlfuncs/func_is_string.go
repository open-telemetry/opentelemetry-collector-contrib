// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
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
		if err == nil {
			return true, nil
		}
		var typeError *ottl.TypeError
		if errors.As(err, &typeError) {
			return false, nil
		}
		return false, err
	}
}
