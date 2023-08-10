// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type DurationToMilliArguments[K any] struct {
	Duration ottl.DurationGetter[K] `ottlarg:"0"`
}

func NewDurationToMilliFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("DurationToMilli", &DurationToMilliArguments[K]{}, createDurationToMilliFunction[K])
}
func createDurationToMilliFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DurationToMilliArguments[K])

	if !ok {
		return nil, fmt.Errorf("DurationToMilliFactory args must be of type *DurationToMilliArguments[K]")
	}

	return DurationToMilli(args.Duration)
}

func DurationToMilli[K any](duration ottl.DurationGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		d, err := duration.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return d.Milliseconds(), nil
	}, nil
}
