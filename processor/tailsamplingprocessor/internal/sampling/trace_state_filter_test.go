// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// testOTEP235BehaviorTraceState tests sampling decision using proper OTEP 235 threshold logic
// for trace state filter tests. Uses fixed randomness values to make tests deterministic.
func testOTEP235BehaviorTraceState(t *testing.T, filter PolicyEvaluator, traceID pcommon.TraceID, trace *TraceData, expectSampled bool) {
	decision, err := filter.Evaluate(context.Background(), traceID, trace)
	assert.NoError(t, err)

	// Test with randomness = 0 (always samples if threshold is AlwaysSampleThreshold)
	randomnessZero, err := sampling.UnsignedToRandomness(0)
	assert.NoError(t, err)

	// Test with randomness near max (only samples if threshold is very high)
	randomnessHigh, err := sampling.UnsignedToRandomness(sampling.MaxAdjustedCount - 1)
	assert.NoError(t, err)

	if expectSampled {
		// If we expect sampling, the decision should have a low threshold that allows sampling
		assert.True(t, decision.ShouldSample(randomnessZero), "Decision should sample with randomness=0")
		// For true "always sample" decisions, even high randomness should work
		if decision.Threshold == sampling.AlwaysSampleThreshold {
			assert.True(t, decision.ShouldSample(randomnessHigh), "AlwaysSampleThreshold should sample with any randomness")
		}
	} else {
		// If we expect no sampling, the decision should have high threshold (NeverSampleThreshold)
		assert.False(t, decision.ShouldSample(randomnessZero), "Decision should not sample with randomness=0")
		assert.False(t, decision.ShouldSample(randomnessHigh), "Decision should not sample with randomness=high")
		assert.Equal(t, sampling.NeverSampleThreshold, decision.Threshold, "Non-sampling decision should have NeverSampleThreshold")
	}
}

// TestTraceStateCfg is replicated with StringAttributeCfg
type TestTraceStateCfg struct {
	Key    string
	Values []string
}

func TestTraceStateFilter(t *testing.T) {
	cases := []struct {
		Desc         string
		Trace        *TraceData
		filterCfg    *TestTraceStateCfg
		ExpectSample bool
	}{
		{
			Desc:         "nonmatching trace_state key",
			Trace:        newTraceState("non_matching=value"),
			filterCfg:    &TestTraceStateCfg{Key: "example", Values: []string{"value"}},
			ExpectSample: false,
		},
		{
			Desc:         "nonmatching trace_state value",
			Trace:        newTraceState("example=non_matching"),
			filterCfg:    &TestTraceStateCfg{Key: "example", Values: []string{"value"}},
			ExpectSample: false,
		},
		{
			Desc:         "matching trace_state",
			Trace:        newTraceState("example=value"),
			filterCfg:    &TestTraceStateCfg{Key: "example", Values: []string{"value"}},
			ExpectSample: true,
		},
		{
			Desc:         "nonmatching trace_state on empty filter list",
			Trace:        newTraceState("example=value"),
			filterCfg:    &TestTraceStateCfg{Key: "example", Values: []string{}},
			ExpectSample: false,
		},
		{
			Desc:         "nonmatching trace_state on multiple key-values",
			Trace:        newTraceState("example=non_matching,non_matching=value"),
			filterCfg:    &TestTraceStateCfg{Key: "example", Values: []string{"value"}},
			ExpectSample: false,
		},
		{
			Desc:         "matching trace_state on multiple key-values",
			Trace:        newTraceState("example=value,non_matching=value"),
			filterCfg:    &TestTraceStateCfg{Key: "example", Values: []string{"value"}},
			ExpectSample: true,
		},
		{
			Desc:         "nonmatching trace_state on multiple filter list",
			Trace:        newTraceState("example=non_matching"),
			filterCfg:    &TestTraceStateCfg{Key: "example", Values: []string{"value1", "value2"}},
			ExpectSample: false,
		},
		{
			Desc:         "matching trace_state on multiple filter list",
			Trace:        newTraceState("example=value1"),
			filterCfg:    &TestTraceStateCfg{Key: "example", Values: []string{"value1", "value2"}},
			ExpectSample: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			filter := NewTraceStateFilter(componenttest.NewNopTelemetrySettings(), c.filterCfg.Key, c.filterCfg.Values)
			testOTEP235BehaviorTraceState(t, filter, pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}), c.Trace, c.ExpectSample)
		})
	}
}

func newTraceState(traceState string) *TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.TraceState().FromRaw(traceState)
	return &TraceData{
		ReceivedBatches: traces,
	}
}
