// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestBudgetLimiterBatchThreshold(t *testing.T) {
	// Burst 5 => starting budget of 5 spans. Three traces of 2 spans
	// each (total 6) exceed the budget by one trace.
	bl := newBudgetLimiter(componenttest.NewNopTelemetrySettings(), 5, 5, traceSpanCount)

	a := rlTrace(1, 100, 2)
	b := rlTrace(2, 50, 2)
	c := rlTrace(3, 10, 2)
	bl.CalculateThreshold(t.Context(), []*samplingpolicy.TraceData{a, b, c})

	wantThreshold, err := sampling.UnsignedToThreshold(11)
	require.NoError(t, err)

	// Highest two randomness values are kept; the threshold is the first
	// excluded randomness (10) plus one.
	for _, tc := range []struct {
		name       string
		td         *samplingpolicy.TraceData
		randomness uint64
		want       samplingpolicy.Decision
	}{
		{"a", a, 100, samplingpolicy.Sampled},
		{"b", b, 50, samplingpolicy.Sampled},
		{"c", c, 10, samplingpolicy.NotSampled},
	} {
		decision, th, err := bl.EvaluateWithThreshold(t.Context(), traceIDOf(tc.td), tc.td)
		require.NoError(t, err)
		assert.Equal(t, tc.want, decision, tc.name)
		if tc.want == samplingpolicy.Sampled {
			assert.Equal(t, wantThreshold, th, tc.name)
			// Consistency: the reported threshold must sample the
			// trace's own randomness.
			assert.True(t, th.ShouldSample(mustRandomness(t, tc.randomness)), tc.name)
		}
	}
}

func TestBudgetLimiterBatchWholeBatchFits(t *testing.T) {
	bl := newBudgetLimiter(componenttest.NewNopTelemetrySettings(), 10, 10, traceSpanCount)

	traces := []*samplingpolicy.TraceData{rlTrace(1, 100, 2), rlTrace(2, 50, 2)}
	bl.CalculateThreshold(t.Context(), traces)

	for _, td := range traces {
		decision, th, err := bl.EvaluateWithThreshold(t.Context(), traceIDOf(td), td)
		require.NoError(t, err)
		assert.Equal(t, samplingpolicy.Sampled, decision)
		assert.Equal(t, sampling.AlwaysSampleThreshold, th)
	}
}

func TestBudgetLimiterBatchLargeTraceDropped(t *testing.T) {
	bl := newBudgetLimiter(componenttest.NewNopTelemetrySettings(), 2, 2, traceSpanCount)

	td := rlTrace(1, 100, 5) // 5 spans > budget of 2
	bl.CalculateThreshold(t.Context(), []*samplingpolicy.TraceData{td})

	decision, _, err := bl.EvaluateWithThreshold(t.Context(), traceIDOf(td), td)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)
}

func TestBudgetLimiterOversizedTraceDoesNotStarveOthers(t *testing.T) {
	// Budget 10. d costs more than the entire burst and has the highest
	// randomness; without excluding it up front it would zero out the
	// budget calculation for a and c, which would otherwise both fit.
	bl := newBudgetLimiter(componenttest.NewNopTelemetrySettings(), 10, 10, traceSpanCount)
	d := rlTrace(1, 200, 20)
	a := rlTrace(2, 100, 3)
	c := rlTrace(3, 50, 3)
	bl.CalculateThreshold(t.Context(), []*samplingpolicy.TraceData{d, a, c})

	for _, tc := range []struct {
		name string
		td   *samplingpolicy.TraceData
		want samplingpolicy.Decision
	}{
		{"d", d, samplingpolicy.NotSampled},
		{"a", a, samplingpolicy.Sampled},
		{"c", c, samplingpolicy.Sampled},
	} {
		decision, _, err := bl.EvaluateWithThreshold(t.Context(), traceIDOf(tc.td), tc.td)
		require.NoError(t, err)
		assert.Equal(t, tc.want, decision, tc.name)
	}
}
