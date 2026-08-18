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
// CalculateThreshold computes a single threshold from the whole group of
// traces that will actually reach this policy this tick (see
// samplingpolicy.BatchEvaluator); the ordinary, per-trace
// EvaluateWithThreshold call then just compares a trace's own randomness
// against it. Deciding stays where every other policy already decides -- in
// the normal per-trace, per-policy evaluation loop -- so a trace that a
// higher-priority policy (a drop policy, or an earlier match under
// sample_on_first_match) already decides never reaches this policy's
// EvaluateWithThreshold at all, and correctly never spends its budget: it
// is the caller's job to exclude such traces from the batch for the same
// reason a trace that will not actually be asked should not shift the
// threshold computed for the traces that will be.
//
// This implementation is only constructed when the usetracestate feature gate
// is enabled; with the gate disabled the limiting policies stay on their
// original per-trace token bucket, untouched.
type budgetLimiter struct {
	// Token bucket implemented by golang.org/x/time/rate.
	limiter *rate.Limiter
	// cost reports the tokens a single trace consumes from the bucket.
	cost   traceCostFunc
	logger *zap.Logger
	// logMsg is recorded at debug level on each per-trace evaluation.
	logMsg string

	// threshold is the live cutoff computed by the most recent
	// CalculateThreshold call. Its zero value is AlwaysSampleThreshold, so
	// a policy that has been batched at least once but whose last batch
	// fit entirely within budget correctly imposes no restriction.
	//
	// Access is confined to the single decision-tick goroutine (batch
	// sampling is not used in span-ingest mode), so no additional
	// synchronization is required.
	threshold sampling.Threshold
	// primed is set the first time CalculateThreshold is ever called, and
	// never cleared afterward. A policy that is never reachable through
	// the processor's batch pre-pass at all (nested inside a composite
	// policy, which does not support batching -- see the tracestate
	// handling docs) stays unprimed forever and falls back to live
	// per-trace token bucket admission instead of comparing against a
	// threshold that would otherwise never be updated.
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
		// A threshold cannot separate traces that share the same
		// randomness. If the first dropped trace ties the smallest kept
		// one, drop the whole tied group so every kept trace is strictly
		// above every dropped trace and the reported threshold sorts them
		// exactly (kept: th <= R, dropped: th > R). Exact ties in 56-bit
		// randomness are vanishingly unlikely; this just keeps the
		// invariant airtight.
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

	// Deduct the kept cost from the bucket so the limit is enforced across
	// batches. cumulative <= budget <= burst, so AllowN always succeeds and
	// consumes exactly the kept tokens; its bool is not meaningful here.
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
