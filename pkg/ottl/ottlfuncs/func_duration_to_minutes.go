// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type DurationToMinsArguments[K any] struct {
	Duration ottl.StringGetter[K] `ottlarg:"0"`
}

func NewDurationToMinsFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("DurationToMins", &DurationToMinsArguments[K]{}, createDurationToMinsFunction[K])
}
func createDurationToMinsFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DurationToMinsArguments[K])

	if !ok {
		return nil, fmt.Errorf("DurationToMinsFactory args must be of type *DurationToMinsArguments[K]")
	}

	return DurationToMins(args.Duration)
}

func DurationToMins[K any](duration ottl.StringGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		d, err := duration.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		dur, err := time.ParseDuration(d)
		if err != nil {
			return nil, err
		}
		return dur.Minutes(), nil
	}, nil
}
