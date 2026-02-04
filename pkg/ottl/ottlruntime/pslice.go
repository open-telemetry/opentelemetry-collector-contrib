// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type PSliceValue interface {
	Value

	// Val returns the underlying value, if IsNil is true this panics.
	Val() pcommon.Slice
}

func NewPSliceValue(val pcommon.Slice) PSliceValue {
	return newValue(val, false)
}

func NewNilPSliceValue() PSliceValue {
	return newValue(pcommon.Slice{}, true)
}

type PSliceExpr[K any] interface {
	Expr[K]

	Eval(ctx context.Context, tCtx K) (PSliceValue, error)
}

func NewPSliceExpr[K any](exprFunc func(ctx context.Context, tCtx K) (PSliceValue, error)) PSliceExpr[K] {
	return newExpr(exprFunc)
}

func NewPSliceLiteralExpr[K any](val PSliceValue) PSliceExpr[K] {
	return newLiteralExpr[K](val)
}

func NewPSliceExprGetter[K any](expr PSliceExpr[K]) PSliceGetter[K] {
	return newExprGetter(expr)
}

type PSliceGetter[K any] interface {
	Getter[K]
	Get(ctx context.Context, tCtx K) (PSliceValue, error)
}

func NewPSliceLiteralGetter[K any](val PSliceValue) PSliceGetter[K] {
	return newLiteralGetter[K](val)
}

func ToPSliceGetter[K any](get Getter[K]) (PSliceGetter[K], error) {
	switch g := get.(type) {
	case PValueGetter[K]:
		return newPSliceTransformGetter(g, pSliceTransformFromPValue)
	case PSliceGetter[K]:
		return g, nil
	default:
		return nil, fmt.Errorf("unsupported type conversion from %T to \"pcommon.Slice\"", g)
	}
}

type PSliceGetSetter[K any] interface {
	PSliceGetter[K]
	Set(ctx context.Context, tCtx K, value PSliceValue) error
}

func NewPSliceGetSetter[K any](getter GetFunc[K, PSliceValue], setter SetFunc[K, PSliceValue]) PSliceGetSetter[K] {
	return &getSetter[K, PSliceValue]{
		getter: getter,
		setter: setter,
	}
}

// pSliceTransformFromPSlice is a transformFunc that is capable of transforming PValueValue into PSliceValue
func pSliceTransformFromPValue(v PValueValue) (PSliceValue, error) {
	if v.IsNil() {
		return NewNilPSliceValue(), nil
	}
	val := v.Val()
	if val.Type() != pcommon.ValueTypeSlice {
		return nil, fmt.Errorf("invalid type conversion from pcommon.Value[%T] to \"pcommon.Slice\"", val)
	}
	return NewPSliceValue(val.Slice()), nil
}

func newPSliceTransformGetter[K any, VI Value](get genericGetter[K, VI], trans transformFunc[VI, PSliceValue]) (PSliceGetter[K], error) {
	return newTransformGetter[K, VI](get, trans)
}
