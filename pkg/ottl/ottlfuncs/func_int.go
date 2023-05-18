// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IntArguments[K any] struct {
	Target ottl.Getter[K] `ottlarg:"0"`
}

func NewIntFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Int", &IntArguments[K]{}, createIntFunction[K])
}

func createIntFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IntArguments[K])

	if !ok {
		return nil, fmt.Errorf("IntFactory args must be of type *IntArguments[K]")
	}

	return intFunc(args.Target), nil
}

func intFunc[K any](target ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		value, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		switch value := value.(type) {
		case int64:
			return value, nil
		case string:
			intValue, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, nil
			}

			return intValue, nil
		case float64:
			return (int64)(value), nil
		case bool:
			if value {
				return int64(1), nil
			}
			return int64(0), nil
		default:
			return nil, nil
		}
	}
}
