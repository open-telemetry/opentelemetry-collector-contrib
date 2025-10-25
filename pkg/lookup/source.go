// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookup // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/lookup"

import (
	"context"

	"go.opentelemetry.io/collector/component"
)

// LookupFunc contains the functional implementation of Source.Lookup.
type LookupFunc func(context.Context, string) (any, bool, error)

// Lookup executes the functional implementation if it is non-nil.
func (f LookupFunc) Lookup(ctx context.Context, key string) (any, bool, error) {
	if f == nil {
		return nil, false, nil
	}
	return f(ctx, key)
}

// TypeFunc contains the functional implementation of Source.Type.
type TypeFunc func() string

// Type executes the functional implementation if it is non-nil.
func (f TypeFunc) Type() string {
	if f == nil {
		return ""
	}
	return f()
}

// StartFunc contains the functional implementation of Source.Start.
type StartFunc func(context.Context, component.Host) error

// Start executes the functional implementation if it is non-nil.
func (f StartFunc) Start(ctx context.Context, host component.Host) error {
	if f == nil {
		return nil
	}
	return f(ctx, host)
}

// ShutdownFunc contains the functional implementation of Source.Shutdown.
type ShutdownFunc func(context.Context) error

// Shutdown executes the functional implementation if it is non-nil.
func (f ShutdownFunc) Shutdown(ctx context.Context) error {
	if f == nil {
		return nil
	}
	return f(ctx)
}

// Source represents a lookup provider used by the lookup processor.
type Source interface {
	Lookup(context.Context, string) (any, bool, error)
	Type() string
	Start(context.Context, component.Host) error
	Shutdown(context.Context) error
	private()
}

var _ Source = (*sourceImpl)(nil)

// sourceImpl composes the functional implementations into a sealed Source.
type sourceImpl struct {
	LookupFunc
	TypeFunc
	StartFunc
	ShutdownFunc
}

func (sourceImpl) private() {}

// SourceOption allows customization of Source implementations.
type SourceOption interface {
	applySource(*sourceImpl)
}

// NewSource constructs a Source from the provided functional pieces.
// A nil function implements the RFC-mandated no-op behavior.
func NewSource(lookup LookupFunc, t TypeFunc, start StartFunc, shutdown ShutdownFunc, opts ...SourceOption) Source {
	impl := sourceImpl{
		LookupFunc:   lookup,
		TypeFunc:     t,
		StartFunc:    start,
		ShutdownFunc: shutdown,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.applySource(&impl)
	}

	return impl
}

// LookupExtension represents a collector extension that also satisfies Source.
type LookupExtension interface {
	Source
	component.Component
	privateExtension()
}

var _ LookupExtension = (*lookupExtensionImpl)(nil)

// ComponentStartFunc contains the functional implementation of component.Component.Start.
type ComponentStartFunc StartFunc

// Start executes the functional implementation if it is non-nil.
func (f ComponentStartFunc) Start(ctx context.Context, host component.Host) error {
	return StartFunc(f).Start(ctx, host)
}

// ComponentShutdownFunc contains the functional implementation of component.Component.Shutdown.
type ComponentShutdownFunc ShutdownFunc

// Shutdown executes the functional implementation if it is non-nil.
func (f ComponentShutdownFunc) Shutdown(ctx context.Context) error {
	return ShutdownFunc(f).Shutdown(ctx)
}

// lookupExtensionImpl composes Source and Component behavior into a sealed LookupExtension.
type lookupExtensionImpl struct {
	sourceImpl
}

func (lookupExtensionImpl) privateExtension() {}

// LookupExtensionOption allows customization of LookupExtension implementations.
type LookupExtensionOption interface {
	applyLookupExtension(*lookupExtensionImpl)
}

type lookupExtensionOptionFunc func(*lookupExtensionImpl)

func (f lookupExtensionOptionFunc) applyLookupExtension(impl *lookupExtensionImpl) {
	if f == nil {
		return
	}
	f(impl)
}

// WithSourceOptions adapts SourceOption values so they can be applied to LookupExtensions.
func WithSourceOptions(opts ...SourceOption) LookupExtensionOption {
	return lookupExtensionOptionFunc(func(impl *lookupExtensionImpl) {
		for _, opt := range opts {
			if opt == nil {
				continue
			}
			opt.applySource(&impl.sourceImpl)
		}
	})
}

// NewLookupExtension constructs a LookupExtension using the provided functional pieces.
func NewLookupExtension(lookup LookupFunc, t TypeFunc, start StartFunc, shutdown ShutdownFunc, opts ...LookupExtensionOption) LookupExtension {
	impl := lookupExtensionImpl{
		sourceImpl: sourceImpl{
			LookupFunc:   lookup,
			TypeFunc:     t,
			StartFunc:    start,
			ShutdownFunc: shutdown,
		},
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.applyLookupExtension(&impl)
	}

	return impl
}
