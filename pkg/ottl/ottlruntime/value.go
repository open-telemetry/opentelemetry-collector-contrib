// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

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

func (value[V]) unexportedFunc() {}

func (v value[V]) IsNil() bool {
	return v.isNil
}

func (v value[V]) Val() V {
	if v.isNil {
		panic("null pointer exception")
	}
	return v.v
}
