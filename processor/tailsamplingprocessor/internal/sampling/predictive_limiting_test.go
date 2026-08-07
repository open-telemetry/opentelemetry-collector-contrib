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

// fakeClock is a manually advanced clock so window rolls are deterministic.
type fakeClock struct {
	t time.Time
}

func (c *fakeClock) Now() time.Time {
	return c.t
}

// advanceWindow moves past the current accumulation window, so the next
// observation rolls it and recomputes the keep fraction.
func (c *fakeClock) advanceWindow() {
	c.t = c.t.Add(defaultRateWindow)
}

// plTrace builds a single-span trace whose trace ID encodes the given
// randomness in its low 8 bytes (the bytes TraceIDToRandomness reads) and a
// unique tag in its first byte, so traces with equal randomness still have
// distinct IDs.
func plTrace(tag byte, randomness uint64, spanCount int64) (pcommon.TraceID, *samplingpolicy.TraceData) {
	var id [16]byte
	id[0] = tag
	binary.BigEndian.PutUint64(id[8:], randomness)
	traceID := pcommon.TraceID(id)
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	return traceID, &samplingpolicy.TraceData{SpanCount: spanCount, ReceivedBatches: traces}
}

// spreadRandomness returns the i-th of n randomness values spread evenly
// across the 56-bit randomness space, so a threshold's probability can be
// checked against the fraction of values it admits.
func spreadRandomness(i, n int) uint64 {
	return uint64(i) * (sampling.MaxAdjustedCount / uint64(n))
}

// newPredictiveRateLimiter builds a rate_limiting policy with the gate on, so
// the constructor selects the predictive implementation.
func newPredictiveRateLimiter(t *testing.T, spansPerSecond int64, clock *fakeClock) *predictiveLimiter {
	t.Helper()
	rl, ok := NewRateLimitingWithBurstCapacity(componenttest.NewNopTelemetrySettings(), spansPerSecond, spansPerSecond*2).(*predictiveLimiter)
	require.True(t, ok, "expected the predictive implementation with the tracestate gate enabled")
	rl.now = clock.Now
	return rl
}

// TestFractionForRate covers the conversion from an observed volume to the
// share of arriving traces that holds throughput at the configured rate.
func TestFractionForRate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		limit    float64
		observed float64
		elapsed  time.Duration
		want     float64
	}{
		{name: "under limit", limit: 100, observed: 50, elapsed: time.Second, want: 1},
		{name: "at limit", limit: 100, observed: 100, elapsed: time.Second, want: 1},
		{name: "nothing observed", limit: 100, observed: 0, elapsed: time.Second, want: 1},
		{name: "no elapsed time", limit: 100, observed: 500, elapsed: 0, want: 1},
		{name: "zero limit keeps nothing", limit: 0, observed: 500, elapsed: time.Second, want: 0},
		{name: "negative limit keeps nothing", limit: -1, observed: 500, elapsed: time.Second, want: 0},
		{name: "twice the limit", limit: 100, observed: 200, elapsed: time.Second, want: 0.5},
		{name: "ten times the limit", limit: 100, observed: 1000, elapsed: time.Second, want: 0.1},
		// Elapsed time is honored, not assumed to be one second: 400 tokens
		// over 2s is 200/s, so the same 0.5 fraction as above.
		{name: "rate uses elapsed time", limit: 100, observed: 400, elapsed: 2 * time.Second, want: 0.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.InDelta(t, tc.want, fractionForRate(tc.limit, tc.observed, tc.elapsed), 0.001)
		})
	}
}

// TestComposeThreshold covers composing the relative keep fraction with a
// trace's incoming sampling probability. Without composition, a fraction
// looser than the incoming probability would admit every arriving trace and
// limit nothing.
func TestComposeThreshold(t *testing.T) {
	incoming := func(t *testing.T, prob float64) tracestateScan {
		t.Helper()
		th, err := sampling.ProbabilityToThreshold(prob)
		require.NoError(t, err)
		return tracestateScan{threshold: th, hasThreshold: true, hasSamplingInfo: true}
	}

	t.Run("no incoming threshold uses the fraction directly", func(t *testing.T) {
		got := composeThreshold(tracestateScan{}, 0.1)
		assert.InDelta(t, 0.1, got.Probability(), 0.01)
	})

	t.Run("composes with incoming probability", func(t *testing.T) {
		// Head sampled at 1%, limiter wants to keep a tenth of arrivals, so
		// the absolute probability is 0.1%.
		got := composeThreshold(incoming(t, 0.01), 0.1)
		assert.InDelta(t, 0.001, got.Probability(), 0.0001)
	})

	t.Run("composed threshold is stricter than the incoming one", func(t *testing.T) {
		scan := incoming(t, 0.01)
		got := composeThreshold(scan, 0.5)
		// A stricter threshold is a larger unsigned value, so it rejects some
		// of the traces the incoming threshold admitted.
		assert.Greater(t, got.Unsigned(), scan.threshold.Unsigned())
	})

	t.Run("fraction of zero keeps nothing", func(t *testing.T) {
		assert.Equal(t, sampling.NeverSampleThreshold, composeThreshold(tracestateScan{}, 0))
	})
}

// TestPredictiveLimiterLimitsHeadSampledTraffic is the case that composition
// exists for. Traces arriving already sampled at 1% have randomness confined
// to the top 1% of the range, so a threshold built from the keep fraction
// alone would admit all of them and the limiter would not limit at all.
func TestPredictiveLimiterLimitsHeadSampledTraffic(t *testing.T) {
	enableTracestateFeatureGate(t)
	clock := &fakeClock{t: time.Unix(1700000000, 0)}
	rl := newPredictiveRateLimiter(t, 100, clock)

	// Settle on keeping a tenth of arrivals.
	rl.observe(1000)
	clock.advanceWindow()

	headThreshold, err := sampling.ProbabilityToThreshold(0.01)
	require.NoError(t, err)

	// Randomness values spread across only the slice a 1% head sampler would
	// have let through, which is where real head-sampled traffic lands.
	const total = 1000
	lowest := headThreshold.Unsigned()
	span := sampling.MaxAdjustedCount - lowest

	var kept int
	for i := range total {
		rnd := lowest + uint64(i)*(span/total)
		id, td := plTrace(byte(i%256), rnd, 1)
		td.ReceivedBatches.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).
			TraceState().FromRaw("ot=th:" + headThreshold.TValue())

		decision, _, err := rl.EvaluateWithThreshold(t.Context(), id, td)
		require.NoError(t, err)
		if decision == samplingpolicy.Sampled {
			kept++
		}
	}

	// About a tenth of the arriving traces survive, not all of them.
	assert.InDelta(t, total/10, kept, float64(total)/20)
}

// TestPredictiveLimiterFirstWindowDoesNotLimit verifies that before any
// window has closed the policy reports always-sample and keeps everything,
// so a freshly started collector does not drop traffic it has not measured.
func TestPredictiveLimiterFirstWindowDoesNotLimit(t *testing.T) {
	enableTracestateFeatureGate(t)
	clock := &fakeClock{t: time.Unix(1700000000, 0)}
	rl := newPredictiveRateLimiter(t, 10, clock)

	// Far more spans than the limit, all within the first window.
	for i := range 100 {
		id, td := plTrace(byte(i), spreadRandomness(i, 100), 1)
		decision, th, err := rl.EvaluateWithThreshold(t.Context(), id, td)
		require.NoError(t, err)
		assert.Equal(t, samplingpolicy.Sampled, decision)
		assert.Equal(t, sampling.AlwaysSampleThreshold, th)
	}
}

// TestPredictiveLimiterThresholdTracksObservedRate verifies the threshold
// derived at a window roll reflects the traffic observed in the window that
// just closed, and that it keeps adapting as the rate changes.
func TestPredictiveLimiterThresholdTracksObservedRate(t *testing.T) {
	enableTracestateFeatureGate(t)
	clock := &fakeClock{t: time.Unix(1700000000, 0)}
	rl := newPredictiveRateLimiter(t, 100, clock)

	// Window 1: 1000 spans in one second. Nothing is limited yet.
	assert.Equal(t, 1.0, rl.observe(1000))

	// Rolling into window 2 prices the observed 1000/s against the limit of
	// 100/s, giving probability 0.1.
	clock.advanceWindow()
	assert.InDelta(t, 0.1, rl.observe(250), 0.01)

	// Window 2 saw only 250 spans, so window 3 relaxes to 100/250 = 0.4.
	clock.advanceWindow()
	assert.InDelta(t, 0.4, rl.observe(50), 0.01)

	// Window 3 came in under the limit, so window 4 stops limiting.
	clock.advanceWindow()
	assert.Equal(t, 1.0, rl.observe(0))
}

// TestPredictiveLimiterReportsAppliedThreshold verifies the invariant that
// makes adjusted counts correct: the threshold reported for a kept trace is
// the threshold that admitted it, and traces the threshold rejects are not
// kept. This is checked per trace rather than in aggregate.
func TestPredictiveLimiterReportsAppliedThreshold(t *testing.T) {
	enableTracestateFeatureGate(t)
	clock := &fakeClock{t: time.Unix(1700000000, 0)}
	rl := newPredictiveRateLimiter(t, 100, clock)

	// Close a window at 1000 spans/s so the limiter settles on p=0.1.
	rl.observe(1000)
	clock.advanceWindow()

	const total = 1000
	var kept int
	for i := range total {
		rnd := spreadRandomness(i, total)
		id, td := plTrace(byte(i%256), rnd, 1)
		decision, th, err := rl.EvaluateWithThreshold(t.Context(), id, td)
		require.NoError(t, err)

		randomness, err := sampling.UnsignedToRandomness(rnd)
		require.NoError(t, err)

		if decision == samplingpolicy.Sampled {
			kept++
			// The reported threshold must admit this trace's randomness.
			assert.True(t, th.ShouldSample(randomness), "trace %d kept but its reported threshold rejects it", i)
			assert.InDelta(t, 0.1, th.Probability(), 0.01)
		}
	}
	require.Positive(t, kept)
}

// TestPredictiveLimiterAdjustedCountsAreCorrect is the property the change
// exists for: summing the adjusted counts of the kept traces reconstructs the
// offered population, which is what under-reporting a threshold breaks today.
func TestPredictiveLimiterAdjustedCountsAreCorrect(t *testing.T) {
	enableTracestateFeatureGate(t)
	clock := &fakeClock{t: time.Unix(1700000000, 0)}
	rl := newPredictiveRateLimiter(t, 100, clock)

	// Settle on p=0.1 by closing a window at 1000 spans/s.
	rl.observe(1000)
	clock.advanceWindow()

	const total = 1000
	var adjusted float64
	var kept int
	for i := range total {
		id, td := plTrace(byte(i%256), spreadRandomness(i, total), 1)
		decision, th, err := rl.EvaluateWithThreshold(t.Context(), id, td)
		require.NoError(t, err)
		if decision == samplingpolicy.Sampled {
			kept++
			adjusted += th.AdjustedCount()
		}
	}

	// Roughly the configured rate is kept, and the adjusted counts add back
	// up to everything that was offered.
	assert.InDelta(t, 100, kept, 20)
	assert.InDelta(t, total, adjusted, float64(total)*0.05)
}

// TestLimitingPolicyImplementationSelection verifies the constructors pick the
// implementation by feature gate: the original token bucket with the gate off,
// the predictive limiter with it on. With the gate off no predictive code runs
// at all, so existing deployments keep the token bucket behavior covered by
// TestRateLimiterTokenBucket and TestBytesLimitingTokenBucket.
func TestLimitingPolicyImplementationSelection(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()

	t.Run("gate off keeps the token bucket", func(t *testing.T) {
		assert.IsType(t, &rateLimiting{}, NewRateLimitingWithBurstCapacity(settings, 10, 20))
		assert.IsType(t, &bytesLimiting{}, NewBytesLimitingWithBurstCapacity(settings, 10, 20))

		// The token bucket does not report a threshold, so the processor wraps
		// it as always-sample just as it does today.
		_, isThresholdEvaluator := NewRateLimitingWithBurstCapacity(settings, 10, 20).(samplingpolicy.ThresholdEvaluator)
		assert.False(t, isThresholdEvaluator)
	})

	t.Run("gate on selects the predictive limiter", func(t *testing.T) {
		enableTracestateFeatureGate(t)
		assert.IsType(t, &predictiveLimiter{}, NewRateLimitingWithBurstCapacity(settings, 10, 20))
		assert.IsType(t, &predictiveLimiter{}, NewBytesLimitingWithBurstCapacity(settings, 10, 20))
		assert.IsType(t, &predictiveLimiter{}, NewRateLimiting(settings, 10))
		assert.IsType(t, &predictiveLimiter{}, NewBytesLimiting(settings, 10))
	})
}

// TestPredictiveLimiterExplicitRandomnessWins verifies the decision uses an
// explicit `rv` from tracestate when present rather than the trace ID, so the
// limiter agrees with upstream samplers about a trace's randomness.
func TestPredictiveLimiterExplicitRandomnessWins(t *testing.T) {
	enableTracestateFeatureGate(t)
	clock := &fakeClock{t: time.Unix(1700000000, 0)}
	rl := newPredictiveRateLimiter(t, 100, clock)

	rl.observe(1000)
	clock.advanceWindow() // p=0.1, so th admits only high randomness

	// Trace ID randomness is low (would be rejected), but the explicit rv is
	// at the top of the range (must be kept).
	id, td := plTrace(1, 0, 1)
	span := td.ReceivedBatches.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	span.TraceState().FromRaw("ot=rv:ffffffffffffff")

	decision, th, err := rl.EvaluateWithThreshold(t.Context(), id, td)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)
	assert.InDelta(t, 0.1, th.Probability(), 0.01)
}

// TestPredictiveLimiterSpanCountWeighting verifies the budget is spent in
// spans rather than traces: a population of multi-span traces settles on a
// proportionally smaller probability.
func TestPredictiveLimiterSpanCountWeighting(t *testing.T) {
	enableTracestateFeatureGate(t)
	clock := &fakeClock{t: time.Unix(1700000000, 0)}
	rl := newPredictiveRateLimiter(t, 100, clock)

	// 100 traces of 10 spans each is 1000 spans, not 100.
	for i := range 100 {
		id, td := plTrace(byte(i), spreadRandomness(i, 100), 10)
		_, _, err := rl.EvaluateWithThreshold(t.Context(), id, td)
		require.NoError(t, err)
	}

	clock.advanceWindow()
	assert.InDelta(t, 0.1, rl.observe(0), 0.01)
}

// TestBytesLimitingPredictiveThreshold verifies the bytes policy budgets in
// marshaled bytes on the predictive path.
func TestBytesLimitingPredictiveThreshold(t *testing.T) {
	enableTracestateFeatureGate(t)
	clock := &fakeClock{t: time.Unix(1700000000, 0)}

	_, td := plTrace(1, spreadRandomness(99, 100), 1)
	size := calculateTraceSize(td)

	// Limit of one trace-size per second against ten arriving per second.
	bl, ok := NewBytesLimitingWithBurstCapacity(componenttest.NewNopTelemetrySettings(), size, size*2).(*predictiveLimiter)
	require.True(t, ok, "expected the predictive implementation with the tracestate gate enabled")
	bl.now = clock.Now

	for i := range 10 {
		traceID, trace := plTrace(byte(i), spreadRandomness(i, 10), 1)
		_, _, err := bl.EvaluateWithThreshold(t.Context(), traceID, trace)
		require.NoError(t, err)
	}

	clock.advanceWindow()
	assert.InDelta(t, 0.1, bl.observe(0), 0.01)
}
