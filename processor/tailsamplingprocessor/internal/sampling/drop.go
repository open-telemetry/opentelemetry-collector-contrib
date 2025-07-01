// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

type Drop struct {
	// the subpolicy evaluators
	subpolicies []PolicyEvaluator
	logger      *zap.Logger
}

func NewDrop(
	logger *zap.Logger,
	subpolicies []PolicyEvaluator,
) PolicyEvaluator {
	return &Drop{
		subpolicies: subpolicies,
		logger:      logger,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (c *Drop) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error) {
	// The policy iterates over all sub-policies and returns Dropped if all
	// sub-policies returned a Sampled Decision. If any subpolicy returns
	// NotSampled, it returns NotSampled Decision.
	//
	// For OTEP 250 threshold combination, we use AND logic (maximum threshold)
	// since all policies must agree to sample before we drop.
	var maxThreshold sampling.Threshold = sampling.AlwaysSampleThreshold // Start with 0 (most permissive)

	for _, sub := range c.subpolicies {
		decision, err := sub.Evaluate(ctx, traceID, trace)
		if err != nil {
			return NewDecisionWithError(err), err
		}

		// For OTEP 250 threshold composition, collect the maximum threshold (most restrictive)
		policyThreshold := decision.Threshold
		if sampling.ThresholdGreater(policyThreshold, maxThreshold) {
			maxThreshold = policyThreshold
		}

		// Early exit: If any policy suggests not sampling (NeverSampleThreshold),
		// we can immediately return NotSampled
		if decision.Threshold == sampling.NeverSampleThreshold {
			return NewDecisionWithThreshold(sampling.NeverSampleThreshold), nil
		}
	}

	// All sub-policies agree to sample, so we drop the trace
	// "Dropped" means we explicitly reject a trace that would otherwise be sampled
	return NewDecisionWithThreshold(sampling.NeverSampleThreshold), nil
}
