// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type PValueValue interface {
	Value

	// Val returns the underlying value, if IsNil is true this panics.
	Val() pcommon.Value
}

func NewPValueValue(val pcommon.Value) PValueValue {
	return newValue(val, false)
}

func NewNilPValueValue() PValueValue {
	return newValue(pcommon.Value{}, true)
}

type PValueExpr[K any] interface {
	Expr[K]

	Eval(ctx context.Context, tCtx K) (PValueValue, error)
}

func NewPValueExpr[K any](exprFunc func(ctx context.Context, tCtx K) (PValueValue, error)) PValueExpr[K] {
	return newExpr(exprFunc)
}

func NewPValueLiteralExpr[K any](val PValueValue) PValueExpr[K] {
	return newLiteralExpr[K](val)
}

func NewPValueExprGetter[K any](expr PValueExpr[K]) PValueGetter[K] {
	return newExprGetter(expr)
}

type PValueGetter[K any] interface {
	Getter[K]
	Get(ctx context.Context, tCtx K) (PValueValue, error)
}

func NewPValueLiteralGetter[K any](val PValueValue) PValueGetter[K] {
	return newLiteralGetter[K](val)
}

func ToPValueGetter[K any](get Getter[K]) (PValueGetter[K], error) {
	switch g := get.(type) {
	case StringGetter[K]:
		return newPValueTransformGetter(g, pValueTransformFromString)
	case Int64Getter[K]:
		return newPValueTransformGetter(g, pValueTransformFromInt64)
	case Float64Getter[K]:
		return newPValueTransformGetter(g, pValueTransformFromFloat64)
	case BoolGetter[K]:
		return newPValueTransformGetter(g, pValueTransformFromBool)
	case PMapGetter[K]:
		return newPValueTransformGetter(g, pValueTransformFromPMap)
	case PSliceGetter[K]:
		return newPValueTransformGetter(g, pValueTransformFromPSlice)
	case PValueGetter[K]:
		return g, nil
	default:
		return nil, fmt.Errorf("unsupported type conversion from %T to \"bool\"", g)
	}
}

type PValueGetSetter[K any] interface {
	PValueGetter[K]
	Set(ctx context.Context, tCtx K, value PValueValue) error
}

func NewPValueGetSetter[K any](getter GetFunc[K, PValueValue], setter SetFunc[K, PValueValue]) PValueGetSetter[K] {
	return &getSetter[K, PValueValue]{
		getter: getter,
		setter: setter,
	}
}

// pValueTransformFromPMap is a transformFunc that is capable of transforming PMapValue into PValueValue
func pValueTransformFromPMap(v PMapValue) (PValueValue, error) {
	if v.IsNil() {
		return NewNilPValueValue(), nil
	}
	ret := pcommon.NewValueMap()
	v.Val().CopyTo(ret.Map())
	return NewPValueValue(ret), nil
}

// pValueTransformFromPSlice is a transformFunc that is capable of transforming PSliceValue into PValueValue
func pValueTransformFromPSlice(v PSliceValue) (PValueValue, error) {
	if v.IsNil() {
		return NewNilPValueValue(), nil
	}
	ret := pcommon.NewValueSlice()
	v.Val().CopyTo(ret.Slice())
	return NewPValueValue(ret), nil
}

// pValueTransformFromString is a transformFunc that is capable of transforming any value into PValueValue
func pValueTransformFromString(v StringValue) (PValueValue, error) {
	if v.IsNil() {
		return NewNilPValueValue(), nil
	}
	return NewPValueValue(pcommon.NewValueStr(v.Val())), nil
}

// pValueTransformFromInt64 is a transformFunc that is capable of transforming any Int64Value into PValueValue
func pValueTransformFromInt64(v Int64Value) (PValueValue, error) {
	if v.IsNil() {
		return NewNilPValueValue(), nil
	}
	return NewPValueValue(pcommon.NewValueInt(v.Val())), nil
}

// pValueTransformFromFloat64 is a transformFunc that is capable of transforming any Float64Value into PValueValue
func pValueTransformFromFloat64(v Float64Value) (PValueValue, error) {
	if v.IsNil() {
		return NewNilPValueValue(), nil
	}
	return NewPValueValue(pcommon.NewValueDouble(v.Val())), nil
}

// pValueTransformFromDefault is a transformFunc that is capable of transforming any BoolValue into PValueValue
func pValueTransformFromBool(v BoolValue) (PValueValue, error) {
	if v.IsNil() {
		return NewNilPValueValue(), nil
	}
	return NewPValueValue(pcommon.NewValueBool(v.Val())), nil
}

func newPValueTransformGetter[K any, VI Value](get genericGetter[K, VI], trans transformFunc[VI, PValueValue]) (PValueGetter[K], error) {
	return newTransformGetter[K, VI](get, trans)
}
