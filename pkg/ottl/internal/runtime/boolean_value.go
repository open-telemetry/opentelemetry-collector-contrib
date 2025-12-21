// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package runtime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/runtime"

import (
	"context"
	"fmt"
)

// BoolExpr represents a condition in OTTL
type BoolExpr[K any] interface {
	Eval(ctx context.Context, tCtx K) (bool, error)
}

type literalBoolExpr[K any] = literalExpr[K, bool]

func NewAlwaysTrue[K any]() BoolExpr[K] {
	return newLiteralExpr[K](true)
}

func NewAlwaysFalse[K any]() BoolExpr[K] {
	return newLiteralExpr[K](false)
}

func NewNot[K any](expr BoolExpr[K]) BoolExpr[K] {
	if f, ok := expr.(*literalBoolExpr[K]); ok {
		return newLiteralExpr[K](!f.getValue())
	}

	return &notBoolExpr[K]{expr: expr}
}

type notBoolExpr[K any] struct {
	expr BoolExpr[K]
}

// Eval evaluates an OTTL condition
func (e *notBoolExpr[K]) Eval(ctx context.Context, tCtx K) (bool, error) {
	val, err := e.expr.Eval(ctx, tCtx)
	return !val, err
}

// NewAndExprs a BoolExpr that returns a short-circuited result of ANDing
// boolExpressionEvaluator funcs
func NewAndExprs[K any](exprs []BoolExpr[K]) BoolExpr[K] {
	newExprs := make([]BoolExpr[K], 0, len(exprs))
	for i := range exprs {
		// If any literal evaluates to false, we can simply return false.
		// If a literal evaluates to true, it won't affect the expression's outcome, so we can skip evaluating it again.
		if f, ok := exprs[i].(*literalBoolExpr[K]); ok {
			if !f.getValue() {
				return NewAlwaysFalse[K]()
			}
			continue
		}
		newExprs = append(newExprs, exprs[i])
	}

	// All expressions evaluated to true, just return literal true.
	if len(newExprs) == 0 {
		return NewAlwaysTrue[K]()
	}

	// One expression left, no need to wrap in "andExprs".
	if len(newExprs) == 1 {
		return newExprs[0]
	}

	return &andExprs[K]{exprs: newExprs}
}

type andExprs[K any] struct {
	exprs []BoolExpr[K]
}

// Eval evaluates an OTTL condition
func (e *andExprs[K]) Eval(ctx context.Context, tCtx K) (bool, error) {
	for _, f := range e.exprs {
		result, err := f.Eval(ctx, tCtx)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil
		}
	}
	return true, nil
}

// NewOrExprs a BoolExpr that returns a short-circuited result of ORing
// boolExpressionEvaluator funcs
func NewOrExprs[K any](exprs []BoolExpr[K]) BoolExpr[K] {
	newExprs := make([]BoolExpr[K], 0, len(exprs))
	// If any literal evaluates to true, we can simply return true.
	// If a literal evaluates to false, it won't affect the expression's outcome, so we can skip evaluating it again.
	for i := range exprs {
		if f, ok := exprs[i].(*literalBoolExpr[K]); ok {
			if f.getValue() {
				return NewAlwaysTrue[K]()
			}
			continue
		}
		newExprs = append(newExprs, exprs[i])
	}

	// All expressions evaluated to false, just return literal false.
	if len(newExprs) == 0 {
		return NewAlwaysFalse[K]()
	}

	// One expression left, no need to wrap in "orExprs".
	if len(newExprs) == 1 {
		return newExprs[0]
	}

	return &orExprs[K]{exprs: newExprs}
}

type orExprs[K any] struct {
	exprs []BoolExpr[K]
}

// Eval evaluates an OTTL condition
func (e *orExprs[K]) Eval(ctx context.Context, tCtx K) (bool, error) {
	for _, f := range e.exprs {
		result, err := f.Eval(ctx, tCtx)
		if err != nil {
			return false, err
		}
		if result {
			return true, nil
		}
	}
	return false, nil
}

func NewComparisonExpr[K any](left, right Getter[K, any], op CompareOp) BoolExpr[K] {
	comparator := NewValueComparator()
	if leftVal, leftOk := GetLiteralValue(left); leftOk {
		if rightVal, rightOk := GetLiteralValue(right); rightOk {
			return newLiteralExpr[K](comparator.compare(leftVal, rightVal, op))
		}
	}

	// The parser ensures that we'll never get an invalid comparison.Op, so we don't have to check that case.
	return &comparisonExpr[K]{left: left, right: right, comparator: comparator, op: op}
}

type comparisonExpr[K any] struct {
	left, right Getter[K, any]
	comparator  *ottlValueComparator
	op          CompareOp
}

func (e *comparisonExpr[K]) Eval(ctx context.Context, tCtx K) (bool, error) {
	a, leftErr := e.left.Get(ctx, tCtx)
	if leftErr != nil {
		return false, leftErr
	}
	b, rightErr := e.right.Get(ctx, tCtx)
	if rightErr != nil {
		return false, rightErr
	}
	return e.comparator.compare(a, b, e.op), nil
}

func NewConverterExpr[K any](getter Getter[K, any]) (BoolExpr[K], error) {
	if val, ok := GetLiteralValue(getter); ok {
		boolResult, okResult := val.(bool)
		if !okResult {
			return nil, fmt.Errorf("value returned from Converter in constant expression must be bool but got %T", val)
		}
		return newLiteralExpr[K](boolResult), nil
	}

	return &converterExpr[K]{getter: getter}, nil
}

type converterExpr[K any] struct {
	getter Getter[K, any]
}

func (e *converterExpr[K]) Eval(ctx context.Context, tCtx K) (bool, error) {
	result, err := e.getter.Get(ctx, tCtx)
	if err != nil {
		return false, err
	}
	boolResult, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("value returned from Converter in constant expression must be bool but got %T", result)
	}
	return boolResult, nil
}
