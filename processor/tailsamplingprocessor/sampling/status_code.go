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
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type statusCodeFilter struct {
	logger      *zap.Logger
	statusCodes []pdata.StatusCode
}

var _ PolicyEvaluator = (*statusCodeFilter)(nil)

// NewStatusCodeFilter creates a policy evaluator that samples all traces with
// a given status code.
func NewStatusCodeFilter(logger *zap.Logger, statusCodeString []string) (PolicyEvaluator, error) {
	if len(statusCodeString) == 0 {
		return nil, errors.New("expected at least one status code to filter on")
	}

	statusCodes := make([]pdata.StatusCode, len(statusCodeString))

	for i := range statusCodeString {
		switch statusCodeString[i] {
		case "OK":
			statusCodes[i] = pdata.StatusCodeOk
		case "ERROR":
			statusCodes[i] = pdata.StatusCodeError
		case "UNSET":
			statusCodes[i] = pdata.StatusCodeUnset
		default:
			return nil, fmt.Errorf("unknown status code %s, supported: OK, ERROR, UNSET", statusCodeString)
		}
	}

	return &statusCodeFilter{
		logger:      logger,
		statusCodes: statusCodes,
	}, nil
}

// OnLateArrivingSpans notifies the evaluator that the given list of spans arrived
// after the sampling decision was already taken for the trace.
// This gives the evaluator a chance to log any message/metrics and/or update any
// related internal state.
func (r *statusCodeFilter) OnLateArrivingSpans(Decision, []*pdata.Span) error {
	r.logger.Debug("Triggering action for late arriving spans in status code filter")
	return nil
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (r *statusCodeFilter) Evaluate(_ pdata.TraceID, trace *TraceData) (Decision, error) {
	r.logger.Debug("Evaluating spans in status code filter")

	trace.Lock()
	batches := trace.ReceivedBatches
	trace.Unlock()

	for _, batch := range batches {
		rspans := batch.ResourceSpans()

		for i := 0; i < rspans.Len(); i++ {
			rs := rspans.At(i)
			ilss := rs.InstrumentationLibrarySpans()

			for i := 0; i < ilss.Len(); i++ {
				ils := ilss.At(i)

				for j := 0; j < ils.Spans().Len(); j++ {
					span := ils.Spans().At(j)

					for _, statusCode := range r.statusCodes {
						if span.Status().Code() == statusCode {
							return Sampled, nil
						}
					}

				}
			}
		}
	}

	return NotSampled, nil
}
