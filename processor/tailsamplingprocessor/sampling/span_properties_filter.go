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
	"errors"
	"regexp"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type spanPropertiesFilter struct {
	operationRe       *regexp.Regexp
	minDurationMicros *int64
	minNumberOfSpans  *int
	logger            *zap.Logger
}

var _ PolicyEvaluator = (*spanPropertiesFilter)(nil)

// NewSpanPropertiesFilter creates a policy evaluator that samples all traces with
// the specified criteria
func NewSpanPropertiesFilter(logger *zap.Logger, operationNamePattern *string, minDurationMicros *int64, minNumberOfSpans *int) (PolicyEvaluator, error) {
	var operationRe *regexp.Regexp
	var err error

	if operationNamePattern == nil && minDurationMicros == nil && minNumberOfSpans == nil {
		return nil, errors.New("at least one property must be defined")
	}

	if operationNamePattern != nil {
		operationRe, err = regexp.Compile(*operationNamePattern)
		if err != nil {
			return nil, err
		}
	}

	if minDurationMicros != nil && *minDurationMicros < int64(0) {
		return nil, errors.New("minimum span duration must be a non-negative number")
	}

	if minNumberOfSpans != nil && *minNumberOfSpans < 1 {
		return nil, errors.New("minimum number of spans must be a positive number")
	}

	return &spanPropertiesFilter{
		operationRe:       operationRe,
		minDurationMicros: minDurationMicros,
		minNumberOfSpans:  minNumberOfSpans,
		logger:            logger,
	}, nil
}

// OnLateArrivingSpans notifies the evaluator that the given list of spans arrived
// after the sampling decision was already taken for the trace.
// This gives the evaluator a chance to log any message/metrics and/or update any
// related internal state.
func (df *spanPropertiesFilter) OnLateArrivingSpans(earlyDecision Decision, spans []*pdata.Span) error {
	return nil
}

// EvaluateSecondChance looks at the trace again and if it can/cannot be fit, returns a SamplingDecision
func (df *spanPropertiesFilter) EvaluateSecondChance(_ pdata.TraceID, trace *TraceData) (Decision, error) {
	return NotSampled, nil
}

func tsToMicros(ts pdata.Timestamp) int64 {
	return int64(ts / 1000)
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (df *spanPropertiesFilter) Evaluate(_ pdata.TraceID, trace *TraceData) (Decision, error) {
	trace.Lock()
	batches := trace.ReceivedBatches
	trace.Unlock()

	matchingOperationFound := false
	spanCount := 0
	minStartTime := int64(0)
	maxEndTime := int64(0)

	for _, batch := range batches {
		rs := batch.ResourceSpans()

		for i := 0; i < rs.Len(); i++ {
			ils := rs.At(i).InstrumentationLibrarySpans()
			for j := 0; j < ils.Len(); j++ {
				spans := ils.At(j).Spans()
				spanCount += spans.Len()
				for k := 0; k < spans.Len(); k++ {
					span := spans.At(k)

					if df.operationRe != nil && !matchingOperationFound {
						if df.operationRe.MatchString(span.Name()) {
							matchingOperationFound = true
						}
					}

					if df.minDurationMicros != nil {
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

	var operationNameConditionMet, minDurationConditionMet, minSpanCountConditionMet bool

	if df.operationRe != nil {
		operationNameConditionMet = matchingOperationFound
	} else {
		operationNameConditionMet = true
	}

	if df.minDurationMicros != nil {
		// Sanity check first
		minDurationConditionMet = maxEndTime > minStartTime && maxEndTime-minStartTime >= *df.minDurationMicros
	} else {
		minDurationConditionMet = true
	}

	if df.minNumberOfSpans != nil {
		minSpanCountConditionMet = spanCount >= *df.minNumberOfSpans
	} else {
		minSpanCountConditionMet = true
	}

	if minDurationConditionMet && operationNameConditionMet && minSpanCountConditionMet {
		return Sampled, nil
	}

	return NotSampled, nil
}
