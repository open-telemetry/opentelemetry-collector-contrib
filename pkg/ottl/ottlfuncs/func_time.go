// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type TimeArguments[K any] struct {
	Time     ottl.StringGetter[K] `ottlarg:"0"`
	Format   ottl.StringGetter[K] `ottlarg:"1"`
	Location ottl.StringGetter[K] `ottlarg:"2"`
}

func NewTimeFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Time", &TimeArguments[K]{}, createTimeFunction[K])
}
func createTimeFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*TimeArguments[K])

	if !ok {
		return nil, fmt.Errorf("TimeFactory args must be of type *TimeArguments[K]")
	}

	return Time(args.Time, args.Format, args.Location)
}

func Time[K any](inputTime ottl.StringGetter[K], format ottl.StringGetter[K], location ottl.StringGetter[K]) (ottl.ExprFunc[K], error) {
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
		if f == "" {
			return nil, fmt.Errorf("format cannot be nil")
		}

		l, err := location.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		loc, err := time.LoadLocation(l)
		if err != nil {
			return nil, err
		}

		timestamp, err := timeutils.ParseStrptime(f, t, loc)
		if err != nil {
			return nil, err
		}
		return timestamp, nil
	}, nil
}
