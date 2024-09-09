// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"math"

	"github.com/dustin/go-humanize"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ParseBytesArguments[K any] struct {
	Target ottl.StringGetter[K]
}

func NewParseBytesFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseBytes", &ParseBytesArguments[K]{}, createParseBytesFunction[K])
}

func createParseBytesFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ParseBytesArguments[K])

	if !ok {
		return nil, fmt.Errorf("ParseBytesFactory args must be of type *ParseBytesArguments[K]")
	}

	return parseBytesFunc(args.Target), nil
}

func parseBytesFunc[K any](target ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		value, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		bytes, err := humanize.ParseBytes(value)

		if err != nil {
			return nil, err
		}

		if bytes > math.MaxInt64 {
			return nil, fmt.Errorf("too large: %v", value)
		}

		return int64(bytes), nil
	}
}
