// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"context"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type BoolValue interface {
	Value

	// Val returns the underlying value, if IsNil is true this panics.
	Val() bool
}

func NewBoolValue(val bool) BoolValue {
	return newValue(val, false)
}

func NewNilBoolValue() BoolValue {
	return newValue(false, true)
}

type BoolExpr[K any] interface {
	Expr[K]

	Eval(ctx context.Context, tCtx K) (BoolValue, error)
}

func NewBoolExpr[K any](exprFunc func(ctx context.Context, tCtx K) (BoolValue, error)) BoolExpr[K] {
	return newExpr(exprFunc)
}

func NewBoolLiteralExpr[K any](val BoolValue) BoolExpr[K] {
	return newLiteralExpr[K](val)
}

func NewBoolExprGetter[K any](expr BoolExpr[K]) BoolGetter[K] {
	return newExprGetter(expr)
}

type BoolGetter[K any] interface {
	Getter[K]
	Get(ctx context.Context, tCtx K) (BoolValue, error)
}

func NewBoolLiteralGetter[K any](val BoolValue) BoolGetter[K] {
	return newLiteralGetter[K](val)
}

func ToBoolGetter[K any](get Getter[K]) (BoolGetter[K], error) {
	switch g := get.(type) {
	case StringGetter[K]:
		return newBoolTransformGetter(g, boolTransformFromString)
	case Int64Getter[K]:
		return newBoolTransformGetter(g, boolTransformFromInt64)
	case Float64Getter[K]:
		return newBoolTransformGetter(g, boolTransformFromFloat64)
	case BoolGetter[K]:
		return g, nil
	case PValueGetter[K]:
		return newBoolTransformGetter(g, boolTransformFromPValue)
	default:
		return nil, fmt.Errorf("unsupported type conversion from %T to \"bool\"", g)
	}
}

type BoolGetSetter[K any] interface {
	BoolGetter[K]
	Set(ctx context.Context, tCtx K, value BoolValue) error
}

func NewBoolGetSetter[K any](getter GetFunc[K, BoolValue], setter SetFunc[K, BoolValue]) BoolGetSetter[K] {
	return &getSetter[K, BoolValue]{
		getter: getter,
		setter: setter,
	}
}

// boolTransformFromPValue is a transformFunc that is capable of transforming PValueValue into BoolValue
func boolTransformFromPValue(v PValueValue) (BoolValue, error) {
	if v.IsNil() {
		return NewNilBoolValue(), nil
	}

	val := v.Val()
	switch val.Type() {
	case pcommon.ValueTypeInt:
		return NewBoolValue(val.Int() != 0), nil
	case pcommon.ValueTypeDouble:
		return NewBoolValue(val.Double() != 0), nil
	case pcommon.ValueTypeStr:
		ret, err := strconv.ParseBool(val.Str())
		if err != nil {
			return nil, nil
		}
		return NewBoolValue(ret), nil
	case pcommon.ValueTypeBool:
		return NewBoolValue(val.Bool()), nil
	default:
		return nil, fmt.Errorf("unsupported type conversion from pcommon.Value[%q] to \"bool\"", val.Type())
	}
}

// boolTransformFromString is a transformFunc that is capable of transforming StringValue into BoolValue
func boolTransformFromString(v StringValue) (BoolValue, error) {
	if v.IsNil() {
		return NewNilBoolValue(), nil
	}
	val, err := strconv.ParseBool(v.Val())
	if err != nil {
		return nil, err
	}
	return NewBoolValue(val), nil
}

// boolTransformFromInt64 is a transformFunc that is capable of transforming Int64Value into BoolValue
func boolTransformFromInt64(v Int64Value) (BoolValue, error) {
	if v.IsNil() {
		return NewNilBoolValue(), nil
	}
	return NewBoolValue(v.Val() != 0), nil
}

// boolTransformFromFloat64 is a transformFunc that is capable of transforming Float64Value into BoolValue
func boolTransformFromFloat64(v Float64Value) (BoolValue, error) {
	if v.IsNil() {
		return NewNilBoolValue(), nil
	}
	return NewBoolValue(v.Val() != 0), nil
}

func newBoolTransformGetter[K any, VI Value](get genericGetter[K, VI], trans transformFunc[VI, BoolValue]) (BoolGetter[K], error) {
	return newTransformGetter[K, VI](get, trans)
}
