// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package requestcounter

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
