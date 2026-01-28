// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestTraceFlags(t *testing.T) {
	cases := []struct {
		Desc     string
		Trace    *samplingpolicy.TraceData
		Decision samplingpolicy.Decision
	}{
		{
			Desc:     "sampled trace_flags",
			Trace:    newTraceFlags(true),
			Decision: samplingpolicy.Sampled,
		},
		{
			Desc:     "non-sampled trace_flags",
			Trace:    newTraceFlags(false),
			Decision: samplingpolicy.NotSampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			filter := NewTraceFlags(componenttest.NewNopTelemetrySettings())
			decision, err := filter.Evaluate(t.Context(), pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}), c.Trace)
			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func newTraceFlags(sampled bool) *samplingpolicy.TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	if sampled {
		span.SetFlags(uint32(trace.FlagsSampled))
	}
	return &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
}
