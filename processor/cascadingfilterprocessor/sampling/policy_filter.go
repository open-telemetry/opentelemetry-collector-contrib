// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling

import (
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
)

// OnLateArrivingSpans notifies the evaluator that the given list of spans arrived
// after the sampling decision was already taken for the trace.
// This gives the evaluator a chance to log any message/metrics and/or update any
// related internal state.
func (pe *policyEvaluator) OnLateArrivingSpans(earlyDecision Decision, spans []*pdata.Span) error {
	return nil
}

func tsToMicros(ts pdata.Timestamp) int64 {
	return int64(ts / 1000)
}

func checkIfNumericAttrFound(attrs pdata.AttributeMap, filter *numericAttributeFilter) bool {
	if v, ok := attrs.Get(filter.key); ok {
		value := v.IntVal()
		if value >= filter.minValue && value <= filter.maxValue {
			return true
		}
	}
	return false
}

func checkIfStringAttrFound(attrs pdata.AttributeMap, filter *stringAttributeFilter) bool {
	if v, ok := attrs.Get(filter.key); ok {
		truncableStr := v.StringVal()
		if len(truncableStr) > 0 {
			if _, ok := filter.values[truncableStr]; ok {
				return true
			}
		}
	}
	return false
}

// evaluateRules goes through the defined properties and checks if they are matched
func (pe *policyEvaluator) evaluateRules(_ pdata.TraceID, trace *TraceData) Decision {
	trace.Lock()
	batches := trace.ReceivedBatches
	trace.Unlock()

	matchingOperationFound := false
	matchingStringAttrFound := false
	matchingNumericAttrFound := false
	spanCount := 0
	minStartTime := int64(0)
	maxEndTime := int64(0)

	for _, batch := range batches {
		rs := batch.ResourceSpans()

		for i := 0; i < rs.Len(); i++ {
			if pe.stringAttr != nil || pe.numericAttr != nil {
				res := rs.At(i).Resource()
				if !matchingStringAttrFound && pe.stringAttr != nil {
					matchingStringAttrFound = checkIfStringAttrFound(res.Attributes(), pe.stringAttr)
				}
				if !matchingNumericAttrFound && pe.numericAttr != nil {
					matchingNumericAttrFound = checkIfNumericAttrFound(res.Attributes(), pe.numericAttr)
				}
			}

			ils := rs.At(i).InstrumentationLibrarySpans()
			for j := 0; j < ils.Len(); j++ {
				spans := ils.At(j).Spans()
				spanCount += spans.Len()
				for k := 0; k < spans.Len(); k++ {
					span := spans.At(k)

					if pe.stringAttr != nil || pe.numericAttr != nil {
						if !matchingStringAttrFound && pe.stringAttr != nil {
							matchingStringAttrFound = checkIfStringAttrFound(span.Attributes(), pe.stringAttr)
						}
						if !matchingNumericAttrFound && pe.numericAttr != nil {
							matchingNumericAttrFound = checkIfNumericAttrFound(span.Attributes(), pe.numericAttr)
						}
					}

					if pe.operationRe != nil && !matchingOperationFound {
						if pe.operationRe.MatchString(span.Name()) {
							matchingOperationFound = true
						}
					}

					if pe.minDuration != nil {
						startTs := tsToMicros(span.StartTimestamp())
						endTs := tsToMicros(span.EndTimestamp())

						if minStartTime == 0 {
							minStartTime = startTs
							maxEndTime = endTs
						} else {
							if startTs < minStartTime {
								minStartTime = startTs
							}
							if endTs > maxEndTime {
								maxEndTime = endTs
							}
						}
					}

				}
			}
		}
	}

	conditionMet := struct {
		operationName, minDuration, minSpanCount, stringAttr, numericAttr bool
	}{
		operationName: true,
		minDuration:   true,
		minSpanCount:  true,
		stringAttr:    true,
		numericAttr:   true,
	}

	if pe.operationRe != nil {
		conditionMet.operationName = matchingOperationFound
	}
	if pe.minNumberOfSpans != nil {
		conditionMet.minSpanCount = spanCount >= *pe.minNumberOfSpans
	}
	if pe.minDuration != nil {
		conditionMet.minDuration = maxEndTime > minStartTime && maxEndTime-minStartTime >= pe.minDuration.Microseconds()
	}
	if pe.numericAttr != nil {
		conditionMet.numericAttr = matchingNumericAttrFound
	}
	if pe.stringAttr != nil {
		conditionMet.stringAttr = matchingStringAttrFound
	}

	if conditionMet.minSpanCount &&
		conditionMet.minDuration &&
		conditionMet.operationName &&
		conditionMet.numericAttr &&
		conditionMet.stringAttr {
		if pe.invertMatch {
			return NotSampled
		}
		return Sampled
	}

	if pe.invertMatch {
		return Sampled
	}
	return NotSampled
}

func (pe *policyEvaluator) shouldConsider(currSecond int64, trace *TraceData) bool {
	if pe.maxSpansPerSecond < 0 {
		// This emits "second chance" traces
		return true
	} else if trace.SpanCount > pe.maxSpansPerSecond {
		// This trace will never fit, there are more spans than max limit
		return false
	} else if pe.currentSecond == currSecond && trace.SpanCount > pe.maxSpansPerSecond-pe.spansInCurrentSecond {
		// This trace will not fit in this second, no way
		return false
	} else {
		// This has some chances
		return true
	}
}

func (pe *policyEvaluator) emitsSecondChance() bool {
	return pe.maxSpansPerSecond < 0
}

func (pe *policyEvaluator) updateRate(currSecond int64, numSpans int64) Decision {
	if pe.currentSecond != currSecond {
		pe.currentSecond = currSecond
		pe.spansInCurrentSecond = 0
	}

	spansInSecondIfSampled := pe.spansInCurrentSecond + numSpans
	if spansInSecondIfSampled <= pe.maxSpansPerSecond {
		pe.spansInCurrentSecond = spansInSecondIfSampled
		return Sampled
	}

	return NotSampled
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision. Also takes into account
// the usage of sampling rate budget
func (pe *policyEvaluator) Evaluate(traceID pdata.TraceID, trace *TraceData) Decision {
	currSecond := time.Now().Unix()

	if !pe.shouldConsider(currSecond, trace) {
		return NotSampled
	}

	decision := pe.evaluateRules(traceID, trace)
	if decision != Sampled {
		return decision
	}

	if pe.emitsSecondChance() {
		return SecondChance
	}

	return pe.updateRate(currSecond, trace.SpanCount)
}
