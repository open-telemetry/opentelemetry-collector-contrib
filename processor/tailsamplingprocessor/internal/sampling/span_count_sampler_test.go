// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestEvaluate_OnlyMinSpans(t *testing.T) {
	filter := NewSpanCount(componenttest.NewNopTelemetrySettings(), 3, 0)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc        string
		NumberSpans []int32
		Decision    Decision
	}{
		{
			"Spans less than the minSpans, in one single batch",
			[]int32{
				1,
			},
			NotSampled,
		},
		{
			"Same number of spans as the minSpans, in one single batch",
			[]int32{
				3,
			},
			Sampled,
		},
		{
			"Spans greater than the minSpans, in one single batch",
			[]int32{
				4,
			},
			Sampled,
		},
		{
			"Spans less than the minSpans, across multiple batches",
			[]int32{
				1, 1,
			},
			NotSampled,
		},
		{
			"Same number of spans as the minSpans, across multiple batches",
			[]int32{
				1, 2, 1,
			},
			Sampled,
		},
		{
			"Spans greater than the minSpans, across multiple batches",
			[]int32{
				1, 2, 3,
			},
			Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			decision, err := filter.Evaluate(context.Background(), traceID, newTraceWithMultipleSpans(c.NumberSpans))

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
		Decision    Decision
	}{
		{
			"Spans greater than the maxSpans, in one single batch",
			[]int32{
				21,
			},
			NotSampled,
		},
		{
			"Same number of spans as the maxSpans, in one single batch",
			[]int32{
				20,
			},
			Sampled,
		},
		{
			"Spans less than the maxSpans, in one single batch",
			[]int32{
				19,
			},
			Sampled,
		},
		{
			"Spans gather than the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 6,
			},
			NotSampled,
		},
		{
			"Same number of spans as the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 5,
			},
			Sampled,
		},
		{
			"Spans less than the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5,
			},
			Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			decision, err := filter.Evaluate(context.Background(), traceID, newTraceWithMultipleSpans(c.NumberSpans))

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
		Decision    Decision
	}{
		{
			"Spans less than the minSpans, in one single batch",
			[]int32{
				1,
			},
			NotSampled,
		},
		{
			"Spans greater than the maxSpans, in one single batch",
			[]int32{
				21,
			},
			NotSampled,
		},
		{
			"Spans range of minSpan and maxSpans, in one single batch",
			[]int32{
				4,
			},
			Sampled,
		},
		{
			"Spans less than the minSpans, across multiple batches",
			[]int32{
				1, 1,
			},
			NotSampled,
		},
		{
			"Spans greater than the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 6,
			},
			NotSampled,
		},
		{
			"Spans range of minSpan and maxSpans, across multiple batches",
			[]int32{
				1, 2, 1,
			},
			Sampled,
		},
		{
			"Same number of spans as the minSpans, in one single batch",
			[]int32{
				3,
			},
			Sampled,
		},
		{
			"Same number of spans as the maxSpans, in one single batch",
			[]int32{
				20,
			},
			Sampled,
		},
		{
			"Same number of spans as the minSpans, across multiple batches",
			[]int32{
				1, 2,
			},
			Sampled,
		},
		{
			"Same number of spans as the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 5,
			},
			Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			decision, err := filter.Evaluate(context.Background(), traceID, newTraceWithMultipleSpans(c.NumberSpans))

			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func newTraceWithMultipleSpans(numberSpans []int32) *TraceData {
	var totalNumberSpans = int32(0)

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

	traceSpanCount := &atomic.Int64{}
	traceSpanCount.Store(int64(totalNumberSpans))
	return &TraceData{
		ReceivedBatches: traces,
		SpanCount:       traceSpanCount,
	}
}
