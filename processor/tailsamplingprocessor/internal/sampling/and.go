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
	var subDecisions []SubPolicyDecision

	for i, sub := range c.subpolicies {
		decision, err := sub.Evaluate(ctx, traceID, trace)
		if err != nil {
			return NewDecisionWithError(err), err
		}

		// Store sub-policy decision for deferred attribute application
		subDecisions = append(subDecisions, SubPolicyDecision{
			Threshold:         decision.Threshold,
			AttributeInserter: CombineAttributeInserterFuncs(decision.AttributeInserters...),
			PolicyName:        c.logger.Name() + ".sub" + string(rune(i)), // For debugging
		})

		// For AND logic, collect the maximum (most restrictive) threshold per OTEP 250
		// This ensures that only traces that ALL policies would sample get sampled
		if sampling.ThresholdGreater(decision.Threshold, maxThreshold) {
			maxThreshold = decision.Threshold
		}
	}

	// Update trace's final threshold using AND logic (maximum threshold)
	c.updateTraceThreshold(trace, maxThreshold)

	// Create deferred attribute inserter that only applies attributes from
	// sub-policies that would actually sample given the final randomness value
	deferredInserter := NewDeferredAttributeInserter(subDecisions, "AND")

	// Return decision with threshold and deferred attributes
	// For consistency with simple policies, return attributes that match the sampling decision pattern
	var attributes map[string]any
	if maxThreshold == sampling.AlwaysSampleThreshold {
		attributes = map[string]any{"sampled": true}
	} else {
		attributes = make(map[string]any) // Empty but non-nil for consistency
	}

	return Decision{
		Threshold:          maxThreshold,
		Attributes:         attributes,
		AttributeInserters: []AttributeInserter{deferredInserter},
	}, nil
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
