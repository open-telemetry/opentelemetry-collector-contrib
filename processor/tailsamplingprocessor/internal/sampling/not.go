// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

type not struct {
	logger             *zap.Logger
	subPolicyEvaluator samplingpolicy.Evaluator
}

var _ samplingpolicy.Evaluator = (*not)(nil)

// NewNot creates a new not policy evaluator returns the opposite of the decision of the wrapped policy.
func NewNot(logger *zap.Logger, subPolicyEvaluator samplingpolicy.Evaluator) samplingpolicy.Evaluator {
	return &not{
		logger:             logger,
		subPolicyEvaluator: subPolicyEvaluator,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingResult.
// The not policy return the opposite of the decision of the wrapped policy.
func (n *not) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	decision, err := n.subPolicyEvaluator.Evaluate(ctx, traceID, trace)
	if err != nil {
		n.logger.Debug("Evaluation error from sub-policy", zap.Error(err))
		return decision, err
	}

	// Return the opposite of the decision
	switch decision {
	case samplingpolicy.Sampled:
		return samplingpolicy.NotSampled, nil
	case samplingpolicy.NotSampled:
		return samplingpolicy.Sampled, nil
	default:
		// For any other decision types (Unspecified, Pending, Error), just let them bubble up.
		return decision, nil
	}
}
