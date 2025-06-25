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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// testOTEP235BehaviorSpanCount tests sampling decision using proper OTEP 235 threshold logic
// instead of boolean comparison. Uses fixed randomness values to make tests deterministic.
func testOTEP235BehaviorSpanCount(t *testing.T, filter PolicyEvaluator, traceID pcommon.TraceID, trace *TraceData, expectSampled bool) {
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
		assert.False(t, decision.ShouldSample(randomnessHigh), "Decision should not sample with any randomness")
		assert.Equal(t, sampling.NeverSampleThreshold, decision.Threshold, "Non-sampling decision should have NeverSampleThreshold")
	}
}

func TestEvaluate_OnlyMinSpans(t *testing.T) {
	filter := NewSpanCount(componenttest.NewNopTelemetrySettings(), 3, 0)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc         string
		NumberSpans  []int32
		ExpectSample bool
	}{
		{
			"Spans less than the minSpans, in one single batch",
			[]int32{
				1,
			},
			false,
		},
		{
			"Same number of spans as the minSpans, in one single batch",
			[]int32{
				3,
			},
			true,
		},
		{
			"Spans greater than the minSpans, in one single batch",
			[]int32{
				4,
			},
			true,
		},
		{
			"Spans less than the minSpans, across multiple batches",
			[]int32{
				1, 1,
			},
			false,
		},
		{
			"Same number of spans as the minSpans, across multiple batches",
			[]int32{
				1, 2, 1,
			},
			true,
		},
		{
			"Spans greater than the minSpans, across multiple batches",
			[]int32{
				1, 2, 3,
			},
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			testOTEP235BehaviorSpanCount(t, filter, traceID, newTraceWithMultipleSpans(c.NumberSpans), c.ExpectSample)
		})
	}
}

func TestEvaluate_OnlyMaxSpans(t *testing.T) {
	filter := NewSpanCount(componenttest.NewNopTelemetrySettings(), 0, 20)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc         string
		NumberSpans  []int32
		ExpectSample bool
	}{
		{
			"Spans greater than the maxSpans, in one single batch",
			[]int32{
				21,
			},
			false,
		},
		{
			"Same number of spans as the maxSpans, in one single batch",
			[]int32{
				20,
			},
			true,
		},
		{
			"Spans less than the maxSpans, in one single batch",
			[]int32{
				19,
			},
			true,
		},
		{
			"Spans gather than the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 6,
			},
			false,
		},
		{
			"Same number of spans as the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 5,
			},
			true,
		},
		{
			"Spans less than the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5,
			},
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			testOTEP235BehaviorSpanCount(t, filter, traceID, newTraceWithMultipleSpans(c.NumberSpans), c.ExpectSample)
		})
	}
}

func TestEvaluate_RangeOfSpans(t *testing.T) {
	filter := NewSpanCount(componenttest.NewNopTelemetrySettings(), 3, 20)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	cases := []struct {
		Desc         string
		NumberSpans  []int32
		ExpectSample bool
	}{
		{
			"Spans less than the minSpans, in one single batch",
			[]int32{
				1,
			},
			false,
		},
		{
			"Spans greater than the maxSpans, in one single batch",
			[]int32{
				21,
			},
			false,
		},
		{
			"Spans range of minSpan and maxSpans, in one single batch",
			[]int32{
				4,
			},
			true,
		},
		{
			"Spans less than the minSpans, across multiple batches",
			[]int32{
				1, 1,
			},
			false,
		},
		{
			"Spans greater than the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 6,
			},
			false,
		},
		{
			"Spans range of minSpan and maxSpans, across multiple batches",
			[]int32{
				1, 2, 1,
			},
			true,
		},
		{
			"Same number of spans as the minSpans, in one single batch",
			[]int32{
				3,
			},
			true,
		},
		{
			"Same number of spans as the maxSpans, in one single batch",
			[]int32{
				20,
			},
			true,
		},
		{
			"Same number of spans as the minSpans, across multiple batches",
			[]int32{
				1, 2,
			},
			true,
		},
		{
			"Same number of spans as the maxSpans, across multiple batches",
			[]int32{
				1, 2, 3, 4, 5, 5,
			},
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			testOTEP235BehaviorSpanCount(t, filter, traceID, newTraceWithMultipleSpans(c.NumberSpans), c.ExpectSample)
		})
	}
}

func newTraceWithMultipleSpans(numberSpans []int32) *TraceData {
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

	traceSpanCount := &atomic.Int64{}
	traceSpanCount.Store(int64(totalNumberSpans))
	return &TraceData{
		ReceivedBatches: traces,
		SpanCount:       traceSpanCount,
	}
}
