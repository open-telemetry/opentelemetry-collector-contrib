// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ToTimeArguments[K any] struct {
	Time   ottl.StringGetter[K] `ottlarg:"0"`
	Format ottl.StringGetter[K] `ottlarg:"1"`
}

func NewToTimeFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("toTime", &ToTimeArguments[K]{}, createToTimeFunction[K])
}
func createToTimeFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ToTimeArguments[K])

	if !ok {
		return nil, fmt.Errorf("ToTimeFactory args must be of type *ToTimeArguments[K]")
	}

	return toTime(args.Time, args.Format)
}

func toTime[K any](inputTime ottl.StringGetter[K], format ottl.StringGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		t, err := inputTime.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if t == "" {
			return nil, fmt.Errorf("time cannot be nil")
		}
		f, err := format.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		var timestamp time.Time
		switch f {
		case "RFC3339":
			timestamp, err = time.Parse(time.RFC3339, t)
			if err != nil {
				return nil, err
			}
			return timestamp, nil
		default:
			timestamp, err = time.Parse(f, t)
			if err != nil {
				return nil, err
			}
			return timestamp, nil
		}
	}, nil
}
