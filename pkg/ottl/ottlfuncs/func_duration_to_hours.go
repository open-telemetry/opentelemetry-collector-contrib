// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type DurationToHoursArguments[K any] struct {
	Duration ottl.DurationGetter[K] `ottlarg:"0"`
}

func NewDurationToHoursFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("DurationToHours", &DurationToHoursArguments[K]{}, createDurationToHoursFunction[K])
}
func createDurationToHoursFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DurationToHoursArguments[K])

	if !ok {
		return nil, fmt.Errorf("DurationToHoursFactory args must be of type *DurationToHoursArguments[K]")
	}

	return DurationToHours(args.Duration)
}

func DurationToHours[K any](duration ottl.DurationGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		d, err := duration.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return d.Hours(), nil
	}, nil
}
