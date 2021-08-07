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
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type probabilisticSampler struct {
	logger                *zap.Logger
	samplingRatio         float32
	includeAlreadySampled bool

	tracesSampled   int
	tracesProcessed int
}

var _ PolicyEvaluator = (*probabilisticSampler)(nil)

// NewProbabilisticSampler creates a policy evaluator that samples a percentage of
// traces.
func NewProbabilisticSampler(logger *zap.Logger, samplingPercentage float32, includeAlreadySampled bool) PolicyEvaluator {
	if samplingPercentage < 0 {
		samplingPercentage = 0
	}
	if samplingPercentage > 100 {
		samplingPercentage = 100
	}

	return &probabilisticSampler{
		logger:                logger,
		samplingRatio:         samplingPercentage / 100,
		includeAlreadySampled: includeAlreadySampled,
	}
}

// OnLateArrivingSpans notifies the evaluator that the given list of spans arrived
// after the sampling decision was already taken for the trace.
// This gives the evaluator a chance to log any message/metrics and/or update any
// related internal state.
func (s *probabilisticSampler) OnLateArrivingSpans(Decision, []*pdata.Span) error {
	s.logger.Debug("Triggering action for late arriving spans in probabilistic filter")
	return nil
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (s *probabilisticSampler) Evaluate(_ pdata.TraceID, trace *TraceData) (Decision, error) {
	s.logger.Debug("Evaluating spans in probabilistic filter")

	if hasSampledDecision(trace) {
		if s.includeAlreadySampled {
			s.tracesSampled++
			s.tracesProcessed++
		}
		return Sampled, nil
	}

	decision := NotSampled

	if float32(s.tracesSampled)/float32(s.tracesProcessed) <= s.samplingRatio {
		s.tracesSampled++
		decision = Sampled
	}
	s.tracesProcessed++

	// reset counters to avoid overflow
	if s.tracesProcessed == 1000 {
		s.tracesSampled = 0
		s.tracesProcessed = 0
	}

	return decision, nil
}

func hasSampledDecision(trace *TraceData) bool {
	for _, decision := range trace.Decisions {
		if decision == Sampled {
			return true
		}
	}
	return false
}
