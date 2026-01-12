// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"context"
)

// Getter resolves a value at runtime from the context.
type Getter[K any] interface {
	// Disallow implementations outside this package.
	unexportedFunc()
}

// genericGetter represents the generic implementation of Getter.
type genericGetter[K any, V Value] interface {
	Getter[K]
	Get(ctx context.Context, tCtx K) (V, error)
}

type GetFunc[K any, V Value] func(ctx context.Context, tCtx K) (V, error)

type SetFunc[K any, V Value] func(ctx context.Context, tCtx K, val V) error

type getSetter[K any, V Value] struct {
	getter GetFunc[K, V]
	setter SetFunc[K, V]
}

func (gs *getSetter[K, V]) Get(ctx context.Context, tCtx K) (V, error) {
	return gs.getter(ctx, tCtx)
}

func (gs *getSetter[K, V]) Set(ctx context.Context, tCtx K, val V) error {
	return gs.setter(ctx, tCtx, val)
}

func (gs *getSetter[K, V]) unexportedFunc() {}

func newTransformGetter[K any, VI Value, VO Value](get genericGetter[K, VI], trans transformFunc[VI, VO]) (genericGetter[K, VO], error) {
	if g, ok := get.(*literalGetter[K, VI]); ok {
		val, err := trans(g.val)
		if err != nil {
			return nil, err
		}
		return newLiteralGetter[K, VO](val), nil
	}
	return &transformGetter[K, VI, VO]{
		get:   get,
		trans: trans,
	}, nil
}

type transformGetter[K any, VI Value, VO Value] struct {
	get   genericGetter[K, VI]
	trans transformFunc[VI, VO]
}

func (g *transformGetter[K, VI, VO]) unexportedFunc() {}

func (g *transformGetter[K, VI, VO]) Get(ctx context.Context, tCtx K) (VO, error) {
	val, err := g.get.Get(ctx, tCtx)
	if err != nil {
		var ret VO
		return ret, err
	}
	return g.trans(val)
}

func newLiteralGetter[K any, V Value](val V) *literalGetter[K, V] {
	return &literalGetter[K, V]{val: val}
}

type literalGetter[K any, V Value] struct {
	val V
}

func (l *literalGetter[K, V]) unexportedFunc() {}

func (l *literalGetter[K, V]) Get(context.Context, K) (V, error) {
	return l.val, nil
}

// GetLiteralValue retrieves the literal value from the given getter.
// If the getter is not a literal getter, or if the value it's currently holding is not a
// literal value, it returns the zero value of V and false.
func GetLiteralValue[K any, V Value](getter Getter[K]) (V, bool) {
	if l, ok := getter.(*literalGetter[K, V]); ok {
		return l.val, true
	}
	var ret V
	return ret, false
}
