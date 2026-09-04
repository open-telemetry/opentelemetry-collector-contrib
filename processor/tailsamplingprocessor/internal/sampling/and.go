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
	and := &And{
		subpolicies: wrapped,
		logger:      logger,
	}
	// Only advertise batch support when a sub-policy actually needs it
	// (e.g. a nested rate_limiting), so plain `and` policies are not
	// pulled into the processor's batch pre-pass.
	for _, sub := range wrapped {
		if _, ok := sub.(BatchEvaluator); ok {
			return &batchAnd{And: and}
		}
	}
	return and
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

// batchAnd is an And that contains at least one batch sub-policy (e.g. a
// rate_limiting policy). It only passes traces that match the non-batch
// sub-policies to the BatchEvaluator.
type batchAnd struct {
	*And
}

var (
	_ samplingpolicy.Evaluator = (*batchAnd)(nil)
	_ BatchEvaluator           = (*batchAnd)(nil)
)

// CalculateThreshold narrows the batch to traces that pass every non-batch
// sub-policy and forwards that subset to each nested batch sub-policy.
func (c *batchAnd) CalculateThreshold(ctx context.Context, batch []*samplingpolicy.TraceData) {
	candidates := make([]*samplingpolicy.TraceData, 0, len(batch))
	for _, td := range batch {
		if c.candidate(ctx, td) {
			candidates = append(candidates, td)
		}
	}
	for _, sub := range c.subpolicies {
		if be, ok := sub.(BatchEvaluator); ok {
			be.CalculateThreshold(ctx, candidates)
		}
	}
}

// candidate reports whether a trace passes every non-batch sub-policy, and
// is therefore eligible to be considered by the nested batch sub-policy.
func (c *batchAnd) candidate(ctx context.Context, td *samplingpolicy.TraceData) bool {
	id := traceIDOf(td)
	for _, sub := range c.subpolicies {
		if _, ok := sub.(BatchEvaluator); ok {
			// Decided by the batch pass below, not part of the filter.
			continue
		}
		d, err := sub.Evaluate(ctx, id, td)
		if err != nil {
			// Mirror And: a failing sub-policy means the trace is not
			// sampled, so it is not a candidate for the budget.
			return false
		}
		//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
		if d == samplingpolicy.NotSampled || d == samplingpolicy.InvertNotSampled {
			return false
		}
	}
	return true
}
