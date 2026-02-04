// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"context"
)

// Value represents a runtime value which can be nil or a concrete value type.
type Value interface {
	IsNil() bool

	// Disallow implementations outside this package.
	unexportedFunc()
}

type value[V any] struct {
	v     V
	isNil bool
}

func newValue[V any](v V, isNil bool) value[V] {
	return value[V]{v: v, isNil: isNil}
}

func (v value[V]) unexportedFunc() {}

func (v value[V]) IsNil() bool {
	return v.isNil
}

func (v value[V]) Val() V {
	if v.isNil {
		panic("null pointer exception")
	}
	return v.v
}

// transformFunc is a transformation function that transforms a function from VI to V0.
type transformFunc[VI Value, VO Value] func(VI) (VO, error)

func newValueGetter[K any, V Value](val V) Getter[K] {
	return valueGetter[K, V]{val: val}
}

type valueGetter[K any, V Value] struct {
	val V
}

func (vg valueGetter[K, V]) unexportedFunc() {}

func (vg valueGetter[K, V]) Get(context.Context, K) (V, error) {
	return vg.val, nil
}
