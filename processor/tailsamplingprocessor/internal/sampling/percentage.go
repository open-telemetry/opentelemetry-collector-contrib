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

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type percentageFilter struct {
	logger          *zap.Logger
	percentage      float32
	tracesSampled   int
	tracesProcessed int
}

var _ PolicyEvaluator = (*percentageFilter)(nil)

// NewPercentageFilter creates a policy evaluator that samples a percentage of
// traces.
func NewPercentageFilter(logger *zap.Logger, percentage float32) (PolicyEvaluator, error) {
	if percentage < 0 || percentage > 1 {
		return nil, errors.New("expected a percentage between 0 and 1")
	}

	return &percentageFilter{
		logger:     logger,
		percentage: percentage,
	}, nil
}

// OnLateArrivingSpans notifies the evaluator that the given list of spans arrived
// after the sampling decision was already taken for the trace.
// This gives the evaluator a chance to log any message/metrics and/or update any
// related internal state.
func (r *percentageFilter) OnLateArrivingSpans(Decision, []*pdata.Span) error {
	r.logger.Debug("Triggering action for late arriving spans in percentage filter")
	return nil
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (r *percentageFilter) Evaluate(_ pdata.TraceID, trace *TraceData) (Decision, error) {
	r.logger.Debug("Evaluating spans in percentage filter")

	// ignore traces that have already been sampled before
	for _, decision := range trace.Decisions {
		if decision == Sampled {
			return NotSampled, nil
		}
	}

	decision := NotSampled

	if float32(r.tracesSampled)/float32(r.tracesProcessed) <= r.percentage {
		r.tracesSampled++
		decision = Sampled
	}
	r.tracesProcessed++

	// reset counters to avoid overflow
	if r.tracesProcessed == 1000 {
		r.tracesSampled = 0
		r.tracesProcessed = 0
	}

	return decision, nil
}
