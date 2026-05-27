// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestRateLimiterTokenBucket(t *testing.T) {
	trace := newTraceStringAttrs(nil, "example", "value")
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rateLimiter := NewRateLimitingWithBurstCapacity(componenttest.NewNopTelemetrySettings(), 3, 3)

	// First trace consumes the entire burst.
	trace.SpanCount = 3
	decision, err := rateLimiter.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)

	// No tokens remain in the same window.
	trace.SpanCount = 1
	decision, err = rateLimiter.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)
}

func TestRateLimiterBurstCapacityRejectsLargeTrace(t *testing.T) {
	trace := newTraceStringAttrs(nil, "example", "value")
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rateLimiter := NewRateLimitingWithBurstCapacity(componenttest.NewNopTelemetrySettings(), 3, 2)

	// A single trace exceeding burst capacity is rejected.
	trace.SpanCount = 3
	decision, err := rateLimiter.Evaluate(t.Context(), traceID, trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)
}

func TestRateLimiterDefaultConstructorBurst(t *testing.T) {
	trace := newTraceStringAttrs(nil, "example", "value")
	rateLimiter := NewRateLimiting(componenttest.NewNopTelemetrySettings(), 2)

	// Default burst is 2x spans_per_second, so the first 4 single-span traces are sampled.
	trace.SpanCount = 1
	for i := range 4 {
		decision, err := rateLimiter.Evaluate(t.Context(), pcommon.TraceID([16]byte{byte(i + 1)}), trace)
		require.NoError(t, err)
		assert.Equal(t, samplingpolicy.Sampled, decision)
	}

	decision, err := rateLimiter.Evaluate(t.Context(), pcommon.TraceID([16]byte{5}), trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)
}

func TestRateLimiterTokenRefill(t *testing.T) {
	trace := newTraceStringAttrs(nil, "example", "value")
	rateLimiter := NewRateLimitingWithBurstCapacity(componenttest.NewNopTelemetrySettings(), 4, 2)

	trace.SpanCount = 2
	decision, err := rateLimiter.Evaluate(t.Context(), pcommon.TraceID([16]byte{1}), trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)

	decision, err = rateLimiter.Evaluate(t.Context(), pcommon.TraceID([16]byte{2}), trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)

	// Refill happens at 4 tokens per second; wait long enough for the bucket to refill.
	time.Sleep(600 * time.Millisecond)

	decision, err = rateLimiter.Evaluate(t.Context(), pcommon.TraceID([16]byte{3}), trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)
}
