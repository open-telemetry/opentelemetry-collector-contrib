// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type comprehensionContext[K any] struct {
	underlying   K
	currentValue any
	index        int
}

// An expression like `[{yield} for {list} where {cond}]`.
type comprehensionExpr[K any] struct {
	// The list we're iterating over.
	listExpr Expr[K]
	// When is the iterator over. This is against the comprhension
	condExpr BoolExpr[comprehensionContext[K]]
	// The expression to evaluate at each item.
	yieldExpr Expr[comprehensionContext[K]]
}

func (c comprehensionExpr[K]) Get(ctx context.Context, tCtx K) (any, error) {
	listResult, err := c.listExpr.Eval(ctx, tCtx)
	if err != nil {
		return nil, err
	}
	var list []any
	switch l := listResult.(type) {
	case pcommon.Slice:
		list = l.AsRaw()
	case []any:
		list = l
	default:
		return nil, fmt.Errorf("unsupported list type: %s", reflect.TypeOf(listResult).Name())
	}
	result := []any{}
	for i, v := range list {
		subCtx := comprehensionContext[K]{tCtx, v, i}

		cond, err := c.condExpr.Eval(ctx, subCtx)
		if err != nil {
			return nil, err
		}
		if cond {
			// Evaluate next value for result.
			item, err := c.yieldExpr.Eval(ctx, subCtx)
			if err != nil {
				return nil, err
			}
			// TODO - cast the item?
			result = append(result, item)
		}
	}
	// We always return results in a pslice.
	sr := pcommon.NewSlice()
	sr.FromRaw(result)
	return sr, nil
}
