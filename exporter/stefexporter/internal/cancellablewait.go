// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter/internal"

import (
	"context"
	"sync"
)

type CancellableCond struct {
	Cond sync.Cond
}

func NewCancellableCond() *CancellableCond {
	c := &CancellableCond{}
	c.Cond = sync.Cond{L: &sync.Mutex{}}
	return c
}

// Wait waits untilCondition() to be true or until ctx is cancelled.
// untilCondition() is always called while the Cond.L lock associated with this
// CancellableCond is locked.
// Implementation borrowed from https://pkg.go.dev/context#AfterFunc
// Returns nil if condition is satisfied or error if ctx is cancelled.
func (c *CancellableCond) Wait(ctx context.Context, untilCondition func() bool) error {
	c.Cond.L.Lock()
	defer c.Cond.L.Unlock()

	stopByCtx := context.AfterFunc(
		ctx, func() {
			// We need to acquire Cond.L here to be sure that the Broadcast
			// below won't occur before the call to Wait, which would result
			// in a missed signal (and deadlock).
			c.Cond.L.Lock()
			defer c.Cond.L.Unlock()

			// If multiple goroutines are waiting on Cond simultaneously,
			// we need to make sure we wake up exactly this one.
			// That means that we need to Broadcast to all of the goroutines,
			// which will wake them all up.
			//
			// If there are N concurrent calls to waitOnCond, each of the goroutines
			// will spuriously wake up O(N) other goroutines that aren't ready yet,
			// so this will cause the overall CPU cost to be O(N²).
			c.Cond.Broadcast()
		},
	)
	defer stopByCtx()

	for !untilCondition() {
		c.Cond.Wait()
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}
