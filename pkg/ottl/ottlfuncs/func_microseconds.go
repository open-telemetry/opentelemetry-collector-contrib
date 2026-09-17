// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type microsecondsArguments[K any] struct {
	Duration ottl.DurationGetter[K]
}

func NewMicrosecondsFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Microseconds", &microsecondsArguments[K]{}, createMicrosecondsFunction[K])
}

func createMicrosecondsFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*microsecondsArguments[K])

	if !ok {
		return nil, errors.New("MicrosecondsFactory args must be of type *microsecondsArguments[K]")
	}

	return microseconds(args.Duration), nil
}

func microseconds[K any](duration ottl.DurationGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		d, err := duration.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return d.Microseconds(), nil
	}
}
