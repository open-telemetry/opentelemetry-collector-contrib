// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"

import (
	"context"

	"go.opentelemetry.io/collector/component"
)

// LookupFunc performs a lookup for the given key.
// Returns (value, found, error):
//   - If found=true, value contains the lookup result
//   - If found=false, the key was not found (not an error)
//   - If error!=nil, the lookup failed
type LookupFunc func(ctx context.Context, key string) (any, bool, error)

type TypeFunc func() string

type StartFunc func(ctx context.Context, host component.Host) error

type ShutdownFunc func(ctx context.Context) error

// Source is the interface for lookup sources.
//
// Use [NewSource] to create implementations.
type Source interface {
	Lookup(ctx context.Context, key string) (any, bool, error)
	Type() string
	Start(ctx context.Context, host component.Host) error
	Shutdown(ctx context.Context) error
}

// NewSource creates a Source from functional components.
//
// Parameters:
//   - lookup: Required. The function that performs lookups.
//   - typeFunc: Required. Returns the source type identifier.
//   - start: Optional. Called when the processor starts. Can be nil.
//   - shutdown: Optional. Called when the processor shuts down. Can be nil.
//
// Example:
//
//	source := lookupsource.NewSource(
//	    func(ctx context.Context, key string) (any, bool, error) {
//	        // lookup logic
//	        return value, true, nil
//	    },
//	    func() string { return "mysource" },
//	    nil, // no start needed
//	    nil, // no shutdown needed
//	)
func NewSource(
	lookup LookupFunc,
	typeFunc TypeFunc,
	start StartFunc,
	shutdown ShutdownFunc,
) Source {
	return &sourceImpl{
		lookupFn:   lookup,
		typeFn:     typeFunc,
		startFn:    start,
		shutdownFn: shutdown,
	}
}

type sourceImpl struct {
	lookupFn   LookupFunc
	typeFn     TypeFunc
	startFn    StartFunc
	shutdownFn ShutdownFunc
}

func (s *sourceImpl) Lookup(ctx context.Context, key string) (any, bool, error) {
	if s.lookupFn == nil {
		return nil, false, nil
	}
	return s.lookupFn(ctx, key)
}

func (s *sourceImpl) Type() string {
	if s.typeFn == nil {
		return "unknown"
	}
	return s.typeFn()
}

func (s *sourceImpl) Start(ctx context.Context, host component.Host) error {
	if s.startFn == nil {
		return nil
	}
	return s.startFn(ctx, host)
}

func (s *sourceImpl) Shutdown(ctx context.Context) error {
	if s.shutdownFn == nil {
		return nil
	}
	return s.shutdownFn(ctx)
}
