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

	// threshold is the cutoff from the most recent CalculateThreshold call.
	// Its zero value is AlwaysSampleThreshold, so a batch that fit entirely
	// within budget correctly imposes no restriction.
	//
	// Confined to the single decision-tick goroutine (batch sampling isn't
	// used in span-ingest mode), so no synchronization is needed.
	threshold sampling.Threshold
	// thresholdCalculated latches true on the first CalculateThreshold call
	// and never clears. A policy never reachable through the batch pre-pass
	// at all (nested inside composite, which doesn't support batching --
	// see the tracestate handling docs) stays false forever and falls back
	// to live per-trace token bucket admission instead.
	thresholdCalculated bool
}

// budgetItem is the per-trace data the batch sort works over, resolved
// once from each trace so randomness and cost aren't recomputed.
type budgetItem struct {
	randomness sampling.Randomness
	cost       int64
}

// newBudgetLimiter builds a limiter that refills tokensPerSecond tokens per
// second into a bucket holding at most burstCapacity, measuring each trace
// with cost. A single trace whose cost exceeds the burst capacity will not
// pass.
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
// highest-randomness traces in batch, spent from the top, exhaust the
// currently available budget, and draws their combined cost from the
// bucket in one call: every trace in batch with randomness at or above
// the result satisfies ShouldSample, and every trace below it does not.
func (b *budgetLimiter) CalculateThreshold(_ context.Context, batch []*samplingpolicy.TraceData) {
	b.thresholdCalculated = true
	now := time.Now()
	budget := int64(b.limiter.TokensAt(now))

	items := make([]budgetItem, len(batch))
	for i, td := range batch {
		items[i] = budgetItem{
			randomness: resolveRandomness(traceIDOf(td), td.ReceivedBatches),
			cost:       b.cost(td),
		}
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
		// A threshold can't separate traces with equal randomness, so if
		// the first dropped trace ties the last kept one, drop the whole
		// tied group -- keeps the invariant exact (kept: th <= R, dropped:
		// th > R) even though 56-bit ties are vanishingly unlikely.
		boundary := items[kept].randomness.Unsigned()
		for kept > 0 && items[kept-1].randomness.Unsigned() == boundary {
			kept--
			cumulative -= items[kept].cost
		}
		if kept == 0 {
			b.threshold = sampling.NeverSampleThreshold
		} else {
			// Error is unreachable: Randomness.Unsigned() is always in
			// [0, MaxAdjustedCount), the valid threshold range.
			b.threshold, _ = sampling.UnsignedToThreshold(items[kept-1].randomness.Unsigned())
		}
	}

	if cumulative > 0 {
		b.limiter.AllowN(now, int(cumulative))
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (b *budgetLimiter) Evaluate(ctx context.Context, id pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	d, _, err := b.EvaluateWithThreshold(ctx, id, trace)
	return d, err
}

// EvaluateWithThreshold compares this trace's own randomness against the
// threshold CalculateThreshold last computed; it does not draw from the
// bucket itself, since CalculateThreshold already reserved budget for
// every trace that will pass.
//
// A policy that has never been reached through the batch pre-pass at all
// (see thresholdCalculated) falls back to live per-trace token bucket
// admission instead, so it still limits something.
func (b *budgetLimiter) EvaluateWithThreshold(_ context.Context, id pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, sampling.Threshold, error) {
	if !b.thresholdCalculated {
		if b.limiter.AllowN(time.Now(), int(b.cost(trace))) {
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
