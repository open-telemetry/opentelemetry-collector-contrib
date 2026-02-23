// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type DeleteMatchingValuesArguments[K any] struct {
	Target  ottl.PMapGetSetter[K]
	Pattern ottl.StringGetter[K]
}

func NewDeleteMatchingValuesFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("delete_matching_values", &DeleteMatchingValuesArguments[K]{}, createDeleteMatchingValuesFunction[K])
}

func createDeleteMatchingValuesFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*DeleteMatchingValuesArguments[K])

	if !ok {
		return nil, errors.New("DeleteMatchingValuesFactory args must be of type *DeleteMatchingValuesArguments[K]")
	}

	return deleteMatchingValues(args.Target, args.Pattern)
}

func deleteMatchingValues[K any](target ottl.PMapGetSetter[K], pattern ottl.StringGetter[K]) (ottl.ExprFunc[K], error) {
	compiledPattern, err := newDynamicRegex("delete_matching_values", pattern)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, tCtx K) (any, error) {
		cp, err := compiledPattern.compile(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		val.RemoveIf(func(_ string, value pcommon.Value) bool {
			return cp.MatchString(value.AsString())
		})
		return nil, target.Set(ctx, tCtx, val)
	}, nil
}
