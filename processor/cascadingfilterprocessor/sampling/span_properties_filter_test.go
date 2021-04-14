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
	"math"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

var (
	operationNamePattern = "foo.*"
	minDuration          = 500 * time.Microsecond
	minNumberOfSpans     = 2
)

func newSpanPropertiesFilter(operationNamePattern *string, minDuration *time.Duration, minNumberOfSpans *int) policyEvaluator {
	var operationRe *regexp.Regexp
	if operationNamePattern != nil {
		operationRe, _ = regexp.Compile(*operationNamePattern)
	}
	return policyEvaluator{
		logger:            zap.NewNop(),
		operationRe:       operationRe,
		minNumberOfSpans:  minNumberOfSpans,
		minDuration:       minDuration,
		maxSpansPerSecond: math.MaxInt64,
	}
}

func evaluate(t *testing.T, evaluator policyEvaluator, traces *TraceData, expectedDecision Decision) {
	u, _ := uuid.NewRandom()
	decision := evaluator.Evaluate(pdata.NewTraceID(u), traces)
	assert.Equal(t, expectedDecision, decision)
}

func TestPartialSpanPropertiesFilter(t *testing.T) {
	opFilter := newSpanPropertiesFilter(&operationNamePattern, nil, nil)
	durationFilter := newSpanPropertiesFilter(nil, &minDuration, nil)
	spansFilter := newSpanPropertiesFilter(nil, nil, &minNumberOfSpans)

	cases := []struct {
		Desc      string
		Evaluator policyEvaluator
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

	matchingTraces := newTraceAttrs("foobar", 1000*time.Microsecond, 100)
	nonMatchingTraces := newTraceAttrs("bar", 100*time.Microsecond, 1)

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			c.Evaluator.invertMatch = false
			evaluate(t, c.Evaluator, matchingTraces, Sampled)
			evaluate(t, c.Evaluator, nonMatchingTraces, NotSampled)

			c.Evaluator.invertMatch = true
			evaluate(t, c.Evaluator, matchingTraces, NotSampled)
			evaluate(t, c.Evaluator, nonMatchingTraces, Sampled)
		})
	}
}

func TestSpanPropertiesFilter(t *testing.T) {
	cases := []struct {
		Desc     string
		Trace    *TraceData
		Decision Decision
	}{
		{
			Desc:     "fully matching",
			Trace:    newTraceAttrs("foobar", 1000*time.Microsecond, 100),
			Decision: Sampled,
		},
		{
			Desc:     "nonmatching operation name",
			Trace:    newTraceAttrs("non_matching", 1000*time.Microsecond, 100),
			Decision: NotSampled,
		},
		{
			Desc:     "nonmatching duration",
			Trace:    newTraceAttrs("foobar", 100*time.Microsecond, 100),
			Decision: NotSampled,
		},
		{
			Desc:     "nonmatching number of spans",
			Trace:    newTraceAttrs("foobar", 1000*time.Microsecond, 1),
			Decision: NotSampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			// Regular match
			filter := newSpanPropertiesFilter(&operationNamePattern, &minDuration, &minNumberOfSpans)
			evaluate(t, filter, c.Trace, c.Decision)

			// Invert match
			filter.invertMatch = true
			invertDecision := Sampled
			if c.Decision == Sampled {
				invertDecision = NotSampled
			}
			evaluate(t, filter, c.Trace, invertDecision)
		})
	}
}

func newTraceAttrs(operationName string, duration time.Duration, numberOfSpans int) *TraceData {
	endTs := time.Now().UnixNano()
	startTs := endTs - duration.Nanoseconds()

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
