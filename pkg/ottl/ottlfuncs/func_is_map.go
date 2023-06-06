// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IsMapArguments[K any] struct {
	Target ottl.PMapGetter[K] `ottlarg:"0"`
}

func NewIsMapFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("IsMap", &IsMapArguments[K]{}, createIsMapFunction[K])
}

func createIsMapFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IsMapArguments[K])

	if !ok {
		return nil, fmt.Errorf("IsMapFactory args must be of type *IsMapArguments[K]")
	}

	return isMap(args.Target), nil
}

func isMap[K any](target ottl.PMapGetter[K]) ottl.ExprFunc[K] {
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
