// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

type probabilisticSampler struct {
	logger              *zap.Logger
	samplingProbability float64
	threshold           sampling.Threshold
}

var _ PolicyEvaluator = (*probabilisticSampler)(nil)

// NewProbabilisticSampler creates a policy evaluator that samples a percentage of
// traces using OTEP 235 consistent probability sampling.
func NewProbabilisticSampler(settings component.TelemetrySettings, hashSalt string, samplingPercentage float64) PolicyEvaluator {
	// Convert percentage to probability (0.0 to 1.0)
	probability := samplingPercentage / 100

	// Handle edge cases first
	var threshold sampling.Threshold
	if probability <= 0 {
		// 0% or negative percentage should never sample
		threshold = sampling.NeverSampleThreshold
	} else if probability >= 1 {
		// 100% or higher percentage should always sample
		threshold = sampling.AlwaysSampleThreshold
	} else {
		// Convert probability to OTEP 235 threshold
		var err error
		threshold, err = sampling.ProbabilityToThreshold(probability)
		if err != nil {
			settings.Logger.Error("Invalid sampling probability, using never sample",
				zap.Float64("probability", probability), zap.Error(err))
			threshold = sampling.NeverSampleThreshold
		}
	}

	return &probabilisticSampler{
		logger:              settings.Logger,
		samplingProbability: probability,
		threshold:           threshold,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision
// using OTEP 235 threshold-based sampling for consistency.
func (s *probabilisticSampler) Evaluate(_ context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error) {
	s.logger.Debug("Evaluating trace in probabilistic filter using OTEP 235 threshold")

	// Return the threshold intent directly - the processor will handle final decision
	return Decision{Threshold: s.threshold}, nil
}
