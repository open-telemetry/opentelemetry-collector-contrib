// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func TestEvaluate_Latency(t *testing.T) {
	filter := NewLatency(zap.NewNop(), 5000)

	traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	now := time.Now()

	cases := []struct {
		Desc     string
		Spans    []spanWithTimeAndDuration
		Decision Decision
	}{
		{
			"trace duration shorter than threshold",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  4500 * time.Millisecond,
				},
			},
			NotSampled,
		},
		{
			"trace duration is equal to threshold",
			[]spanWithTimeAndDuration{
				{
					StartTime: now,
					Duration:  5000 * time.Millisecond,
				},
			},
			Sampled,
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
			Sampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			decision, err := filter.Evaluate(traceID, newTraceWithSpans(c.Spans))

			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func TestOnLateArrivingSpans_Latency(t *testing.T) {
	filter := NewLatency(zap.NewNop(), 5000)
	err := filter.OnLateArrivingSpans(NotSampled, nil)
	assert.Nil(t, err)
}

type spanWithTimeAndDuration struct {
	StartTime time.Time
	Duration  time.Duration
}

func newTraceWithSpans(spans []spanWithTimeAndDuration) *TraceData {
	var traceBatches []pdata.Traces
	traces := pdata.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.InstrumentationLibrarySpans().AppendEmpty()

	for _, s := range spans {
		span := ils.Spans().AppendEmpty()
		span.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
		span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
		span.SetStartTimestamp(pdata.NewTimestampFromTime(s.StartTime))
		span.SetEndTimestamp(pdata.NewTimestampFromTime(s.StartTime.Add(s.Duration)))
	}

	traceBatches = append(traceBatches, traces)
	return &TraceData{
		ReceivedBatches: traceBatches,
	}
}
