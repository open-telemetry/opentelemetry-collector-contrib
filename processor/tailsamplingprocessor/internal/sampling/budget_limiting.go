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

// traceCostFunc reports how much of a limiter's budget a single trace
// consumes: spans for rate_limiting, protobuf-marshaled bytes for
// bytes_limiting.
type traceCostFunc func(*samplingpolicy.TraceData) int64

// traceSpanCount reports the number of spans in a trace, the unit the
// rate_limiting policy budgets in. bytes_limiting uses calculateTraceSize.
func traceSpanCount(trace *samplingpolicy.TraceData) int64 {
	return trace.SpanCount
}

// budgetLimiter is the token-bucket sampling shared by the rate_limiting and
// bytes_limiting policies. Both spend a per-second budget across the traces
// eligible for a decision and differ only in what a token measures, so the
// batch machinery is parameterized by a cost function.
//
// CalculateThreshold computes one threshold from the traces that will
// actually reach this policy this tick; EvaluateWithThreshold then just
// compares a trace's own randomness against it, in the normal per-trace
// evaluation loop. A trace a higher-priority policy already decided (a drop
// policy, or an earlier match under sample_on_first_match) never reaches
// EvaluateWithThreshold, so it correctly never spends budget -- callers must
// exclude such traces from the batch for the same reason.
//
// Only constructed when the usetracestate feature gate is enabled; with the
// gate disabled the limiting policies stay on their original per-trace
// token bucket, untouched.
type budgetLimiter struct {
	// Token bucket implemented by golang.org/x/time/rate.
	limiter *rate.Limiter
	// cost reports the tokens a single trace consumes from the bucket.
	cost   traceCostFunc
	logger *zap.Logger
	// logMsg is recorded at debug level on each per-trace evaluation.
	logMsg string

	// threshold is the cutoff from the most recent CalculateThreshold call.
	// Its zero value is AlwaysSampleThreshold, so a batch that fit entirely
	// within budget correctly imposes no restriction.
	//
	// Confined to the single decision-tick goroutine (batch sampling isn't
	// used in span-ingest mode), so no synchronization is needed.
	threshold sampling.Threshold
	// primed latches true on the first CalculateThreshold call and never
	// clears. A policy never reachable through the batch pre-pass at all
	// (nested inside composite, which doesn't support batching -- see the
	// tracestate handling docs) stays unprimed and falls back to live
	// per-trace token bucket admission instead.
	primed bool
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
func newBudgetLimiter(settings component.TelemetrySettings, tokensPerSecond, burstCapacity int64, cost traceCostFunc, logMsg string) *budgetLimiter {
	return &budgetLimiter{
		limiter: rate.NewLimiter(rate.Limit(tokensPerSecond), int(burstCapacity)),
		cost:    cost,
		logger:  settings.Logger,
		logMsg:  logMsg,
	}
}

var (
	_ samplingpolicy.Evaluator          = (*budgetLimiter)(nil)
	_ samplingpolicy.ThresholdEvaluator = (*budgetLimiter)(nil)
	_ samplingpolicy.BatchEvaluator     = (*budgetLimiter)(nil)
)

// CalculateThreshold spends the currently available budget on the
// highest-randomness traces in batch first and sets the threshold at the
// point where the budget runs out: every trace in batch with randomness at
// or above the result satisfies ShouldSample, and every trace below it does
// not. It decides nothing itself and returns nothing; EvaluateWithThreshold
// applies the result afterward.
func (b *budgetLimiter) CalculateThreshold(_ context.Context, batch []*samplingpolicy.TraceData) {
	b.primed = true
	now := time.Now()
	// The token bucket's current fill is the budget for this batch. Reading
	// it here (rather than a fixed rate*interval) preserves the existing
	// continuous-refill and burst behavior across ticks.
	budget := int64(b.limiter.TokensAt(now))

	// Resolve each trace's randomness and cost once.
	items := make([]budgetItem, len(batch))
	for i, td := range batch {
		items[i] = budgetItem{
			randomness: ResolveRandomness(td.TraceID(), td.ReceivedBatches),
			cost:       b.cost(td),
		}
	}

	// Sort by randomness descending: consistent sampling keeps the
	// traces with the highest randomness (threshold <= randomness), so
	// spending the budget from the top yields a single clean threshold.
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].randomness.Unsigned() > items[j].randomness.Unsigned()
	})

	// Keep the longest prefix whose cumulative cost fits the budget. A
	// trace at the boundary that would overflow the budget is dropped
	// rather than skipped over, so the kept set stays a prefix in
	// randomness order.
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
		// Everything fit; nothing is limited.
		b.threshold = sampling.AlwaysSampleThreshold
	case kept == 0:
		// Nothing fit, not even the single highest-randomness item alone.
		b.threshold = sampling.NeverSampleThreshold
		cumulative = 0
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

	// Deduct the kept cost so the limit holds across ticks. cumulative <=
	// budget <= burst, so AllowN always succeeds; its bool return isn't
	// meaningful here.
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
// threshold CalculateThreshold last computed. A policy that has never been
// reached through the batch pre-pass at all falls back to live per-trace
// token bucket admission instead, so it still limits something.
func (b *budgetLimiter) EvaluateWithThreshold(_ context.Context, id pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, sampling.Threshold, error) {
	b.logger.Debug(b.logMsg)

	if !b.primed {
		if b.limiter.AllowN(time.Now(), int(b.cost(trace))) {
			return samplingpolicy.Sampled, sampling.AlwaysSampleThreshold, nil
		}
		return samplingpolicy.NotSampled, sampling.AlwaysSampleThreshold, nil
	}

	randomness := ResolveRandomness(id, trace.ReceivedBatches)
	if b.threshold.ShouldSample(randomness) {
		return samplingpolicy.Sampled, b.threshold, nil
	}
	return samplingpolicy.NotSampled, sampling.AlwaysSampleThreshold, nil
}

func (*budgetLimiter) IsStateful() bool {
	return true
}
