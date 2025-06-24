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
	// The policy iterates over all sub-policies and returns Sampled if all sub-policies returned a Sampled Decision.
	// If any subpolicy returns NotSampled or InvertNotSampled, it returns NotSampled Decision.
	var maxThreshold sampling.Threshold = sampling.AlwaysSampleThreshold // Start with 0 (most permissive)
	allSampled := true

	for _, sub := range c.subpolicies {
		decision, err := sub.Evaluate(ctx, traceID, trace)
		if err != nil {
			return NewDecisionWithError(err), err
		}

		// Check if this decision represents sampling
		if !decision.IsSampled() {
			allSampled = false
			// Don't return early - continue evaluating all policies to collect thresholds
		} else {
			// For AND logic, collect the maximum (most restrictive) threshold per OTEP 250
			if sampling.ThresholdGreater(decision.Threshold, maxThreshold) {
				maxThreshold = decision.Threshold
			}
		}
	}

	if allSampled {
		// For AND logic, we want the most restrictive (maximum) threshold per OTEP 250
		// This ensures that only traces that ALL policies would sample get sampled
		c.updateTraceThreshold(trace, maxThreshold)

		// For backward compatibility, return the Sampled constant if threshold equals AlwaysSampleThreshold
		if maxThreshold == sampling.AlwaysSampleThreshold {
			return Sampled, nil
		}
		return NewDecisionWithThreshold(maxThreshold), nil
	}

	return NotSampled, nil
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
