// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"context"
)

// Getter resolves a value at runtime without performing any type checking on the value that is returned.
type Getter[K any, V any] interface {
	// Get retrieves a value of type 'Any' and returns an error if there are any issues during retrieval.
	Get(ctx context.Context, tCtx K) (V, error)
}

type GetFunc[K any, V any] func(ctx context.Context, tCtx K) (V, error)

// Setter allows setting an untyped value on a predefined field within some data at runtime.
type Setter[K any, V any] interface {
	// Set sets a value of type 'Any' and returns an error if there are any issues during the setting process.
	Set(ctx context.Context, tCtx K, val V) error
}

type SetFunc[K any, V any] func(ctx context.Context, tCtx K, val V) error

// GetSetter is an interface that combines the Getter and Setter interfaces.
// It should be used to represent the ability to both get and set a value.
type GetSetter[K any, V any] interface {
	Getter[K, V]
	Setter[K, V]
}

var _ GetSetter[any, any] = (*getSetter[any, any])(nil)

type getSetter[K any, V any] struct {
	getter GetFunc[K, V]
	setter SetFunc[K, V]
}

func NewGetSetter[K any, V any](getter GetFunc[K, V], setter SetFunc[K, V]) GetSetter[K, V] {
	return &getSetter[K, V]{
		getter: getter,
		setter: setter,
	}
}

func (gs *getSetter[K, V]) Get(ctx context.Context, tCtx K) (V, error) {
	return gs.getter(ctx, tCtx)
}

func (gs *getSetter[K, V]) Set(ctx context.Context, tCtx K, val V) error {
	return gs.setter(ctx, tCtx, val)
}
