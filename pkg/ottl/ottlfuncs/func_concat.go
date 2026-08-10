// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ConcatArguments[K any] struct {
	Vals      ottl.StringLikeSliceGetter[K]
	Delimiter ottl.StringGetter[K]
}

func NewConcatFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Concat", &ConcatArguments[K]{}, createConcatFunction[K])
}

func createConcatFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ConcatArguments[K])

	if !ok {
		return nil, errors.New("ConcatFactory args must be of type *ConcatArguments[K]")
	}

	return concat(args.Vals, args.Delimiter), nil
}

func concat[K any](vals ottl.StringLikeSliceGetter[K], delimiter ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		delimiterVal, err := delimiter.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		vs, err := vals.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return strings.Join(vs, delimiterVal), nil
	}
}
