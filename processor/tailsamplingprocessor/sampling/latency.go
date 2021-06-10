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
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type latency struct {
	logger      *zap.Logger
	thresholdMs int64
}

var _ PolicyEvaluator = (*latency)(nil)

// NewLatency creates a policy evaluator the samples all traces.
func NewLatency(logger *zap.Logger, thresholdMs int64) PolicyEvaluator {
	return &latency{
		logger:      logger,
		thresholdMs: thresholdMs,
	}
}

// OnLateArrivingSpans notifies the evaluator that the given list of spans arrived
// after the sampling decision was already taken for the trace.
// This gives the evaluator a chance to log any message/metrics and/or update any
// related internal state.
func (l *latency) OnLateArrivingSpans(Decision, []*pdata.Span) error {
	l.logger.Debug("Triggering action for late arriving spans in latency filter")
	return nil
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (l *latency) Evaluate(traceID pdata.TraceID, traceData *TraceData) (Decision, error) {
	l.logger.Debug("Evaluating spans in latency filter")

	traceData.Lock()
	batches := traceData.ReceivedBatches
	traceData.Unlock()

	for _, batch := range batches {
		rspans := batch.ResourceSpans()

		for i := 0; i < rspans.Len(); i++ {
			rs := rspans.At(i)
			ilss := rs.InstrumentationLibrarySpans()

			for i := 0; i < ilss.Len(); i++ {
				ils := ilss.At(i)

				for j := 0; j < ils.Spans().Len(); j++ {
					span := ils.Spans().At(j)

					startTime := span.StartTimestamp().AsTime()
					endTime := span.EndTimestamp().AsTime()

					duration := endTime.Sub(startTime)
					if duration.Milliseconds() >= l.thresholdMs {
						return Sampled, nil
					}
				}
			}
		}
	}
	return NotSampled, nil
}
