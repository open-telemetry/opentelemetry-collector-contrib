// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"context"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type Float64Value interface {
	Value

	// Val returns the underlying value, if IsNil is true this panics.
	Val() float64
}

func NewFloat64Value(val float64) Float64Value {
	return newValue(val, false)
}

func NewNilFloat64Value() Float64Value {
	return newValue(float64(0), true)
}

type Float64Expr[K any] interface {
	Expr[K]

	Eval(ctx context.Context, tCtx K) (Float64Value, error)
}

func NewFloat64Expr[K any](exprFunc func(ctx context.Context, tCtx K) (Float64Value, error)) Float64Expr[K] {
	return newExpr(exprFunc)
}

func NewFloat64LiteralExpr[K any](val Float64Value) Float64Expr[K] {
	return newLiteralExpr[K](val)
}

func NewFloat64ExprGetter[K any](expr Float64Expr[K]) Float64Getter[K] {
	return newExprGetter(expr)
}

type Float64Getter[K any] interface {
	Getter[K]
	Get(ctx context.Context, tCtx K) (Float64Value, error)
}

func NewFloat64LiteralGetter[K any](val Float64Value) Float64Getter[K] {
	return newLiteralGetter[K](val)
}

func ToFloat64Getter[K any, VI any](get Getter[K]) (Float64Getter[K], error) {
	switch g := get.(type) {
	case StringGetter[K]:
		return newFloat64TransformGetter[K](g, float64TransformFromString)
	case Int64Getter[K]:
		return newFloat64TransformGetter[K](g, float64TransformFromInt64)
	case Float64Getter[K]:
		return g, nil
	case BoolGetter[K]:
		return newFloat64TransformGetter[K](g, float64TransformFromBool)
	case PValueGetter[K]:
		return newFloat64TransformGetter[K](g, float64TransformFromPValue)
	default:
		return nil, fmt.Errorf("unsupported type conversion from %T to \"float64\"", g)
	}
}

type Float64GetSetter[K any] interface {
	Float64Getter[K]
	Set(ctx context.Context, tCtx K, value Float64Value) error
}

func NewFloat64GetSetter[K any](getter GetFunc[K, Float64Value], setter SetFunc[K, Float64Value]) Float64GetSetter[K] {
	return &getSetter[K, Float64Value]{
		getter: getter,
		setter: setter,
	}
}

// float64TransformFromPValue is a transformFunc that is capable of transforming PValueValue into Float64Value
func float64TransformFromPValue(v PValueValue) (Float64Value, error) {
	if v.IsNil() {
		return NewNilFloat64Value(), nil
	}

	val := v.Val()
	switch val.Type() {
	case pcommon.ValueTypeInt:
		return NewFloat64Value(float64(val.Int())), nil
	case pcommon.ValueTypeDouble:
		return NewFloat64Value(val.Double()), nil
	case pcommon.ValueTypeStr:
		ret, err := strconv.ParseFloat(val.Str(), 64)
		if err != nil {
			return nil, nil
		}
		return NewFloat64Value(ret), nil
	case pcommon.ValueTypeBool:
		if val.Bool() {
			return NewFloat64Value(float64(1)), nil
		}
		return NewFloat64Value(float64(0)), nil
	default:
		return nil, fmt.Errorf("unsupported type conversion from pcommon.Map[%q] to \"float64\"", val.Type())
	}
}

// float64TransformFromString is a transformFunc that is capable of transforming StringValue into Float64Value
func float64TransformFromString(v StringValue) (Float64Value, error) {
	if v.IsNil() {
		return NewNilFloat64Value(), nil
	}
	val, err := strconv.ParseFloat(v.Val(), 64)
	if err != nil {
		return nil, err
	}
	return NewFloat64Value(val), nil
}

// float64TransformFromInt64 is a transformFunc that is capable of transforming Int64Value into Float64Value
func float64TransformFromInt64(v Int64Value) (Float64Value, error) {
	if v.IsNil() {
		return NewNilFloat64Value(), nil
	}
	return NewFloat64Value(float64(v.Val())), nil
}

// float64TransformFromBool is a transformFunc that is capable of transforming BoolValue into Float64Value
func float64TransformFromBool(v BoolValue) (Float64Value, error) {
	if v.IsNil() {
		return NewNilFloat64Value(), nil
	}
	if v.Val() {
		return NewFloat64Value(float64(1)), nil
	}
	return NewFloat64Value(float64(0)), nil
}

func newFloat64TransformGetter[K any, VI Value](get genericGetter[K, VI], trans transformFunc[VI, Float64Value]) (Float64Getter[K], error) {
	return newTransformGetter[K, VI](get, trans)
}
