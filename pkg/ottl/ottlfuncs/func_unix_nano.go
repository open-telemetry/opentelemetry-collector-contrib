// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type unixNanoArguments[K any] struct {
	Time ottl.TimeGetter[K]
}

func NewUnixNanoFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("UnixNano", &unixNanoArguments[K]{}, createUnixNanoFunction[K])
}

func createUnixNanoFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*unixNanoArguments[K])

	if !ok {
		return nil, errors.New("UnixNanoFactory args must be of type *unixNanoArguments[K]")
	}

	return unixNano(args.Time), nil
}

func unixNano[K any](inputTime ottl.TimeGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		t, err := inputTime.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return t.UnixNano(), nil
	}
}
