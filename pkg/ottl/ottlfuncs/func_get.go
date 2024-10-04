// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type GetArguments[K any] struct {
	Value ottl.Getter[K]
}

func NewGetFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("get", &GetArguments[K]{}, createGetFunction[K])
}

func createGetFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*GetArguments[K])

	if !ok {
		return nil, fmt.Errorf("GetFactory args must be of type *GetArguments[K]")
	}

	return get(args.Value), nil
}

func get[K any](value ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		return value.Get(ctx, tCtx)
	}
}
