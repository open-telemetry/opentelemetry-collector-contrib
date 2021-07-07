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

import "go.opentelemetry.io/collector/model/pdata"

// hasResourceOrSpanWithCondition iterates through all the resources and instrumentation library spans until any
// callback returns true.
func hasResourceOrSpanWithCondition(
	batches []pdata.Traces,
	shouldSampleResource func(resource pdata.Resource) bool,
	shouldSampleSpan func(span pdata.Span) bool,
) Decision {
	for _, batch := range batches {
		rspans := batch.ResourceSpans()

		for i := 0; i < rspans.Len(); i++ {
			rs := rspans.At(i)

			resource := rs.Resource()
			if shouldSampleResource(resource) {
				return Sampled
			}

			if hasInstrumentationLibrarySpanWithCondition(rs.InstrumentationLibrarySpans(), shouldSampleSpan) {
				return Sampled
			}
		}
	}
	return NotSampled
}

// hasSpanWithCondition iterates through all the instrumentation library spans until any callback returns true.
func hasSpanWithCondition(batches []pdata.Traces, shouldSample func(span pdata.Span) bool) Decision {
	for _, batch := range batches {
		rspans := batch.ResourceSpans()

		for i := 0; i < rspans.Len(); i++ {
			rs := rspans.At(i)

			if hasInstrumentationLibrarySpanWithCondition(rs.InstrumentationLibrarySpans(), shouldSample) {
				return Sampled
			}
		}
	}
	return NotSampled
}

func hasInstrumentationLibrarySpanWithCondition(ilss pdata.InstrumentationLibrarySpansSlice, check func(span pdata.Span) bool) bool {
	for i := 0; i < ilss.Len(); i++ {
		ils := ilss.At(i)

		for j := 0; j < ils.Spans().Len(); j++ {
			span := ils.Spans().At(j)

			if check(span) {
				return true
			}
		}
	}
	return false
}
