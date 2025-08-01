// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracelimiter // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/tracelimiter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type DropOldTracesLimiter struct {
	deleteChan chan pcommon.TraceID
	dropTrace  func(id pcommon.TraceID, time time.Time)
}

func NewDropOldTracesLimiter(numTraces uint64, dropTrace func(id pcommon.TraceID, time time.Time)) *DropOldTracesLimiter {
	return &DropOldTracesLimiter{
		deleteChan: make(chan pcommon.TraceID, numTraces),
		dropTrace:  dropTrace,
	}
}

func (l *DropOldTracesLimiter) AcceptTrace(ctx context.Context, id pcommon.TraceID, time time.Time) {
	postDeletion := false
	for !postDeletion {
		select {
		case <-ctx.Done():
			return
		case l.deleteChan <- id:
			postDeletion = true
		default:
			toDelete := <-l.deleteChan
			l.dropTrace(toDelete, time)
		}
	}
}

func (*DropOldTracesLimiter) OnDeleteTrace() {
	// Nothing to do here, in this limiter, traces are deleted automatically
	// when the deleteChan is full
}
