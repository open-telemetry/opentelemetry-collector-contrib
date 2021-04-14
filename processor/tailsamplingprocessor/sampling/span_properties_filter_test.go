// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

var operationNamePattern = "foo.*"
var minDurationMicros = int64(500)
var minNumberOfSpans = 2

func TestInvalidArguments(t *testing.T) {
	_, err := NewSpanPropertiesFilter(zap.NewNop(), nil, nil, nil)
	assert.Error(t, err)
}

func TestPartialSpanPropertiesFilter(t *testing.T) {
	opFilter, _ := NewSpanPropertiesFilter(zap.NewNop(), &operationNamePattern, nil, nil)
	durationFilter, _ := NewSpanPropertiesFilter(zap.NewNop(), nil, &minDurationMicros, nil)
	spansFilter, _ := NewSpanPropertiesFilter(zap.NewNop(), nil, nil, &minNumberOfSpans)

	cases := []struct {
		Desc      string
		Evaluator PolicyEvaluator
	}{
		{
			Desc:      "operation name filter",
			Evaluator: opFilter,
		},
		{
			Desc:      "duration filter",
			Evaluator: durationFilter,
		},
		{
			Desc:      "spans filter",
			Evaluator: spansFilter,
		},
	}

	matchingTraces := newTraceAttrs("foobar", 1000, 100)
	nonMatchingTraces := newTraceAttrs("bar", 100, 1)

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			u, _ := uuid.NewRandom()
			decision, err := c.Evaluator.Evaluate(pdata.NewTraceID(u), matchingTraces)
			assert.NoError(t, err)
			assert.Equal(t, decision, Sampled)

			u, _ = uuid.NewRandom()
			decision, err = c.Evaluator.Evaluate(pdata.NewTraceID(u), nonMatchingTraces)
			assert.NoError(t, err)
			assert.Equal(t, decision, NotSampled)
		})
	}
}

func TestSpanPropertiesFilter(t *testing.T) {
	filter, _ := NewSpanPropertiesFilter(zap.NewNop(), &operationNamePattern, &minDurationMicros, &minNumberOfSpans)

	cases := []struct {
		Desc     string
		Trace    *TraceData
		Decision Decision
	}{
		{
			Desc:     "fully matching",
			Trace:    newTraceAttrs("foobar", 1000, 100),
			Decision: Sampled,
		},
		{
			Desc:     "nonmatching operation name",
			Trace:    newTraceAttrs("non_matching", 1000, 100),
			Decision: NotSampled,
		},
		{
			Desc:     "nonmatching duration",
			Trace:    newTraceAttrs("foobar", 100, 100),
			Decision: NotSampled,
		},
		{
			Desc:     "nonmatching number of spans",
			Trace:    newTraceAttrs("foobar", 1000, 1),
			Decision: NotSampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			u, _ := uuid.NewRandom()
			decision, err := filter.Evaluate(pdata.NewTraceID(u), c.Trace)
			assert.NoError(t, err)
			assert.Equal(t, decision, c.Decision)
		})
	}
}

func newTraceAttrs(operationName string, durationMicros int64, numberOfSpans int) *TraceData {
	endTs := time.Now().UnixNano()
	startTs := endTs - durationMicros*1000

	var traceBatches []pdata.Traces

	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	rs := traces.ResourceSpans().At(0)
	rs.InstrumentationLibrarySpans().Resize(1)
	ils := rs.InstrumentationLibrarySpans().At(0)

	ils.Spans().Resize(numberOfSpans)

	for i := 0; i < numberOfSpans; i++ {
		span := ils.Spans().At(i)
		span.SetName(operationName)
		span.SetStartTimestamp(pdata.Timestamp(startTs))
		span.SetEndTimestamp(pdata.Timestamp(endTs))
	}

	traceBatches = append(traceBatches, traces)

	return &TraceData{
		ReceivedBatches: traceBatches,
	}
}
