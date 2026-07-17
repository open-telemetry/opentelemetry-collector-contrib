// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"

	pkgsampling "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
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

func TestRateLimitingImplementsThresholdEvaluator(t *testing.T) {
	rateLimiter := NewRateLimiting(componenttest.NewNopTelemetrySettings(), 3)
	require.Implements(t, (*samplingpolicy.ThresholdEvaluator)(nil), rateLimiter)
}

func TestRateLimitingThresholdAlwaysSampleWhenNotLimiting(t *testing.T) {
	gate := metadata.ProcessorTailsamplingprocessorUsetracestateFeatureGate
	prev := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), true))
	defer func() {
		_ = featuregate.GlobalRegistry().Set(gate.ID(), prev)
	}()

	trace := newTraceStringAttrs(nil, "example", "value")
	trace.SpanCount = 1
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	
	rateLimiter := NewRateLimiting(componenttest.NewNopTelemetrySettings(), 1000)
	te := rateLimiter.(samplingpolicy.ThresholdEvaluator)

	// First evaluation initializes EMA
	decision, th, err := te.EvaluateWithThreshold(t.Context(), traceID, trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)
	assert.Equal(t, pkgsampling.AlwaysSampleThreshold, th)

	// Wait a bit, evaluate again with span count small enough to be under the limit
	time.Sleep(10 * time.Millisecond)
	decision, th, err = te.EvaluateWithThreshold(t.Context(), traceID, trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)
	assert.Equal(t, pkgsampling.AlwaysSampleThreshold, th)
}

func TestRateLimitingThresholdReflectsRate(t *testing.T) {
	gate := metadata.ProcessorTailsamplingprocessorUsetracestateFeatureGate
	prev := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), true))
	defer func() {
		_ = featuregate.GlobalRegistry().Set(gate.ID(), prev)
	}()

	trace := newTraceStringAttrs(nil, "example", "value")
	traceID := pcommon.TraceID([16]byte{1})
	
	// Spans per sec = 1, burst = 100
	rateLimiter := NewRateLimitingWithBurstCapacity(componenttest.NewNopTelemetrySettings(), 1, 100)
	te := rateLimiter.(samplingpolicy.ThresholdEvaluator)

	// First eval to set lastTime
	trace.SpanCount = 1
	_, _, err := te.EvaluateWithThreshold(t.Context(), traceID, trace)
	require.NoError(t, err)

	// Sleep 100ms, then hit it with 10 spans. Instant rate = 100 spans/sec.
	time.Sleep(100 * time.Millisecond)
	trace.SpanCount = 10
	
	decision, th, err := te.EvaluateWithThreshold(t.Context(), traceID, trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision) // We have a burst of 100, so it's allowed
	
	// The EMA should be updated and > 1.0 (the limit).
	// So the threshold should be greater than AlwaysSampleThreshold.
	assert.True(t, pkgsampling.ThresholdGreater(th, pkgsampling.AlwaysSampleThreshold))
}

func TestRateLimitingThresholdWithoutFeatureGate(t *testing.T) {
	gate := metadata.ProcessorTailsamplingprocessorUsetracestateFeatureGate
	prev := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), false))
	defer func() {
		_ = featuregate.GlobalRegistry().Set(gate.ID(), prev)
	}()

	trace := newTraceStringAttrs(nil, "example", "value")
	traceID := pcommon.TraceID([16]byte{1})
	
	rateLimiter := NewRateLimitingWithBurstCapacity(componenttest.NewNopTelemetrySettings(), 1, 100)
	te := rateLimiter.(samplingpolicy.ThresholdEvaluator)

	trace.SpanCount = 1
	_, _, _ = te.EvaluateWithThreshold(t.Context(), traceID, trace)
	
	time.Sleep(100 * time.Millisecond)
	trace.SpanCount = 10
	decision, th, err := te.EvaluateWithThreshold(t.Context(), traceID, trace)
	
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)
	assert.Equal(t, pkgsampling.AlwaysSampleThreshold, th)
}
