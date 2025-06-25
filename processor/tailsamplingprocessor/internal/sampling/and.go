// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

type And struct {
	// the subpolicy evaluators
	subpolicies []PolicyEvaluator
	logger      *zap.Logger
}

func NewAnd(
	logger *zap.Logger,
	subpolicies []PolicyEvaluator,
) PolicyEvaluator {
	return &And{
		subpolicies: subpolicies,
		logger:      logger,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (c *And) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error) {
	// OTEP 250 AND logic: Collect threshold intents and return maximum (most restrictive) threshold
	// This implements proper threshold composition for intersection semantics
	var maxThreshold sampling.Threshold = sampling.AlwaysSampleThreshold // Start with 0 (most permissive)

	for _, sub := range c.subpolicies {
		decision, err := sub.Evaluate(ctx, traceID, trace)
		if err != nil {
			return NewDecisionWithError(err), err
		}

		// For OTEP 250 threshold composition, we work with raw threshold values
		// The "inverted" metadata is just bookkeeping - the threshold value is what matters
		policyThreshold := decision.Threshold

		// For AND logic, collect the maximum (most restrictive) threshold per OTEP 250
		// This ensures that only traces that ALL policies would sample get sampled
		if sampling.ThresholdGreater(policyThreshold, maxThreshold) {
			maxThreshold = policyThreshold
		}
	}

	// Update trace's final threshold using AND logic (maximum threshold)
	c.updateTraceThreshold(trace, maxThreshold)

	// Return the combined threshold intent - pure OTEP 250 approach
	// Final sampling decision will be made via randomness >= threshold comparison
	return NewDecisionWithThreshold(maxThreshold), nil
}

// updateTraceThreshold updates the trace's final threshold using AND logic (maximum threshold).
// For AND logic, we use the most restrictive (maximum) threshold per OTEP 250 AndOf semantics.
func (c *And) updateTraceThreshold(trace *TraceData, policyThreshold sampling.Threshold) {
	if trace.FinalThreshold == nil {
		// First policy to set a threshold
		trace.FinalThreshold = &policyThreshold
	} else {
		// Use the more restrictive (higher) threshold for AND logic
		if sampling.ThresholdGreater(policyThreshold, *trace.FinalThreshold) {
			trace.FinalThreshold = &policyThreshold
		}
	}
}
