// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
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
	// If any subpolicy returns NotSampled, it returns NotSampled Decision.
	for _, sub := range c.subpolicies {
		decision, err := sub.Evaluate(ctx, traceID, trace)
		if err != nil {
			return Unspecified, err
		}
		if decision == NotSampled || decision == InvertNotSampled {
			return NotSampled, nil
		}

	}
	return Sampled, nil
}

// OnDroppedSpans is called when the trace needs to be dropped, due to memory
// pressure, before the decision_wait time has been reached.
func (c *And) OnDroppedSpans(pcommon.TraceID, *TraceData) (Decision, error) {
	return Sampled, nil
}
