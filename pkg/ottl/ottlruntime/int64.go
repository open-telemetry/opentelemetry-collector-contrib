// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"context"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type Int64Value interface {
	Value

	// Val returns the underlying value, if IsNil is true this panics.
	Val() int64
}

func NewInt64Value(val int64) Int64Value {
	return newValue(val, false)
}

func NewNilInt64Value() Int64Value {
	return newValue(int64(0), true)
}

type Int64Expr[K any] interface {
	Expr[K]

	Eval(ctx context.Context, tCtx K) (Int64Value, error)
}

func NewInt64Expr[K any](exprFunc func(ctx context.Context, tCtx K) (Int64Value, error)) Int64Expr[K] {
	return newExpr(exprFunc)
}

func NewInt64LiteralExpr[K any](val Int64Value) Int64Expr[K] {
	return newLiteralExpr[K](val)
}

func NewInt64ExprGetter[K any](expr Int64Expr[K]) Int64Getter[K] {
	return newExprGetter(expr)
}

type Int64Getter[K any] interface {
	Getter[K]
	Get(ctx context.Context, tCtx K) (Int64Value, error)
}

func NewInt64LiteralGetter[K any](val Int64Value) Int64Getter[K] {
	return newLiteralGetter[K](val)
}

func ToInt64Getter[K any](get Getter[K]) (Int64Getter[K], error) {
	switch g := get.(type) {
	case StringGetter[K]:
		return newInt64TransformGetter(g, int64TransformFromString)
	case Int64Getter[K]:
		return g, nil
	case Float64Getter[K]:
		return newInt64TransformGetter(g, int64TransformFromFloat64)
	case BoolGetter[K]:
		return newInt64TransformGetter(g, int64TransformFromBool)
	case PValueGetter[K]:
		return newInt64TransformGetter(g, int64TransformFromPValue)
	default:
		return nil, fmt.Errorf("unsupported type conversion from %T to \"bool\"", g)
	}
}

type Int64GetSetter[K any] interface {
	Int64Getter[K]
	Set(ctx context.Context, tCtx K, value Int64Value) error
}

func NewInt64GetSetter[K any](getter GetFunc[K, Int64Value], setter SetFunc[K, Int64Value]) Int64GetSetter[K] {
	return &getSetter[K, Int64Value]{
		getter: getter,
		setter: setter,
	}
}

// int64TransformFromPValue is a transformFunc that is capable of transforming pcommon.Value into Int64Value
func int64TransformFromPValue(v PValueValue) (Int64Value, error) {
	if v.IsNil() {
		return NewNilInt64Value(), nil
	}

	val := v.Val()
	switch val.Type() {
	case pcommon.ValueTypeInt:
		return NewInt64Value(val.Int()), nil
	case pcommon.ValueTypeDouble:
		return NewInt64Value(int64(val.Double())), nil
	case pcommon.ValueTypeStr:
		ret, err := strconv.ParseInt(val.Str(), 10, 64)
		if err != nil {
			return nil, nil
		}
		return NewInt64Value(ret), nil
	case pcommon.ValueTypeBool:
		if val.Bool() {
			return NewInt64Value(int64(1)), nil
		}
		return NewInt64Value(int64(0)), nil
	default:
		return nil, fmt.Errorf("unsupported type conversion from pcommon.Map[%q] to \"int64\"", val.Type())
	}
}

// int64TransformFromString is a transformFunc that is capable of transforming string into Int64Value
func int64TransformFromString(v StringValue) (Int64Value, error) {
	if v.IsNil() {
		return NewNilInt64Value(), nil
	}
	val, err := strconv.ParseInt(v.Val(), 10, 64)
	if err != nil {
		return nil, err
	}
	return NewInt64Value(val), nil
}

// int64TransformFromFloat64 is a transformFunc that is capable of transforming float64 into Int64Value
func int64TransformFromFloat64(v Float64Value) (Int64Value, error) {
	if v.IsNil() {
		return NewNilInt64Value(), nil
	}
	return NewInt64Value(int64(v.Val())), nil
}

// int64TransformFromBool is a transformFunc that is capable of transforming bool into Int64Value
func int64TransformFromBool(v BoolValue) (Int64Value, error) {
	if v.IsNil() {
		return NewNilInt64Value(), nil
	}
	if v.Val() {
		return NewInt64Value(int64(1)), nil
	}
	return NewInt64Value(int64(0)), nil
}

func newInt64TransformGetter[K any, VI Value](get genericGetter[K, VI], trans transformFunc[VI, Int64Value]) (Int64Getter[K], error) {
	return newTransformGetter[K, VI](get, trans)
}
