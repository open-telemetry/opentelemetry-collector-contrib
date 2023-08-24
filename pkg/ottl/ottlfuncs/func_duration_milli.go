// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type MilliArguments[K any] struct {
	Duration ottl.DurationGetter[K] `ottlarg:"0"`
}

func NewMilliFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Milli", &MilliArguments[K]{}, createMilliFunction[K])
}
func createMilliFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*MilliArguments[K])

	if !ok {
		return nil, fmt.Errorf("MilliFactory args must be of type *MilliArguments[K]")
	}

	return Milli(args.Duration)
}

func Milli[K any](duration ottl.DurationGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		d, err := duration.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return d.Milliseconds(), nil
	}, nil
}
