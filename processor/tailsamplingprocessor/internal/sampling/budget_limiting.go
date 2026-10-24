// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"sort"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

// BatchEvaluator is implemented by policies whose threshold depends on the
// whole group of traces eligible for a decision at once, rather than a
// single trace in isolation. rate_limiting is the canonical example: to
// report a consistent threshold it must sort the group by randomness and
// cut where the span budget runs out.
type BatchEvaluator interface {
	samplingpolicy.ThresholdEvaluator
	// CalculateThreshold calculates and stores the threshold to be used for the passed in batch.
	CalculateThreshold(ctx context.Context, batch []*samplingpolicy.TraceData)
}

// traceCostFunc reports how much of a limiter's budget a single trace
// consumes: spans for rate_limiting, protobuf-marshaled bytes for
// bytes_limiting.
type traceCostFunc func(*samplingpolicy.TraceData) int64

// traceSpanCount reports the number of spans in a trace, the unit the
// rate_limiting policy budgets in. bytes_limiting uses calculateTraceSize.
func traceSpanCount(trace *samplingpolicy.TraceData) int64 {
	return trace.SpanCount
}

// budgetLimiter is a token-bucket sampling algorithm that calculates a tracestate threshold cutoff for a batch of traces.
type budgetLimiter struct {
	limiter *rate.Limiter
	// cost reports the tokens a single trace consumes from the bucket.
	cost   traceCostFunc
	logger *zap.Logger

	threshold sampling.Threshold
	// thresholdCalculated controls if we have calculated the threshold and
	// therefore should use it in EvaluateWithThreshold. If the threshold was
	// never calculated then the budgetLimiter is used in a component that does
	// not support tracestate yet.
	thresholdCalculated bool
}

type budgetItem struct {
	randomness sampling.Randomness
	cost       int64
}

// newBudgetLimiter builds a limiter that refills tokensPerSecond tokens per
// second into a bucket holding at most burstCapacity, measuring each trace
// with cost.
func newBudgetLimiter(settings component.TelemetrySettings, tokensPerSecond, burstCapacity int64, cost traceCostFunc) *budgetLimiter {
	return &budgetLimiter{
		limiter: rate.NewLimiter(rate.Limit(tokensPerSecond), int(burstCapacity)),
		cost:    cost,
		logger:  settings.Logger,
	}
}

var (
	_ samplingpolicy.Evaluator          = (*budgetLimiter)(nil)
	_ samplingpolicy.ThresholdEvaluator = (*budgetLimiter)(nil)
	_ BatchEvaluator                    = (*budgetLimiter)(nil)
)

// CalculateThreshold sets the threshold at the point where the
// highest-randomness traces in batch, exhaust the currently available budget.
func (b *budgetLimiter) CalculateThreshold(_ context.Context, batch []*samplingpolicy.TraceData) {
	b.thresholdCalculated = true
	now := time.Now()
	budget := int64(b.limiter.TokensAt(now))
	burst := int64(b.limiter.Burst())

	items := make([]budgetItem, 0, len(batch))
	for _, td := range batch {
		cost := b.cost(td)
		if cost > burst {
			// Can never fit even a fully-refilled bucket; excluding it here
			// keeps it from starving traces that do fit (see EvaluateWithThreshold).
			continue
		}
		items = append(items, budgetItem{
			randomness: resolveRandomness(traceIDOf(td), td.ReceivedBatches),
			cost:       cost,
		})
	}

	// Sort by randomness descending: consistent sampling keeps the
	// traces with the highest randomness (threshold <= randomness), so
	// spending the budget from the top yields a single clean threshold.
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].randomness.Unsigned() > items[j].randomness.Unsigned()
	})

	var cumulative int64
	kept := len(items)
	for i, it := range items {
		if cumulative+it.cost > budget {
			kept = i
			break
		}
		cumulative += it.cost
	}

	switch {
	case kept == len(items):
		b.threshold = sampling.AlwaysSampleThreshold
	case kept == 0:
		b.threshold = sampling.NeverSampleThreshold
	default:
		// A threshold can't separate traces with equal randomness, so if the
		// first rejected trace ties the last kept one, drop the whole tied
		// group -- keeps the invariant exact (kept: th <= R, dropped: th > R)
		// even though 56-bit ties are vanishingly unlikely.
		firstRejected := items[kept].randomness.Unsigned()
		for kept > 0 && items[kept-1].randomness.Unsigned() == firstRejected {
			kept--
			cumulative -= items[kept].cost
		}
		if kept == 0 {
			b.threshold = sampling.NeverSampleThreshold
		} else {
			// Set the threshold to the priority (randomness) of the first
			// trace that causes the budget to be exceeded. See (Ting,
			// "Adaptive Threshold Sampling," §3.1: arxiv.org/abs/1708.04970).
			// Note that if the per second budget is not double the maximum
			// trace cost then future extrapolation can break down. The +1 is
			// because we sample on threshold <= randomness so we need to move
			// the position by 1.
			b.threshold, _ = sampling.UnsignedToThreshold(firstRejected + 1)
		}
	}

	if cumulative > 0 {
		// Update the limiter here as we have now decided which traces will be
		// sampled. This avoids ordering being necessary when calling
		// EvaluateWithThreshold alongside other policies in an and.
		b.limiter.AllowN(now, int(cumulative))
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (b *budgetLimiter) Evaluate(ctx context.Context, id pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	d, _, err := b.EvaluateWithThreshold(ctx, id, trace)
	return d, err
}

// EvaluateWithThreshold compares this trace's own randomness against the
// threshold CalculateThreshold last computed.
//
// A policy that has never does not calculate the threshold falls back to live
// per-trace token bucket admission.
func (b *budgetLimiter) EvaluateWithThreshold(_ context.Context, id pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, sampling.Threshold, error) {
	cost := b.cost(trace)
	if cost > int64(b.limiter.Burst()) {
		// Never fits, regardless of randomness or threshold: CalculateThreshold
		// excludes it from the batch, so the reported threshold says nothing
		// about it either way.
		return samplingpolicy.NotSampled, sampling.AlwaysSampleThreshold, nil
	}
	if !b.thresholdCalculated {
		if b.limiter.AllowN(time.Now(), int(cost)) {
			return samplingpolicy.Sampled, sampling.AlwaysSampleThreshold, nil
		}
		return samplingpolicy.NotSampled, sampling.AlwaysSampleThreshold, nil
	}

	randomness := resolveRandomness(id, trace.ReceivedBatches)
	if b.threshold.ShouldSample(randomness) {
		return samplingpolicy.Sampled, b.threshold, nil
	}
	return samplingpolicy.NotSampled, sampling.AlwaysSampleThreshold, nil
}

func (*budgetLimiter) IsStateful() bool {
	return true
}
