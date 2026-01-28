// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"context"
)

// Expr is the base struct that represents a function call.
type Expr[K any] interface {
	// Disallow implementations outside this package.
	unexportedFunc()
}

type genericExpr[K any, V Value] interface {
	Expr[K]

	Eval(ctx context.Context, tCtx K) (V, error)
}

func newExpr[K any, V Value](exprFunc func(ctx context.Context, tCtx K) (V, error)) *expr[K, V] {
	return &expr[K, V]{exprFunc: exprFunc}
}

type expr[K any, V Value] struct {
	exprFunc func(ctx context.Context, tCtx K) (V, error)
}

func (e *expr[K, V]) unexportedFunc() {}

func (e *expr[K, V]) Eval(ctx context.Context, tCtx K) (V, error) {
	return e.exprFunc(ctx, tCtx)
}

func newExprGetter[K any, V Value](expr genericExpr[K, V]) genericGetter[K, V] {
	if e, ok := expr.(*literalExpr[K, V]); ok {
		return newLiteralGetter[K, V](e.val)
	}
	return &exprGetter[K, V]{genericExpr: expr}
}

type exprGetter[K any, V Value] struct {
	genericExpr[K, V]
}

func (v exprGetter[K, V]) Get(ctx context.Context, tCtx K) (V, error) {
	return v.Eval(ctx, tCtx)
}

func newLiteralExpr[K any, V Value](val V) *literalExpr[K, V] {
	return &literalExpr[K, V]{val: val}
}

type literalExpr[K any, V Value] struct {
	val V
}

func (e *literalExpr[K, V]) unexportedFunc() {}

func (e *literalExpr[K, V]) Eval(context.Context, K) (V, error) {
	return e.val, nil
}
