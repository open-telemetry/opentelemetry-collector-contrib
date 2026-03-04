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

func TestEvaluate_OnlyMinSpans(t *testing.T) {
	filter := NewSpanCount(componenttest.NewNopTelemetrySettings(), 3, 0)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc        string
		NumberSpans []int32
		Decision    samplingpolicy.Decision
	}{
		{
			"Spans less than the minSpans, in one single batch",
			[]int32{
				1,
			},
			samplingpolicy.NotSampled,
		},
		{
			"Same number of spans as the minSpans, in one single batch",
			[]int32{
				3,
			},
			samplingpolicy.Sampled,
		},
		{
			"Spans greater than the minSpans, in one single batch",
			[]int32{
				4,
			},
			samplingpolicy.Sampled,
		},
		{
			"Spans less than the minSpans, across multiple batches",
			[]int32{
				1, 1,
			},
			samplingpolicy.NotSampled,
		},
		{
			"Same number of spans as the minSpans, across multiple batches",
			[]int32{
				1, 2, 1,
			},
			samplingpolicy.Sampled,
		},
		{
			"Spans greater than the minSpans, across multiple batches",
			[]int32{
				1, 2, 3,
			},
			samplingpolicy.Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			decision, err := filter.Evaluate(t.Context(), traceID, newTraceWithMultipleSpans(c.NumberSpans))

			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func TestEvaluate_OnlyMaxSpans(t *testing.T) {
	filter := NewSpanCount(componenttest.NewNopTelemetrySettings(), 0, 20)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc        string
		NumberSpans []int32
		Decision    samplingpolicy.Decision
	}{
		{
			"Spans greater than the maxSpans, in one single batch",
			[]int32{
				21,
			},
			samplingpolicy.NotSampled,
		},
		{
			"Same number of spans as the maxSpans, in one single batch",
			[]int32{
				20,
			},
			samplingpolicy.Sampled,
		},
		{
			"Spans less than the maxSpans, in one single batch",
			[]int32{
				19,
			},
			samplingpolicy.Sampled,
		},
		{
			"Spans gather than the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 6,
			},
			samplingpolicy.NotSampled,
		},
		{
			"Same number of spans as the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 5,
			},
			samplingpolicy.Sampled,
		},
		{
			"Spans less than the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5,
			},
			samplingpolicy.Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			decision, err := filter.Evaluate(t.Context(), traceID, newTraceWithMultipleSpans(c.NumberSpans))

			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func TestEvaluate_RangeOfSpans(t *testing.T) {
	filter := NewSpanCount(componenttest.NewNopTelemetrySettings(), 3, 20)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc        string
		NumberSpans []int32
		Decision    samplingpolicy.Decision
	}{
		{
			"Spans less than the minSpans, in one single batch",
			[]int32{
				1,
			},
			samplingpolicy.NotSampled,
		},
		{
			"Spans greater than the maxSpans, in one single batch",
			[]int32{
				21,
			},
			samplingpolicy.NotSampled,
		},
		{
			"Spans range of minSpan and maxSpans, in one single batch",
			[]int32{
				4,
			},
			samplingpolicy.Sampled,
		},
		{
			"Spans less than the minSpans, across multiple batches",
			[]int32{
				1, 1,
			},
			samplingpolicy.NotSampled,
		},
		{
			"Spans greater than the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 6,
			},
			samplingpolicy.NotSampled,
		},
		{
			"Spans range of minSpan and maxSpans, across multiple batches",
			[]int32{
				1, 2, 1,
			},
			samplingpolicy.Sampled,
		},
		{
			"Same number of spans as the minSpans, in one single batch",
			[]int32{
				3,
			},
			samplingpolicy.Sampled,
		},
		{
			"Same number of spans as the maxSpans, in one single batch",
			[]int32{
				20,
			},
			samplingpolicy.Sampled,
		},
		{
			"Same number of spans as the minSpans, across multiple batches",
			[]int32{
				1, 2,
			},
			samplingpolicy.Sampled,
		},
		{
			"Same number of spans as the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 5,
			},
			samplingpolicy.Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			decision, err := filter.Evaluate(t.Context(), traceID, newTraceWithMultipleSpans(c.NumberSpans))

			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func newTraceWithMultipleSpans(numberSpans []int32) *samplingpolicy.TraceData {
	totalNumberSpans := int32(0)

	// For each resource, going to create the number of spans defined in the array
	traces := ptrace.NewTraces()
	for i := range numberSpans {
		// Creates trace
		rs := traces.ResourceSpans().AppendEmpty()
		ils := rs.ScopeSpans().AppendEmpty()

		for r := 0; r < int(numberSpans[i]); r++ {
			span := ils.Spans().AppendEmpty()
			span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
			span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
		}
		totalNumberSpans += numberSpans[i]
	}

	return &samplingpolicy.TraceData{
		ReceivedBatches: traces,
		SpanCount:       int64(totalNumberSpans),
	}
}
