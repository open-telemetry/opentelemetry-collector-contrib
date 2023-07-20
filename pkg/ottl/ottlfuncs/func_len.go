// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type LenArguments[K any] struct {
	Target ottl.StringGetter[K] `ottlarg:"0"`
}

func NewLenFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Len", &LenArguments[K]{}, createLenFunction[K])
}

func createLenFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*LenArguments[K])

	if !ok {
		return nil, fmt.Errorf("LenFactory args must be of type *LenArguments[K]")
	}

	return strLen(args.Target), nil
}

func strLen[K any](target ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return int64(len(val)), nil
	}
}
