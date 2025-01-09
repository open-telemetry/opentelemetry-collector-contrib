// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type TimestampArguments[K any] struct {
	Time   ottl.TimeGetter[K]
	Format string
}

func NewTimestampFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Timestamp", &TimestampArguments[K]{}, createTimestampFunction[K])
}

func createTimestampFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*TimestampArguments[K])

	if !ok {
		return nil, fmt.Errorf("TimestampFactory args must be of type *TimestampArguments[K]")
	}

	return Timestamp(args.Time, args.Format)
}

func Timestamp[K any](timeValue ottl.TimeGetter[K], format string) (ottl.ExprFunc[K], error) {
	if format == "" {
		return nil, fmt.Errorf("format cannot be nil")
	}

	gotimeFormat, err := timeutils.StrptimeToGotime(format)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		t, err := timeValue.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		return t.Format(gotimeFormat), nil
	}, nil
}
