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

	// Use the randomness value already extracted in TraceData (from TraceState rv or TraceID)
	randomness := trace.RandomnessValue

	// Use pkg/sampling for consistent OTEP 235 decision
	if s.threshold.ShouldSample(randomness) {
		// Update trace threshold to reflect this sampling decision
		s.updateTraceThreshold(trace, s.threshold)
		return Sampled, nil
	}

	return NotSampled, nil
}

// updateTraceThreshold updates the trace's final threshold using OR logic (minimum threshold).
// Since the tail sampler uses OR logic (any policy can trigger sampling), 
// we use the least restrictive (minimum) threshold per OTEP 250 AnyOf semantics.
func (s *probabilisticSampler) updateTraceThreshold(trace *TraceData, policyThreshold sampling.Threshold) {
	if trace.FinalThreshold == nil {
		// First policy to set a threshold
		trace.FinalThreshold = &policyThreshold
	} else {
		// Use the less restrictive (lower) threshold for OR logic
		if sampling.ThresholdGreater(*trace.FinalThreshold, policyThreshold) {
			trace.FinalThreshold = &policyThreshold
		}
	}
}
