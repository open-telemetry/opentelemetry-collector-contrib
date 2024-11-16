// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type TrimArguments[K any] struct {
	Target ottl.StringGetter[K]
}

func NewTrimFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Trim", &TrimArguments[K]{}, createTrimFunction[K])
}

func createTrimFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*TrimArguments[K])

	if !ok {
		return nil, fmt.Errorf("TrimFactory args must be of type *TrimArguments[K]")
	}

	return trim(args.Target), nil
}

func trim[K any](target ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return strings.Trim(val, " "), nil
	}
}
