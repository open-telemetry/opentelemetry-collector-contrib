// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestEvaluate_Latency(t *testing.T) {
	filter := NewLatency(componenttest.NewNopTelemetrySettings(), 5000, 0)

	testTraceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	now := time.Now()

	cases := []struct {
		Desc     string
		Spans    []spanWithTimeAndDuration
		Decision samplingpolicy.Decision
	}{
		{
			"trace duration shorter than threshold",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  4500 * time.Millisecond,
				},
			},
			samplingpolicy.NotSampled,
		},
		{
			"trace duration is equal to threshold",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  5000 * time.Millisecond,
				},
			},
			samplingpolicy.Sampled,
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
			samplingpolicy.Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			decision, err := filter.Evaluate(t.Context(), testTraceID, newTraceWithSpans(c.Spans))

			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func TestEvaluate_Bounded_Latency(t *testing.T) {
	filter := NewLatency(componenttest.NewNopTelemetrySettings(), 5000, 10000)

	testTraceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	now := time.Now()

	cases := []struct {
		Desc     string
		Spans    []spanWithTimeAndDuration
		Decision samplingpolicy.Decision
	}{
		{
			"trace duration shorter than lower bound",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  4500 * time.Millisecond,
				},
			},
			samplingpolicy.NotSampled,
		},
		{
			"trace duration is equal to lower bound",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  5000 * time.Millisecond,
				},
			},
			samplingpolicy.NotSampled,
		},
		{
			"trace duration is within lower and upper bounds",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  5001 * time.Millisecond,
				},
			},
			samplingpolicy.Sampled,
		},
		{
			"trace duration is above upper bound",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  10001 * time.Millisecond,
				},
			},
			samplingpolicy.NotSampled,
		},
		{
			"trace duration equals upper bound",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  10000 * time.Millisecond,
				},
			},
			samplingpolicy.Sampled,
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
			samplingpolicy.Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			decision, err := filter.Evaluate(t.Context(), testTraceID, newTraceWithSpans(c.Spans))

			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

type spanWithTimeAndDuration struct {
	StartTime time.Time
	Duration  time.Duration
}

func newTraceWithSpans(spans []spanWithTimeAndDuration) *samplingpolicy.TraceData {
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

	return &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
}
