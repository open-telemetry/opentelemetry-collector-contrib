// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type SplitArguments[K any] struct {
	Target    ottl.StringGetter[K]
	Delimiter ottl.StringGetter[K]
}

func NewSplitFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Split", &SplitArguments[K]{}, createSplitFunction[K])
}

func createSplitFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SplitArguments[K])

	if !ok {
		return nil, errors.New("SplitFactory args must be of type *SplitArguments[K]")
	}

	return split(args.Target, args.Delimiter), nil
}

func split[K any](target, delimiter ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		delimiterVal, err := delimiter.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		res := strings.Split(val, delimiterVal)

		resPSlice := pcommon.NewSlice()
		resPSlice.EnsureCapacity(len(res))
		for _, s := range res {
			resPSlice.AppendEmpty().SetStr(s)
		}
		return resPSlice, nil
	}
}
