// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
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
	for _, sub := range c.subpolicies {
		decision, err := sub.Evaluate(ctx, traceID, trace)
		if err != nil {
			return Unspecified, err
		}
		if decision == NotSampled {
			return NotSampled, nil
		}
	}
	return Dropped, nil
}
