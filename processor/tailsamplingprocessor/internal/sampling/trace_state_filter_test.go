// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

// TestTraceStateCfg is replicated with StringAttributeCfg
type TestTraceStateCfg struct {
	Key    string
	Values []string
}

func TestTraceStateFilter(t *testing.T) {
	cases := []struct {
		Desc      string
		Trace     *samplingpolicy.TraceData
		filterCfg *TestTraceStateCfg
		Decision  samplingpolicy.Decision
	}{
		{
			Desc:      "nonmatching trace_state key",
			Trace:     newTraceState("non_matching=value"),
			filterCfg: &TestTraceStateCfg{Key: "example", Values: []string{"value"}},
			Decision:  samplingpolicy.NotSampled,
		},
		{
			Desc:      "nonmatching trace_state value",
			Trace:     newTraceState("example=non_matching"),
			filterCfg: &TestTraceStateCfg{Key: "example", Values: []string{"value"}},
			Decision:  samplingpolicy.NotSampled,
		},
		{
			Desc:      "matching trace_state",
			Trace:     newTraceState("example=value"),
			filterCfg: &TestTraceStateCfg{Key: "example", Values: []string{"value"}},
			Decision:  samplingpolicy.Sampled,
		},
		{
			Desc:      "nonmatching trace_state on empty filter list",
			Trace:     newTraceState("example=value"),
			filterCfg: &TestTraceStateCfg{Key: "example", Values: []string{}},
			Decision:  samplingpolicy.NotSampled,
		},
		{
			Desc:      "nonmatching trace_state on multiple key-values",
			Trace:     newTraceState("example=non_matching,non_matching=value"),
			filterCfg: &TestTraceStateCfg{Key: "example", Values: []string{"value"}},
			Decision:  samplingpolicy.NotSampled,
		},
		{
			Desc:      "matching trace_state on multiple key-values",
			Trace:     newTraceState("example=value,non_matching=value"),
			filterCfg: &TestTraceStateCfg{Key: "example", Values: []string{"value"}},
			Decision:  samplingpolicy.Sampled,
		},
		{
			Desc:      "nonmatching trace_state on multiple filter list",
			Trace:     newTraceState("example=non_matching"),
			filterCfg: &TestTraceStateCfg{Key: "example", Values: []string{"value1", "value2"}},
			Decision:  samplingpolicy.NotSampled,
		},
		{
			Desc:      "matching trace_state on multiple filter list",
			Trace:     newTraceState("example=value1"),
			filterCfg: &TestTraceStateCfg{Key: "example", Values: []string{"value1", "value2"}},
			Decision:  samplingpolicy.Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			filter := NewTraceStateFilter(componenttest.NewNopTelemetrySettings(), c.filterCfg.Key, c.filterCfg.Values)
			decision, err := filter.Evaluate(t.Context(), pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}), c.Trace)
			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func newTraceState(traceState string) *samplingpolicy.TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.TraceState().FromRaw(traceState)
	return &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
}
