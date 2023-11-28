// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/requests/requestcounter/counter.go

package requestcounter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/requests/requestcounter"

import (
	"context"
	"sync/atomic"
)

type key int

const (
	getRequestCountKey       key = 1
	incrementRequestCountKey key = 2
	resetRequestCountKey     key = 3
)

type getRequestCount func() uint32
type incrementRequestCount func()
type resetRequestCount func()

// checks if a counter already exists on the context
func counterExists(ctx context.Context) (exists bool) {
	if _, exists = ctx.Value(getRequestCountKey).(getRequestCount); !exists {
		if _, exists = ctx.Value(incrementRequestCountKey).(incrementRequestCount); !exists {
			_, exists = ctx.Value(resetRequestCountKey).(resetRequestCount)
		}
	}
	return exists
}

// ContextWithRequestCounter adds a counter to the context if one does not already exist
func ContextWithRequestCounter(ctx context.Context) context.Context {
	if counterExists(ctx) {
		// don't create a new context with counter if a counter already exists on the counter
		return ctx
	}
	var c uint32
	newCtx := context.WithValue(ctx, getRequestCountKey, getRequestCount(func() uint32 { return atomic.LoadUint32(&c) }))
	newCtx = context.WithValue(newCtx, incrementRequestCountKey, incrementRequestCount(func() { atomic.AddUint32(&c, 1) }))
	return context.WithValue(newCtx, resetRequestCountKey, resetRequestCount(func() { atomic.StoreUint32(&c, 0) }))
}

// ResetRequestCount resets the request counter on the provided context if the context has one
func ResetRequestCount(ctx context.Context) {
	if res, ok := ctx.Value(resetRequestCountKey).(resetRequestCount); ok {
		res()
	}
}

// IncrementRequestCount increments the request counter on the provided context if the context has one
func IncrementRequestCount(ctx context.Context) {
	if inc, ok := ctx.Value(incrementRequestCountKey).(incrementRequestCount); ok {
		inc()
	}
}

// GetRequestCount retrieves the current request count on the provided context.  It returns 0 if the context does not
// have a request counter
func GetRequestCount(ctx context.Context) (count uint32) {
	if get, ok := ctx.Value(getRequestCountKey).(getRequestCount); ok {
		count = get()
	}
	return count
}
