// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestRateLimiter(t *testing.T) {
	trace := newTraceStringAttrs(nil, "example", "value")
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rateLimiter := NewRateLimiting(componenttest.NewNopTelemetrySettings(), 3)

	// Trace span count greater than spans per second
	traceSpanCount := &atomic.Int64{}
	traceSpanCount.Store(10)
	trace.SpanCount = traceSpanCount
	decision, err := rateLimiter.Evaluate(t.Context(), traceID, trace)
	assert.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)

	// Trace span count equal to spans per second
	traceSpanCount = &atomic.Int64{}
	traceSpanCount.Store(3)
	trace.SpanCount = traceSpanCount
	decision, err = rateLimiter.Evaluate(t.Context(), traceID, trace)
	assert.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)

	// Trace span count less than spans per second
	traceSpanCount = &atomic.Int64{}
	traceSpanCount.Store(2)
	trace.SpanCount = traceSpanCount
	decision, err = rateLimiter.Evaluate(t.Context(), traceID, trace)
	assert.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)

	// Trace span count less than spans per second
	traceSpanCount = &atomic.Int64{}
	trace.SpanCount = traceSpanCount
	decision, err = rateLimiter.Evaluate(t.Context(), traceID, trace)
	assert.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)
}
