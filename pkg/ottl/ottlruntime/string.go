// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

type StringValue interface {
	Value

	// Val returns the underlying value, if IsNil is true this panics.
	Val() string
}

func NewStringValue(val string) StringValue {
	return newValue(val, false)
}

func NewNilStringValue() StringValue {
	return newValue("", true)
}

type StringExpr[K any] interface {
	Expr[K]

	Eval(ctx context.Context, tCtx K) (StringValue, error)
}

func NewStringExpr[K any](exprFunc func(ctx context.Context, tCtx K) (StringValue, error)) StringExpr[K] {
	return newExpr(exprFunc)
}

func NewStringLiteralExpr[K any](val StringValue) StringExpr[K] {
	return newLiteralExpr[K](val)
}

func NewStringExprGetter[K any](expr StringExpr[K]) StringGetter[K] {
	return newExprGetter(expr)
}

type StringGetter[K any] interface {
	Getter[K]
	Get(ctx context.Context, tCtx K) (StringValue, error)
}

func NewStringLiteralGetter[K any](val StringValue) StringGetter[K] {
	return newLiteralGetter[K](val)
}

func ToStringGetter[K any](get Getter[K]) (StringGetter[K], error) {
	switch g := get.(type) {
	case StringGetter[K]:
		return g, nil
	case Int64Getter[K]:
		return newStringTransformGetter[K](g, stringTransformFromInt64)
	case Float64Getter[K]:
		return newStringTransformGetter[K](g, stringTransformFromFloat64)
	case BoolGetter[K]:
		return newStringTransformGetter[K](g, stringTransformFromBool)
	case PMapGetter[K]:
		return newStringTransformGetter[K](g, stringTransformFromPMap)
	case PSliceGetter[K]:
		return newStringTransformGetter[K](g, stringTransformFromPSlice)
	case PValueGetter[K]:
		return newStringTransformGetter[K](g, stringTransformFromPValue)
	default:
		return nil, fmt.Errorf("unsupported type conversion from %T to \"string\"", g)
	}
}

type StringGetSetter[K any] interface {
	StringGetter[K]
	Set(ctx context.Context, tCtx K, value StringValue) error
}

func NewStringGetSetter[K any](getter GetFunc[K, StringValue], setter SetFunc[K, StringValue]) StringGetSetter[K] {
	return &getSetter[K, StringValue]{
		getter: getter,
		setter: setter,
	}
}

// stringTransformFromPValue is a transformFunc that is capable of transforming pcommon.Value into string
func stringTransformFromPValue(v PValueValue) (StringValue, error) {
	if v.IsNil() {
		return NewNilStringValue(), nil
	}
	return NewStringValue(v.Val().AsString()), nil
}

// stringTransformFromPMap is a transformFunc that is capable of transforming pcommon.Map into string
func stringTransformFromPMap(v PMapValue) (StringValue, error) {
	if v.IsNil() {
		return NewNilStringValue(), nil
	}
	val, err := json.Marshal(v.Val().AsRaw())
	if err != nil {
		return nil, err
	}
	return NewStringValue(string(val)), nil
}

// stringTransformFromPSlice is a transformFunc that is capable of transforming pcommon.Slice into string
func stringTransformFromPSlice(v PSliceValue) (StringValue, error) {
	if v.IsNil() {
		return NewNilStringValue(), nil
	}
	val, err := json.Marshal(v.Val().AsRaw())
	if err != nil {
		return nil, err
	}
	return NewStringValue(string(val)), nil
}

// stringTransformFromInt64 is a transformFunc that is capable of transforming any Int64Value into string
func stringTransformFromInt64(v Int64Value) (StringValue, error) {
	if v.IsNil() {
		return NewNilStringValue(), nil
	}
	return NewStringValue(strconv.FormatInt(v.Val(), 10)), nil
}

// stringTransformFromFloat64 is a transformFunc that is capable of transforming any value into string
func stringTransformFromFloat64(v Float64Value) (StringValue, error) {
	if v.IsNil() {
		return NewNilStringValue(), nil
	}
	return NewStringValue(strconv.FormatFloat(v.Val(), 'g', -1, 64)), nil
}

// stringTransformFromBool is a transformFunc that is capable of transforming any value into string
func stringTransformFromBool(v BoolValue) (StringValue, error) {
	if v.IsNil() {
		return NewNilStringValue(), nil
	}
	return NewStringValue(strconv.FormatBool(v.Val())), nil
}

func newStringTransformGetter[K any, VI Value](get genericGetter[K, VI], trans transformFunc[VI, StringValue]) (StringGetter[K], error) {
	return newTransformGetter[K, VI](get, trans)
}
