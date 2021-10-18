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
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

var (
	operationNamePattern = "foo.*"
	minDuration          = 500 * time.Microsecond
	minNumberOfSpans     = 2
	minNumberOfErrors    = 1
)

func newSpanPropertiesFilter(t *testing.T, operationNamePattern *string, minDuration *time.Duration, minNumberOfSpans *int, minNumberOfErrors *int) policyEvaluator {
	var operationRe *regexp.Regexp
	var err error
	if operationNamePattern != nil {
		operationRe, err = regexp.Compile(*operationNamePattern)
		require.NoError(t, err)
	}
	return policyEvaluator{
		logger:            zap.NewNop(),
		operationRe:       operationRe,
		minNumberOfSpans:  minNumberOfSpans,
		minDuration:       minDuration,
		maxSpansPerSecond: math.MaxInt32,
		minNumberOfErrors: minNumberOfErrors,
	}
}

func evaluate(t *testing.T, evaluator policyEvaluator, traces *TraceData, expectedDecision Decision) {
	u, err := uuid.NewRandom()
	require.NoError(t, err)
	decision := evaluator.Evaluate(pdata.NewTraceID(u), traces)
	assert.Equal(t, expectedDecision, decision)
}

func TestPartialSpanPropertiesFilter(t *testing.T) {
	opFilter := newSpanPropertiesFilter(t, &operationNamePattern, nil, nil, nil)
	durationFilter := newSpanPropertiesFilter(t, nil, &minDuration, nil, nil)
	spansFilter := newSpanPropertiesFilter(t, nil, nil, &minNumberOfSpans, nil)
	errorsFilter := newSpanPropertiesFilter(t, nil, nil, nil, &minNumberOfErrors)

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
			Desc:      "number of spans filter",
			Evaluator: spansFilter,
		},
		{
			Desc:      "errors filter",
			Evaluator: errorsFilter,
		},
	}

	matchingTraces := newTraceAttrs("foobar", 1000*time.Microsecond, 100, 100)
	nonMatchingTraces := newTraceAttrs("bar", 100*time.Microsecond, 1, 0)

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
			Trace:    newTraceAttrs("foobar", 1000*time.Microsecond, 100, 100),
			Decision: Sampled,
		},
		{
			Desc:     "nonmatching operation name",
			Trace:    newTraceAttrs("non_matching", 1000*time.Microsecond, 100, 100),
			Decision: NotSampled,
		},
		{
			Desc:     "nonmatching duration",
			Trace:    newTraceAttrs("foobar", 100*time.Microsecond, 100, 100),
			Decision: NotSampled,
		},
		{
			Desc:     "nonmatching number of spans",
			Trace:    newTraceAttrs("foobar", 1000*time.Microsecond, 1, 1),
			Decision: NotSampled,
		},
		{
			Desc:     "nonmatching number of errors",
			Trace:    newTraceAttrs("foobar", 1000*time.Microsecond, 100, 0),
			Decision: NotSampled,
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			// Regular match
			filter := newSpanPropertiesFilter(t, &operationNamePattern, &minDuration, &minNumberOfSpans, &minNumberOfErrors)
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

func newTraceAttrs(operationName string, duration time.Duration, numberOfSpans int, numberOfErrors int) *TraceData {
	endTs := time.Now().UnixNano()
	startTs := endTs - duration.Nanoseconds()

	var traceBatches []pdata.Traces

	traces := pdata.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.InstrumentationLibrarySpans().AppendEmpty()

	spans := ils.Spans()
	spans.EnsureCapacity(numberOfSpans)

	for i := 0; i < numberOfSpans; i++ {
		span := spans.AppendEmpty()
		span.SetName(operationName)
		span.SetStartTimestamp(pdata.Timestamp(startTs))
		span.SetEndTimestamp(pdata.Timestamp(endTs))
	}

	for i := 0; i < numberOfErrors && i < numberOfSpans; i++ {
		span := spans.At(i)
		span.Status().SetCode(pdata.StatusCodeError)
	}

	traceBatches = append(traceBatches, traces)

	return &TraceData{
		ReceivedBatches: traceBatches,
	}
}
