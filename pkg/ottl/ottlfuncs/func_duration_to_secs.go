// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type DurationToSecsArguments[K any] struct {
	Duration ottl.DurationGetter[K] `ottlarg:"0"`
}

func NewDurationToSecsFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("DurationToSecs", &DurationToSecsArguments[K]{}, createDurationToSecsFunction[K])
}
func createDurationToSecsFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DurationToSecsArguments[K])

	if !ok {
		return nil, fmt.Errorf("DurationToSecsFactory args must be of type *DurationToSecsArguments[K]")
	}

	return DurationToSecs(args.Duration)
}

func DurationToSecs[K any](duration ottl.DurationGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		d, err := duration.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return d.Seconds(), nil
	}, nil
}
