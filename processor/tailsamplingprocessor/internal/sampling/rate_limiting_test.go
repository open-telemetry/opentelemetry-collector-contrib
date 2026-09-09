// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
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

func TestLimitingPolicyImplementationSelection(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()

	t.Run("gate off keeps the token bucket", func(t *testing.T) {
		assert.IsType(t, &rateLimiting{}, NewRateLimitingWithBurstCapacity(settings, 10, 20))
		assert.IsType(t, &bytesLimiting{}, NewBytesLimitingWithBurstCapacity(settings, 10, 20))

		// The token bucket neither reports a threshold nor participates in the
		// processor's batch pre-pass, exactly as today.
		rl := NewRateLimitingWithBurstCapacity(settings, 10, 20)
		_, isThresholdEvaluator := rl.(samplingpolicy.ThresholdEvaluator)
		assert.False(t, isThresholdEvaluator)
		_, isBatchEvaluator := rl.(BatchEvaluator)
		assert.False(t, isBatchEvaluator)
	})

	t.Run("gate on selects the batch limiter", func(t *testing.T) {
		enableTracestateFeatureGate(t)
		assert.IsType(t, &budgetLimiter{}, NewRateLimitingWithBurstCapacity(settings, 10, 20))
		assert.IsType(t, &budgetLimiter{}, NewBytesLimitingWithBurstCapacity(settings, 10, 20))
		assert.IsType(t, &budgetLimiter{}, NewRateLimiting(settings, 10))
		assert.IsType(t, &budgetLimiter{}, NewBytesLimiting(settings, 10))
	})
}

func TestRateLimiterBatchNonUniformSpanCounts(t *testing.T) {
	enableTracestateFeatureGate(t)

	// Budget 3 spans. Sorted by randomness: A(2 spans), B(1 span) fill the
	// budget exactly (3 spans); C(2 spans) overflows and is dropped.
	rl := NewRateLimitingWithBurstCapacity(componenttest.NewNopTelemetrySettings(), 3, 3).(*budgetLimiter)
	a := rlTrace(1, 100, 2)
	b := rlTrace(2, 50, 1)
	c := rlTrace(3, 10, 2)
	rl.CalculateThreshold(t.Context(), []*samplingpolicy.TraceData{a, b, c})

	// a and b are kept; the threshold is the first excluded randomness
	// (c's, 10) plus one.
	wantThreshold, err := sampling.UnsignedToThreshold(11)
	require.NoError(t, err)

	for _, tc := range []struct {
		name string
		td   *samplingpolicy.TraceData
		want samplingpolicy.Decision
	}{
		{"a", a, samplingpolicy.Sampled},
		{"b", b, samplingpolicy.Sampled},
		{"c", c, samplingpolicy.NotSampled},
	} {
		decision, th, err := rl.EvaluateWithThreshold(t.Context(), traceIDOf(tc.td), tc.td)
		require.NoError(t, err)
		assert.Equal(t, tc.want, decision, tc.name)
		if tc.want == samplingpolicy.Sampled {
			assert.Equal(t, wantThreshold, th, tc.name)
		}
	}
}

func TestRateLimiterBatchBoundaryTie(t *testing.T) {
	enableTracestateFeatureGate(t)

	// Budget 2 spans, single-span traces. Sorted: A(100), B(50), C(50),
	// D(10). The budget prefix would keep A and B, but B ties C at the
	// boundary, so the tied group (B, C) is dropped and only A is kept.
	rl := NewRateLimitingWithBurstCapacity(componenttest.NewNopTelemetrySettings(), 2, 2).(*budgetLimiter)
	a := rlTrace(1, 100, 1)
	b := rlTrace(2, 50, 1)
	c := rlTrace(3, 50, 1)
	d := rlTrace(4, 10, 1)
	rl.CalculateThreshold(t.Context(), []*samplingpolicy.TraceData{a, b, c, d})

	// Only a is kept; the threshold is the tied boundary (50) plus one.
	wantThreshold, err := sampling.UnsignedToThreshold(51)
	require.NoError(t, err)

	for _, tc := range []struct {
		name       string
		td         *samplingpolicy.TraceData
		randomness uint64
		want       samplingpolicy.Decision
	}{
		{"a", a, 100, samplingpolicy.Sampled},
		{"b", b, 50, samplingpolicy.NotSampled},
		{"c", c, 50, samplingpolicy.NotSampled},
		{"d", d, 10, samplingpolicy.NotSampled},
	} {
		decision, th, err := rl.EvaluateWithThreshold(t.Context(), traceIDOf(tc.td), tc.td)
		require.NoError(t, err)
		assert.Equal(t, tc.want, decision, tc.name)
		if tc.want == samplingpolicy.Sampled {
			assert.Equal(t, wantThreshold, th, tc.name)
			// Kept traces must pass their own threshold; dropped traces
			// (including the tied ones) must not, so the partition is exact.
			assert.True(t, th.ShouldSample(mustRandomness(t, tc.randomness)))
		} else {
			assert.False(t, wantThreshold.ShouldSample(mustRandomness(t, tc.randomness)), "dropped %s must fail the reported threshold", tc.name)
		}
	}
}

func TestRateLimiterBatchCrossTick(t *testing.T) {
	enableTracestateFeatureGate(t)

	// Rate 1/s (negligible refill during the test), burst 4 => budget 4.
	rl := NewRateLimitingWithBurstCapacity(componenttest.NewNopTelemetrySettings(), 1, 4).(*budgetLimiter)

	// First batch keeps both single-span traces, consuming 2 of 4 tokens.
	first := []*samplingpolicy.TraceData{rlTrace(1, 100, 1), rlTrace(2, 90, 1)}
	rl.CalculateThreshold(t.Context(), first)
	for _, td := range first {
		decision, _, err := rl.EvaluateWithThreshold(t.Context(), traceIDOf(td), td)
		require.NoError(t, err)
		assert.Equal(t, samplingpolicy.Sampled, decision)
	}

	// Second batch sees ~2 remaining tokens, so only the two highest of
	// three single-span traces are kept.
	high := rlTrace(3, 100, 1)
	mid := rlTrace(4, 50, 1)
	low := rlTrace(5, 10, 1)
	rl.CalculateThreshold(t.Context(), []*samplingpolicy.TraceData{high, mid, low})
	assertDecision := func(td *samplingpolicy.TraceData, want samplingpolicy.Decision, msg string) {
		decision, _, err := rl.EvaluateWithThreshold(t.Context(), traceIDOf(td), td)
		require.NoError(t, err)
		assert.Equal(t, want, decision, msg)
	}
	assertDecision(high, samplingpolicy.Sampled, "high")
	assertDecision(mid, samplingpolicy.Sampled, "mid")
	assertDecision(low, samplingpolicy.NotSampled, "budget carried over should drop the lowest-randomness trace")
}

func TestRateLimiterBatchEmpty(t *testing.T) {
	enableTracestateFeatureGate(t)

	rl := NewRateLimitingWithBurstCapacity(componenttest.NewNopTelemetrySettings(), 2, 2).(*budgetLimiter)
	rl.CalculateThreshold(t.Context(), nil)

	td := rlTrace(1, 100, 1)
	decision, th, err := rl.EvaluateWithThreshold(t.Context(), traceIDOf(td), td)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)
	assert.Equal(t, sampling.AlwaysSampleThreshold, th)
}

func TestResolveRandomness(t *testing.T) {
	traceID := pcommon.TraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0xaa, 0, 0, 0, 0, 0, 0})

	// No tracestate: derived from the trace ID.
	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID(traceID)
	assert.Equal(t, sampling.TraceIDToRandomness(traceID), resolveRandomness(traceID, td))

	// Explicit rv in tracestate takes precedence over the trace ID.
	tdRV := ptrace.NewTraces()
	span := tdRV.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.TraceState().FromRaw("ot=rv:ffffffffffffff")
	want, err := sampling.RValueToRandomness("ffffffffffffff")
	require.NoError(t, err)
	assert.Equal(t, want, resolveRandomness(traceID, tdRV))
}

func mustRandomness(t *testing.T, u uint64) sampling.Randomness {
	t.Helper()
	r, err := sampling.UnsignedToRandomness(u)
	require.NoError(t, err)
	return r
}

// rlTrace builds a single-span trace whose trace ID encodes the given
// randomness in its low 7 bytes (the bytes TraceIDToRandomness reads) and a
// unique tag in its first byte, so traces with equal randomness still have
// distinct IDs.
func rlTrace(tag byte, randomness uint64, spanCount int64) *samplingpolicy.TraceData {
	var id [16]byte
	id[0] = tag
	binary.BigEndian.PutUint64(id[8:], randomness)
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID(id))
	return &samplingpolicy.TraceData{SpanCount: spanCount, ReceivedBatches: traces}
}
