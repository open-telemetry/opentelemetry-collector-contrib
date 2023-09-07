// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestRateLimiter(t *testing.T) {
	trace := newTraceStringAttrs(nil, "example", "value")
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rateLimiter := NewRateLimiting(componenttest.NewNopTelemetrySettings(), 3)

	// Trace span count greater than spans per second
	traceSpanCount := &atomic.Int64{}
	traceSpanCount.Store(10)
	trace.SpanCount = traceSpanCount
	decision, err := rateLimiter.Evaluate(context.Background(), traceID, trace)
	assert.Nil(t, err)
	assert.Equal(t, decision, NotSampled)

	// Trace span count equal to spans per second
	traceSpanCount = &atomic.Int64{}
	traceSpanCount.Store(3)
	trace.SpanCount = traceSpanCount
	decision, err = rateLimiter.Evaluate(context.Background(), traceID, trace)
	assert.Nil(t, err)
	assert.Equal(t, decision, NotSampled)

	// Trace span count less than spans per second
	traceSpanCount = &atomic.Int64{}
	traceSpanCount.Store(2)
	trace.SpanCount = traceSpanCount
	decision, err = rateLimiter.Evaluate(context.Background(), traceID, trace)
	assert.Nil(t, err)
	assert.Equal(t, decision, Sampled)

	// Trace span count less than spans per second
	traceSpanCount = &atomic.Int64{}
	trace.SpanCount = traceSpanCount
	decision, err = rateLimiter.Evaluate(context.Background(), traceID, trace)
	assert.Nil(t, err)
	assert.Equal(t, decision, Sampled)
}
