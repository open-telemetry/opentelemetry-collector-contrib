// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracelimiter // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/tracelimiter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// BlockingTracesLimiter is a limiter that blocks when the number of traces exceeds the limit.
// It is used to limit the number of traces that are processed by the processor.
type BlockingTracesLimiter struct {
	semaphore chan struct{}
}

func NewBlockingTracesLimiter(numTraces uint64) *BlockingTracesLimiter {
	return &BlockingTracesLimiter{
		semaphore: make(chan struct{}, numTraces),
	}
}

func (l *BlockingTracesLimiter) AcceptTrace(ctx context.Context, _ pcommon.TraceID, _ time.Time) {
	select {
	case <-ctx.Done():
		return
	case l.semaphore <- struct{}{}:
	}
}

func (l *BlockingTracesLimiter) OnDeleteTrace() {
	<-l.semaphore
}
