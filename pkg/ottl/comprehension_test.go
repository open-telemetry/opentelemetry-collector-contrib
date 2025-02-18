// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type testListGetter[K any] struct{}

func (t testListGetter[K]) Get(ctx context.Context, tCtx K) (any, error) {
	return tCtx, nil
}

type testYieldExpr struct {
	Yield func(tCtx comprehensionContext[any]) (any, error)
}

func (t testYieldExpr) Get(ctx context.Context, tCtx comprehensionContext[any]) (any, error) {
	return t.Yield(tCtx)
}

func Test_comprehensions(t *testing.T) {

	tests := []struct {
		name       string
		collection any
		want       any
		expr       comprehensionExpr[any]
	}{
		{
			name:       "return same collection",
			collection: []any{"one", "two"},
			want: func() any {
				r := pcommon.NewSlice()
				r.FromRaw([]any{"one", "two"})
				return r
			}(),
			expr: comprehensionExpr[any]{
				currentValueId: "x",
				listExpr:       testListGetter[any]{},
				condExpr: BoolExpr[comprehensionContext[any]]{
					func(ctx context.Context, tCtx comprehensionContext[any]) (bool, error) {
						return true, nil
					},
				},
				yieldExpr: &testYieldExpr{
					Yield: func(tCtx comprehensionContext[any]) (any, error) {
						return tCtx.currentValue, nil
					},
				},
			},
		},
		{
			name:       "return first item of collection",
			collection: []any{"one", "two"},
			want: func() any {
				r := pcommon.NewSlice()
				r.FromRaw([]any{"one"})
				return r
			}(),
			expr: comprehensionExpr[any]{
				currentValueId: "x",
				listExpr:       testListGetter[any]{},
				condExpr: BoolExpr[comprehensionContext[any]]{
					func(ctx context.Context, tCtx comprehensionContext[any]) (bool, error) {
						return tCtx.index == 0, nil
					},
				},
				yieldExpr: &testYieldExpr{
					Yield: func(tCtx comprehensionContext[any]) (any, error) {
						return tCtx.currentValue, nil
					},
				},
			},
		},
		{
			name:       "return appended strings of collection",
			collection: []any{"one", "two"},
			want: func() any {
				r := pcommon.NewSlice()
				r.FromRaw([]any{"one-extra", "two-extra"})
				return r
			}(),
			expr: comprehensionExpr[any]{
				currentValueId: "x",
				listExpr:       testListGetter[any]{},
				condExpr: BoolExpr[comprehensionContext[any]]{
					func(ctx context.Context, tCtx comprehensionContext[any]) (bool, error) {
						return true, nil
					},
				},
				yieldExpr: &testYieldExpr{
					Yield: func(tCtx comprehensionContext[any]) (any, error) {
						return fmt.Sprintf("%s-extra", tCtx.currentValue), nil
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.expr.Get(context.Background(), tt.collection)
			assert.Nil(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}
