// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

type And struct {
	// the subpolicy evaluators
	subpolicies []samplingpolicy.ThresholdEvaluator
	logger      *zap.Logger
}

var (
	_ samplingpolicy.Evaluator          = (*And)(nil)
	_ samplingpolicy.ThresholdEvaluator = (*And)(nil)
)

func NewAnd(
	logger *zap.Logger,
	subpolicies []samplingpolicy.Evaluator,
) samplingpolicy.Evaluator {
	wrapped := make([]samplingpolicy.ThresholdEvaluator, len(subpolicies))
	for i, sub := range subpolicies {
		wrapped[i] = samplingpolicy.AsThresholdEvaluator(sub)
	}
	return &And{
		subpolicies: wrapped,
		logger:      logger,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (c *And) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	d, _, err := c.EvaluateWithThreshold(ctx, traceID, trace)
	return d, err
}

// EvaluateWithThreshold returns Sampled iff all sub-policies return
// Sampled. The reported threshold is the most aggressive (largest)
// threshold across sub-policies, because a trace passes AND iff it
// passes the most aggressive sub-policy.
func (c *And) EvaluateWithThreshold(ctx context.Context, traceID pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, sampling.Threshold, error) {
	threshold := sampling.AlwaysSampleThreshold
	for _, sub := range c.subpolicies {
		d, subTh, err := sub.EvaluateWithThreshold(ctx, traceID, trace)
		if err != nil {
			return samplingpolicy.Unspecified, sampling.AlwaysSampleThreshold, err
		}
		//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
		if d == samplingpolicy.NotSampled || d == samplingpolicy.InvertNotSampled {
			return samplingpolicy.NotSampled, sampling.AlwaysSampleThreshold, nil
		}
		if sampling.ThresholdGreater(subTh, threshold) {
			threshold = subTh
		}
	}
	return samplingpolicy.Sampled, threshold, nil
}

func (c *And) IsStateful() bool {
	for _, sub := range c.subpolicies {
		if sub.IsStateful() {
			return true
		}
	}
	return false
}
