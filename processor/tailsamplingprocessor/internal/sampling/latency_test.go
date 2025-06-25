// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// testOTEP235BehaviorLatency tests sampling decision using proper OTEP 235 threshold logic
// for latency filter tests. Uses fixed randomness values to make tests deterministic.
func testOTEP235BehaviorLatency(t *testing.T, filter PolicyEvaluator, traceID pcommon.TraceID, trace *TraceData, expectSampled bool) {
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

func TestEvaluate_Latency(t *testing.T) {
	filter := NewLatency(componenttest.NewNopTelemetrySettings(), 5000, 0)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	now := time.Now()

	cases := []struct {
		Desc         string
		Spans        []spanWithTimeAndDuration
		ExpectSample bool
	}{
		{
			"trace duration shorter than threshold",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  4500 * time.Millisecond,
				},
			},
			false,
		},
		{
			"trace duration is equal to threshold",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  5000 * time.Millisecond,
				},
			},
			true,
		},
		{
			"total trace duration is longer than threshold but every single span is shorter",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  3000 * time.Millisecond,
				},
				{
					StartTime: now.Add(2500 * time.Millisecond),
					Duration:  3000 * time.Millisecond,
				},
			},
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			testOTEP235BehaviorLatency(t, filter, traceID, newTraceWithSpans(c.Spans), c.ExpectSample)
		})
	}
}

func TestEvaluate_Bounded_Latency(t *testing.T) {
	filter := NewLatency(componenttest.NewNopTelemetrySettings(), 5000, 10000)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	now := time.Now()

	cases := []struct {
		Desc         string
		Spans        []spanWithTimeAndDuration
		ExpectSample bool
	}{
		{
			"trace duration shorter than lower bound",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  4500 * time.Millisecond,
				},
			},
			false,
		},
		{
			"trace duration is equal to lower bound",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  5000 * time.Millisecond,
				},
			},
			false,
		},
		{
			"trace duration is within lower and upper bounds",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  5001 * time.Millisecond,
				},
			},
			true,
		},
		{
			"trace duration is above upper bound",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  10001 * time.Millisecond,
				},
			},
			false,
		},
		{
			"trace duration equals upper bound",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  10000 * time.Millisecond,
				},
			},
			true,
		},
		{
			"total trace duration is longer than threshold but every single span is shorter",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  3000 * time.Millisecond,
				},
				{
					StartTime: now.Add(2500 * time.Millisecond),
					Duration:  3000 * time.Millisecond,
				},
			},
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			testOTEP235BehaviorLatency(t, filter, traceID, newTraceWithSpans(c.Spans), c.ExpectSample)
		})
	}
}

type spanWithTimeAndDuration struct {
	StartTime time.Time
	Duration  time.Duration
}

func newTraceWithSpans(spans []spanWithTimeAndDuration) *TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	for _, s := range spans {
		span := ils.Spans().AppendEmpty()
		span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(s.StartTime))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(s.StartTime.Add(s.Duration)))
	}

	return &TraceData{
		ReceivedBatches: traces,
	}
}
