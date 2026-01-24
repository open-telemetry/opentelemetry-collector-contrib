// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlpath // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlpath"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"
)

// Option represents options for the Factory creation.
type Option[K any] interface {
	apply(Factory[K])
}

// Factory defines an OTTL context factory that will generate an OTTL
// value getter from the OTTL context to be called within a statement.
type Factory[K any] interface {
	// Disallow implementations outside this package.
	unexportedFactoryFunc()
}

type factory[K any, V any] struct {
	getSetter ottlruntime.GetSetter[K, V]
}

func (f *factory[K, V]) Create(ctx context.Context, set component.TelemetrySettings) (ottlruntime.GetSetter[K, V], error) {
	return f.getSetter, nil
}

func (f *factory[K, V]) unexportedFactoryFunc() {}

// StringFactory is a Factory that returns a GetSetter that works with string.
type StringFactory[K any] interface {
	Factory[K]
	Create(ctx context.Context, set component.TelemetrySettings) (ottlruntime.GetSetter[K, string], error)
}

// NewStringFactory returns a new StringFactory.
func NewStringFactory[K any](getter ottlruntime.GetFunc[K, string], setter ottlruntime.SetFunc[K, string], opts ...Option[K]) StringFactory[K] {
	f := &factory[K, string]{
		getSetter: ottlruntime.NewGetSetter[K, string](getter, setter),
	}
	for _, opt := range opts {
		opt.apply(f)
	}
	return f
}

// Int64Factory is a Factory that returns a GetSetter that works with int64.
type Int64Factory[K any] interface {
	Factory[K]
	Create(ctx context.Context, set component.TelemetrySettings) (ottlruntime.GetSetter[K, int64], error)
}

// NewInt64Factory returns a new Int64Factory.
func NewInt64Factory[K any](getter ottlruntime.GetFunc[K, int64], setter ottlruntime.SetFunc[K, int64], opts ...Option[K]) Int64Factory[K] {
	f := &factory[K, int64]{
		getSetter: ottlruntime.NewGetSetter[K, int64](getter, setter),
	}
	for _, opt := range opts {
		opt.apply(f)
	}
	return f
}

// Uint64Factory is a Factory that returns a GetSetter that works with uint64.
type Uint64Factory[K any] interface {
	Factory[K]
	Create(ctx context.Context, set component.TelemetrySettings) (ottlruntime.GetSetter[K, uint64], error)
}

// NewUint64Factory returns a new Uint64Factory.
func NewUint64Factory[K any](getter ottlruntime.GetFunc[K, uint64], setter ottlruntime.SetFunc[K, uint64], opts ...Option[K]) Uint64Factory[K] {
	f := &factory[K, uint64]{
		getSetter: ottlruntime.NewGetSetter[K, uint64](getter, setter),
	}
	for _, opt := range opts {
		opt.apply(f)
	}
	return f
}

// BoolFactory is a Factory that returns a GetSetter that works with bool.
type BoolFactory[K any] interface {
	Factory[K]
	Create(ctx context.Context, set component.TelemetrySettings) (ottlruntime.GetSetter[K, bool], error)
}

// NewBoolFactory returns a new BoolFactory.
func NewBoolFactory[K any](getter ottlruntime.GetFunc[K, bool], setter ottlruntime.SetFunc[K, bool], opts ...Option[K]) BoolFactory[K] {
	f := &factory[K, bool]{
		getSetter: ottlruntime.NewGetSetter[K, bool](getter, setter),
	}
	for _, opt := range opts {
		opt.apply(f)
	}
	return f
}

// PMapFactory is a Factory that returns a GetSetter that works with pcommon.Map.
type PMapFactory[K any] interface {
	Factory[K]
	Create(ctx context.Context, set component.TelemetrySettings) (ottlruntime.GetSetter[K, pcommon.Map], error)
}

// NewPMapFactory returns a new PMapFactory.
func NewPMapFactory[K any](getter ottlruntime.GetFunc[K, pcommon.Map], setter ottlruntime.SetFunc[K, pcommon.Map], opts ...Option[K]) PMapFactory[K] {
	f := &factory[K, pcommon.Map]{
		getSetter: ottlruntime.NewGetSetter[K, pcommon.Map](getter, setter),
	}
	for _, opt := range opts {
		opt.apply(f)
	}
	return f
}

// PValueFactory is a Factory that returns a GetSetter that works with pcommon.Value.
type PValueFactory[K any] interface {
	Factory[K]
	Create(ctx context.Context, set component.TelemetrySettings) (ottlruntime.GetSetter[K, pcommon.Value], error)
}

// NewPValueFactory returns a new PValueFactory.
func NewPValueFactory[K any](getter ottlruntime.GetFunc[K, pcommon.Value], setter ottlruntime.SetFunc[K, pcommon.Value], opts ...Option[K]) PValueFactory[K] {
	f := &factory[K, pcommon.Value]{
		getSetter: ottlruntime.NewGetSetter[K, pcommon.Value](getter, setter),
	}
	for _, opt := range opts {
		opt.apply(f)
	}
	return f
}

// AnyFactory is a Factory that returns a GetSetter that works with bool.
type AnyFactory[K any] interface {
	Factory[K]
	Create(ctx context.Context, set component.TelemetrySettings) (ottlruntime.GetSetter[K, any], error)
}

// NewAnyFactory returns a new AnyFactory.
func NewAnyFactory[K any](getter ottlruntime.GetFunc[K, any], setter ottlruntime.SetFunc[K, any], opts ...Option[K]) AnyFactory[K] {
	f := &factory[K, any]{
		getSetter: ottlruntime.NewGetSetter[K, any](getter, setter),
	}
	for _, opt := range opts {
		opt.apply(f)
	}
	return f
}
