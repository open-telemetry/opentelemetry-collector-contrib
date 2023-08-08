// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type DurationToMicroArguments[K any] struct {
	Duration ottl.StringGetter[K] `ottlarg:"0"`
}

func NewDurationToMicroFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("DurationToMicro", &DurationToMicroArguments[K]{}, createDurationToMicroFunction[K])
}
func createDurationToMicroFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DurationToMicroArguments[K])

	if !ok {
		return nil, fmt.Errorf("DurationToMicroFactory args must be of type *DurationToMicroArguments[K]")
	}

	return DurationToMicro(args.Duration)
}

func DurationToMicro[K any](duration ottl.StringGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		d, err := duration.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		dur, err := time.ParseDuration(d)
		if err != nil {
			return nil, err
		}
		return dur.Microseconds(), nil
	}, nil
}
