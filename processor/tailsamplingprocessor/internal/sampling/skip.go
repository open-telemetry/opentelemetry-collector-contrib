// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type Skip struct {
	// the subpolicy evaluators
	subpolicies []PolicyEvaluator
}

func NewSkip(
	subpolicies []PolicyEvaluator,
) PolicyEvaluator {
	return &Skip{
		subpolicies: subpolicies,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (c *Skip) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error) {
	// The policy iterates over all sub-policies and returns Continue if all
	// sub-policies returned a NotSampled or InvertNotSampled Decision. If any subpolicy returns
	// Sampled or InvertSampled, it returns Skipped Decision.
	for _, sub := range c.subpolicies {
		decision, err := sub.Evaluate(ctx, traceID, trace)
		if err != nil {
			return Unspecified, err
		}
		if decision == NotSampled || decision == InvertNotSampled {
			return Continued, nil
		}
	}
	return Skipped, nil
}
