// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"encoding/binary"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestProbabilisticSampling(t *testing.T) {
	tests := []struct {
		name                       string
		samplingPercentage         float64
		hashSalt                   string
		expectedSamplingPercentage float64
	}{
		{
			"100%",
			100,
			"",
			100,
		},
		{
			"0%",
			0,
			"",
			0,
		},
		{
			"25%",
			25,
			"",
			25,
		},
		{
			"33%",
			33,
			"",
			33,
		},
		{
			"33% - custom salt",
			33,
			"test-salt",
			33,
		},
		{
			"-%50",
			-50,
			"",
			0,
		},
		{
			"150%",
			150,
			"",
			100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traceCount := 100_000

			probabilisticSampler := NewProbabilisticSampler(componenttest.NewNopTelemetrySettings(), tt.hashSalt, tt.samplingPercentage)

			sampled := 0
			for _, traceID := range genRandomTraceIDs(traceCount) {
				trace := newTraceStringAttrs(nil, "example", "value")

				decision, err := probabilisticSampler.Evaluate(t.Context(), traceID, trace)
				assert.NoError(t, err)

				if decision == samplingpolicy.Sampled {
					sampled++
				}
			}

			effectiveSamplingPercentage := float32(sampled) / float32(traceCount) * 100
			assert.InDelta(t, tt.expectedSamplingPercentage, effectiveSamplingPercentage, 0.2,
				"Effective sampling percentage is %f, expected %f", effectiveSamplingPercentage, tt.expectedSamplingPercentage,
			)
		})
	}
}

// enableTracestateFeatureGate flips the
// processor.tailsamplingprocessor.usetracestate feature gate on for
// the duration of a test, restoring its prior value via t.Cleanup.
func enableTracestateFeatureGate(t *testing.T) {
	t.Helper()
	gate := metadata.ProcessorTailsamplingprocessorUsetracestateFeatureGate
	prev := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), prev))
	})
}

// TestProbabilisticSampling_RespectsTraceState verifies that the
// probabilistic sampler prefers W3C tracestate sampling information
// (the "ot" section's rv and th fields) over its FNV hash fallback,
// matching the equalizing semantics described by the OpenTelemetry
// probability sampling specification.
func TestProbabilisticSampling_RespectsTraceState(t *testing.T) {
	enableTracestateFeatureGate(t)

	// Helper: a randomness value of all 0xf, which is the maximum
	// possible R-value and is therefore sampled by any threshold
	// less than NeverSampleThreshold.
	rvAlwaysSampled := "ffffffffffffff"
	// A randomness value of 0x40_0000_0000_0000 (zero-padded). At
	// 50% sampling (T = 0x80...), this R < T so it is NOT sampled.
	// At 25% sampling (T = 0xc0...), R < T so it is NOT sampled.
	// At 75% sampling (T = 0x40...), R == T so it IS sampled.
	rvBelow50Percent := "40000000000000"

	tests := []struct {
		name               string
		samplingPercentage float64
		tracestates        []string
		want               samplingpolicy.Decision
	}{
		{
			name:               "rv only, max randomness, 50% configured -> sampled",
			samplingPercentage: 50,
			tracestates:        []string{"ot=rv:" + rvAlwaysSampled},
			want:               samplingpolicy.Sampled,
		},
		{
			name:               "rv only, low randomness, 50% configured -> not sampled",
			samplingPercentage: 50,
			tracestates:        []string{"ot=rv:" + rvBelow50Percent},
			want:               samplingpolicy.NotSampled,
		},
		{
			name:               "rv only, configured 0% -> never sampled",
			samplingPercentage: 0,
			tracestates:        []string{"ot=rv:" + rvAlwaysSampled},
			want:               samplingpolicy.NotSampled,
		},
		{
			name:               "rv only, configured 100% -> always sampled",
			samplingPercentage: 100,
			tracestates:        []string{"ot=rv:" + rvBelow50Percent},
			want:               samplingpolicy.Sampled,
		},
		{
			name: "incoming th ignored: th:8 with max R, configured 100% -> sampled (R vs configured only)",
			// The probabilistic policy does not factor in
			// incoming th when making its decision. R passes
			// the configured 100% (th:0) threshold, so the
			// trace is kept; per-span preservation of stricter
			// incoming th is handled by the processor on the
			// way out.
			samplingPercentage: 100,
			tracestates:        []string{"ot=th:8;rv:" + rvAlwaysSampled},
			want:               samplingpolicy.Sampled,
		},
		{
			name: "incoming th ignored: th:8 with low R, configured 100% -> sampled",
			// At 100% configured, R always passes; incoming
			// th does not tighten the policy's decision.
			samplingPercentage: 100,
			tracestates:        []string{"ot=th:8;rv:" + rvBelow50Percent},
			want:               samplingpolicy.Sampled,
		},
		{
			name: "configured 0% with th:0 incoming -> not sampled",
			// Configured 0% (NeverSampleThreshold) dominates;
			// incoming th does not matter.
			samplingPercentage: 0,
			tracestates:        []string{"ot=th:0;rv:" + rvAlwaysSampled},
			want:               samplingpolicy.NotSampled,
		},
		{
			name: "multi-span: shared rv used for decision regardless of varying th",
			// Two spans, same rv, different th. The decision
			// only uses rv vs configured (50%); incoming th
			// values are ignored by the policy.
			samplingPercentage: 50,
			tracestates: []string{
				"ot=th:0;rv:" + rvAlwaysSampled,
				"ot=th:8;rv:" + rvAlwaysSampled,
			},
			want: samplingpolicy.Sampled,
		},
		{
			name: "multi-span: rv on later span still found",
			// First span has only th; second span has rv.
			// The scan must find the rv to take the
			// tracestate decision path.
			samplingPercentage: 50,
			tracestates: []string{
				"ot=th:e",
				"ot=rv:" + rvAlwaysSampled,
			},
			want: samplingpolicy.Sampled,
		},
		{
			name: "invalid tracestate falls back to FNV",
			// Invalid tracestate parsing fails; with no
			// other span carrying sampling info, the legacy
			// FNV hash path is used. At 100% configured, the
			// hash path always samples.
			samplingPercentage: 100,
			tracestates:        []string{"ot=not_a_valid_key$"},
			want:               samplingpolicy.Sampled,
		},
		{
			name: "malformed rv hex falls back to FNV",
			// rv must be 14 lowercase hex digits; a non-hex
			// value fails parsing of the whole tracestate
			// and is treated as missing. At 100% configured
			// the FNV fallback always samples.
			samplingPercentage: 100,
			tracestates:        []string{"ot=rv:zzzzzzzzzzzzzz"},
			want:               samplingpolicy.Sampled,
		},
		{
			name: "empty trace (no spans) falls back to FNV",
			// scanOTelTracestate returns no sampling info
			// for an empty trace; the FNV path applies.
			samplingPercentage: 100,
			tracestates:        nil,
			want:               samplingpolicy.Sampled,
		},
		{
			name: "0% configured with rv and th present -> never sampled",
			// NeverSampleThreshold must dominate any incoming
			// th, regardless of rv.
			samplingPercentage: 0,
			tracestates:        []string{"ot=th:8;rv:" + rvAlwaysSampled},
			want:               samplingpolicy.NotSampled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := NewProbabilisticSampler(componenttest.NewNopTelemetrySettings(), "", tt.samplingPercentage)

			trace := newTraceWithTraceStates(tt.tracestates)
			// Use an all-zero TraceID so that the FNV
			// fallback path is deterministic and so the
			// W3C TraceID-derived randomness is zero (no
			// accidental sampled-by-traceid effects).
			traceID := pcommon.TraceID{}

			decision, err := sampler.Evaluate(t.Context(), traceID, trace)
			require.NoError(t, err)
			assert.Equal(t, tt.want, decision)
		})
	}
}

// TestProbabilisticSampling_TraceStateRandomnessConsistentWithProbability
// verifies that when traces carry explicit R-values in tracestate,
// the empirical sampling rate matches the configured percentage.
func TestProbabilisticSampling_TraceStateRandomnessConsistentWithProbability(t *testing.T) {
	enableTracestateFeatureGate(t)
	const (
		traceCount         = 50_000
		samplingPercentage = 25
	)
	sampler := NewProbabilisticSampler(componenttest.NewNopTelemetrySettings(), "", samplingPercentage)

	r := rand.New(rand.NewPCG(7, 8))
	sampled := 0
	for range traceCount {
		// Build a tracestate with a random 56-bit rv value
		// (14 hex digits, zero-padded).
		rv := r.Uint64() & ((1 << 56) - 1)
		ts := []string{tracestateWithRValue(rv)}
		trace := newTraceWithTraceStates(ts)
		// Use a TraceID independent of rv so the rv truly
		// drives the decision.
		var tid [16]byte
		binary.BigEndian.PutUint64(tid[:8], r.Uint64())
		binary.BigEndian.PutUint64(tid[8:], r.Uint64())
		decision, err := sampler.Evaluate(t.Context(), pcommon.TraceID(tid), trace)
		require.NoError(t, err)
		if decision == samplingpolicy.Sampled {
			sampled++
		}
	}
	got := float64(sampled) / float64(traceCount) * 100
	assert.InDelta(t, samplingPercentage, got, 0.5,
		"Effective sampling percentage from rv-driven decisions is %f, expected %f", got, float64(samplingPercentage))
}

// TestProbabilisticSampling_TraceStateIgnoredWhenGateDisabled verifies
// that with the feature gate disabled the sampler falls back to the
// legacy FNV trace ID hash even when spans carry OpenTelemetry
// tracestate sampling info. Tracestate values are chosen so that the
// gate-enabled path would yield the opposite decision.
func TestProbabilisticSampling_TraceStateIgnoredWhenGateDisabled(t *testing.T) {
	// Explicitly force the gate off so this test does not depend
	// on the global default or on other tests restoring it
	// correctly.
	gate := metadata.ProcessorTailsamplingprocessorUsetracestateFeatureGate
	prev := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), false))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), prev))
	})

	// 100% sampling: FNV path always samples regardless of trace
	// content. With the gate enabled, the configured 0% incoming
	// th would yield NotSampled - so observing Sampled proves we
	// took the FNV path.
	sampler := NewProbabilisticSampler(componenttest.NewNopTelemetrySettings(), "", 100)
	// rv=0 + th encoding ~0% via NeverSampleThreshold isn't
	// expressible as a t-value (NeverSampleThreshold's tvalue is
	// ""). Use a very aggressive but expressible threshold: th
	// with all f's would mean ~0% sampling; here we use th:fffff
	// (sampling at ~2^-20).
	trace := newTraceWithTraceStates([]string{"ot=th:fffff;rv:00000000000000"})
	decision, err := sampler.Evaluate(t.Context(), pcommon.TraceID{}, trace)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision,
		"gate disabled: expected FNV fallback (sampled at 100%%) regardless of tracestate")
}

func tracestateWithRValue(rv uint64) string {
	// Format as 14 lowercase hex digits, zero-padded.
	const hexDigits = "0123456789abcdef"
	buf := make([]byte, 14)
	for i := 13; i >= 0; i-- {
		buf[i] = hexDigits[rv&0xf]
		rv >>= 4
	}
	return "ot=rv:" + string(buf)
}

func newTraceWithTraceStates(tracestates []string) *samplingpolicy.TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	for _, ts := range tracestates {
		span := ils.Spans().AppendEmpty()
		span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
		span.TraceState().FromRaw(ts)
	}
	return &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
}

func genRandomTraceIDs(num int) (ids []pcommon.TraceID) {
	// NOTE: using a fixed seed is intentional here,
	// as otherwise the delta in the tests above will
	// be unpredictable.
	r := rand.New(rand.NewPCG(123, 456))
	ids = make([]pcommon.TraceID, 0, num)
	for range num {
		traceID := [16]byte{}
		binary.BigEndian.PutUint64(traceID[:8], r.Uint64())
		binary.BigEndian.PutUint64(traceID[8:], r.Uint64())
		ids = append(ids, pcommon.TraceID(traceID))
	}
	return ids
}
