// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type DurationToNanoArguments[K any] struct {
	Duration ottl.DurationGetter[K] `ottlarg:"0"`
}

func NewDurationToNanoFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("DurationToNano", &DurationToNanoArguments[K]{}, createDurationToNanoFunction[K])
}
func createDurationToNanoFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DurationToNanoArguments[K])

	if !ok {
		return nil, fmt.Errorf("DurationToNanoFactory args must be of type *DurationToNanoArguments[K]")
	}

	return DurationToNano(args.Duration)
}

func DurationToNano[K any](duration ottl.DurationGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		d, err := duration.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return d.Nanoseconds(), nil
	}, nil
}
