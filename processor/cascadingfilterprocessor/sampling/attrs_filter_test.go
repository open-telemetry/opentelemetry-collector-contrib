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

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func newAttrsFilter(filters []attributeFilter) policyEvaluator {
	return policyEvaluator{
		logger:            zap.NewNop(),
		attrs:             filters,
		maxSpansPerSecond: math.MaxInt32,
	}
}

func newAttrFilter(key string, regexValues []string, ranges []attributeRange) attributeFilter {
	var patterns []*regexp.Regexp
	for _, value := range regexValues {
		re := regexp.MustCompile(value)
		patterns = append(patterns, re)
	}

	return attributeFilter{
		key:      key,
		values:   nil,
		patterns: patterns,
		ranges:   ranges,
	}
}

func TestAttributesFilter(t *testing.T) {
	filterFooPattern := newAttrFilter("foo", []string{"foob.*"}, nil)
	filterBarPattern := newAttrFilter("bar", []string{"baz.*"}, nil)
	filterCooNothing := newAttrFilter("coo", nil, nil)
	filterFooRange := newAttrFilter("foo", nil, []attributeRange{{minValue: 100, maxValue: 150}})
	filterFooRangesOrPatterns := newAttrFilter("foo", []string{"foo.*", "claz.*"}, []attributeRange{{minValue: 100, maxValue: 150}, {minValue: 200, maxValue: 250}})

	composite := newAttrsFilter([]attributeFilter{filterFooRangesOrPatterns, filterBarPattern})
	bar := newAttrsFilter([]attributeFilter{filterBarPattern})
	fooRange := newAttrsFilter([]attributeFilter{filterFooRange})
	fooPattern := newAttrsFilter([]attributeFilter{filterFooPattern})
	coo := newAttrsFilter([]attributeFilter{filterCooNothing})

	fooTraces, fooAttrs := newTrace()
	fooAttrs.InsertString("foo", "foobar")

	fooNumTraces, fooNumAttrs := newTrace()
	fooNumAttrs.InsertInt("foo", 130)

	fooBarTraces, fooBarAttrs := newTrace()
	fooBarAttrs.InsertString("foo", "foobar")
	fooBarAttrs.InsertString("bar", "bazbar")

	booTraces, booAttrs := newTrace()
	booAttrs.InsertString("bar", "bazboo")

	cooTraces, cooAttrs := newTrace()
	cooAttrs.InsertString("coo", "fsdkfjsdkljsda")

	cases := []struct {
		Desc      string
		Evaluator policyEvaluator
		Match     []*TraceData
		DontMatch []*TraceData
	}{
		{
			Desc:      "simple string pattern",
			Evaluator: fooPattern,
			Match:     []*TraceData{fooTraces, fooBarTraces},
			DontMatch: []*TraceData{fooNumTraces, booTraces, cooTraces},
		},
		{
			Desc:      "simple numeric ranges",
			Evaluator: fooRange,
			Match:     []*TraceData{fooNumTraces},
			DontMatch: []*TraceData{fooTraces, fooBarTraces, booTraces, cooTraces},
		},
		{
			Desc:      "simple pattern",
			Evaluator: bar,
			Match:     []*TraceData{fooBarTraces, booTraces},
			DontMatch: []*TraceData{fooTraces, fooNumTraces},
		},
		{
			Desc:      "composite",
			Evaluator: composite,
			Match:     []*TraceData{fooBarTraces},
			DontMatch: []*TraceData{fooTraces, fooNumTraces, booTraces, cooTraces},
		},
		{
			Desc:      "no pattern, just existence of key",
			Evaluator: coo,
			Match:     []*TraceData{cooTraces},
			DontMatch: []*TraceData{fooTraces, fooNumTraces, fooBarTraces, booTraces},
		},
	}

	for _, c := range cases {
		t.Run(c.Desc, func(t *testing.T) {
			for _, traces := range c.Match {
				c.Evaluator.invertMatch = false
				evaluate(t, c.Evaluator, traces, Sampled)
				c.Evaluator.invertMatch = true
				evaluate(t, c.Evaluator, traces, NotSampled)
			}
			for _, traces := range c.DontMatch {
				c.Evaluator.invertMatch = false
				evaluate(t, c.Evaluator, traces, NotSampled)
				c.Evaluator.invertMatch = true
				evaluate(t, c.Evaluator, traces, Sampled)
			}
		})
	}
}

func newTrace() (*TraceData, pdata.AttributeMap) {
	endTs := time.Now().UnixNano()
	startTs := endTs - 100000

	var traceBatches []pdata.Traces

	traces := pdata.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.InstrumentationLibrarySpans().AppendEmpty()

	spans := ils.Spans()
	spans.EnsureCapacity(1)

	span := spans.AppendEmpty()
	span.SetName("fooname")
	span.SetStartTimestamp(pdata.Timestamp(startTs))
	span.SetEndTimestamp(pdata.Timestamp(endTs))

	traceBatches = append(traceBatches, traces)

	return &TraceData{
		ReceivedBatches: traceBatches,
	}, span.Attributes()
}
