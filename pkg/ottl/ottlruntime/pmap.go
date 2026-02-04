// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type PMapValue interface {
	Value

	// Val returns the underlying value, if IsNil is true this panics.
	Val() pcommon.Map
}

func NewPMapValue(val pcommon.Map) PMapValue {
	return newValue(val, false)
}

func NewNilPMapValue() PMapValue {
	return newValue(pcommon.Map{}, true)
}

type PMapExpr[K any] interface {
	Expr[K]

	Eval(ctx context.Context, tCtx K) (PMapValue, error)
}

func NewPMapExpr[K any](exprFunc func(ctx context.Context, tCtx K) (PMapValue, error)) PMapExpr[K] {
	return newExpr(exprFunc)
}

func NewPMapLiteralExpr[K any](val PMapValue) PMapExpr[K] {
	return newLiteralExpr[K](val)
}

func NewPMapExprGetter[K any](expr PMapExpr[K]) PMapGetter[K] {
	return newExprGetter(expr)
}

type PMapGetter[K any] interface {
	Getter[K]
	Get(ctx context.Context, tCtx K) (PMapValue, error)
}

func NewPMapLiteralGetter[K any](val PMapValue) PMapGetter[K] {
	return newLiteralGetter[K](val)
}

func ToPMapGetter[K any](get Getter[K]) (PMapGetter[K], error) {
	switch g := get.(type) {
	case PValueGetter[K]:
		return newPMapTransformGetter(g, pMapTransformFromPValue)
	case PMapGetter[K]:
		return g, nil
	default:
		return nil, fmt.Errorf("unsupported type conversion from %T to \"pcommon.Map\"", g)
	}
}

type PMapGetSetter[K any] interface {
	PMapGetter[K]
	Set(ctx context.Context, tCtx K, value PMapValue) error
}

func NewPMapGetSetter[K any](getter GetFunc[K, PMapValue], setter SetFunc[K, PMapValue]) PMapGetSetter[K] {
	return &getSetter[K, PMapValue]{
		getter: getter,
		setter: setter,
	}
}

// pMapTransformFromPMap is a transformFunc that is capable of transforming PValueValue into PMapValue
func pMapTransformFromPValue(v PValueValue) (PMapValue, error) {
	if v.IsNil() {
		return NewNilPMapValue(), nil
	}
	val := v.Val()
	if val.Type() != pcommon.ValueTypeMap {
		return nil, fmt.Errorf("invalid type conversion from pcommon.Value[%T] to \"pcommon.Map\"", val)
	}
	return NewPMapValue(val.Map()), nil
}

func newPMapTransformGetter[K any, VI Value](get genericGetter[K, VI], trans transformFunc[VI, PMapValue]) (PMapGetter[K], error) {
	return newTransformGetter[K, VI](get, trans)
}
