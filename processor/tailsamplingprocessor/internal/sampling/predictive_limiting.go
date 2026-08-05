// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

// defaultRateWindow is how long arrivals accumulate before the sampling
// probability is recomputed. The configured limits are per second, so a
// one second window makes the observed rate directly comparable to them.
const defaultRateWindow = time.Second

// traceCostFunc reports how much of a limiter's budget a single trace
// consumes: spans for rate_limiting, protobuf-marshaled bytes for
// bytes_limiting.
type traceCostFunc func(*samplingpolicy.TraceData) int64

// traceSpanCount reports the number of spans in a trace, the unit the
// rate_limiting policy budgets in. bytes_limiting uses calculateTraceSize.
func traceSpanCount(trace *samplingpolicy.TraceData) int64 {
	return trace.SpanCount
}

// predictiveLimiter samples traces so that a per-second budget is respected
// on average, reporting the sampling threshold it actually applied.
//
// Instead of admitting traces until a token bucket empties, it measures the
// arrival rate over a tumbling window and converts the configured limit into
// the fraction of arriving traces it must keep: at an observed rate R with
// limit L that fraction is L/R, which admits L tokens per second in
// expectation.
//
// That fraction is relative to what arrives, so it cannot be used as an
// absolute threshold directly. Traces reaching the tail sampler may already
// have been sampled upstream, in which case their randomness is confined to
// the top slice of the range and an absolute threshold built from L/R alone
// would admit all of them and limit nothing. The fraction is therefore
// composed with each trace's incoming probability: a trace already sampled at
// probability p is judged against the threshold for p*L/R, so it passes with
// conditional probability L/R. Because that composed threshold is what the
// trace is judged by and what gets reported, the threshold advertised on a
// kept trace is by construction the threshold that admitted it, and
// downstream adjusted counts are correct.
//
// The tradeoff against a token bucket is that the limit is respected in
// expectation rather than exactly: the fraction is derived from the previous
// window, so a sudden change in traffic is not tracked until the next window
// rolls, and the window that follows a long idle period is not limited at
// all. In exchange the decision is a pure function of the trace's own
// randomness and tracestate plus one scalar, needing no coordination across
// the traces being decided together.
//
// This implementation is only constructed when the usetracestate feature gate
// is enabled; with the gate disabled the limiting policies stay on their
// original token bucket, untouched.
type predictiveLimiter struct {
	// limit is the configured budget in tokens per second.
	limit float64
	// cost reports the tokens a single trace consumes.
	cost traceCostFunc
	// window is how long arrivals accumulate before threshold is recomputed.
	window time.Duration

	// now returns the current time. It is a field so tests can roll the
	// window deterministically instead of sleeping.
	now func() time.Time

	// The window state below is not synchronized: every policy evaluation
	// happens on the processor's single decision goroutine, for both sampling
	// strategies. ConsumeTraces only hands work to that goroutine over a
	// channel, which is what lets the processor keep its inner loop
	// lock-free. A future change that evaluates policies from more than one
	// goroutine has to revisit this.
	//
	// windowStart is when the current accumulation window opened.
	windowStart time.Time
	// windowCost is the tokens observed so far in the current window.
	windowCost float64
	// fraction is the share of arriving traces to keep, derived from the
	// previous window and applied until the next roll. It starts at 1 so a
	// freshly started collector does not limit traffic it has not measured,
	// unless the configured limit makes keeping anything impossible.
	fraction float64
}

var (
	_ samplingpolicy.Evaluator          = (*predictiveLimiter)(nil)
	_ samplingpolicy.ThresholdEvaluator = (*predictiveLimiter)(nil)
)

// newPredictiveLimiter builds a limiter that targets tokensPerSecond,
// measuring each trace with cost.
func newPredictiveLimiter(tokensPerSecond int64, cost traceCostFunc) *predictiveLimiter {
	return &predictiveLimiter{
		limit:  float64(tokensPerSecond),
		cost:   cost,
		window: defaultRateWindow,
		now:    time.Now,
		// A non-positive limit can never admit anything, so there is no
		// initial grace period for it.
		fraction: initialFraction(tokensPerSecond),
	}
}

func initialFraction(tokensPerSecond int64) float64 {
	if tokensPerSecond <= 0 {
		return 0
	}
	return 1
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (p *predictiveLimiter) Evaluate(ctx context.Context, id pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	d, _, err := p.EvaluateWithThreshold(ctx, id, trace)
	return d, err
}

// EvaluateWithThreshold decides the trace against the threshold predicted
// from recent traffic and reports that same threshold, so a kept trace
// carries the probability that was actually applied to it.
func (p *predictiveLimiter) EvaluateWithThreshold(_ context.Context, id pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, sampling.Threshold, error) {
	fraction := p.observe(p.cost(trace))
	if fraction >= 1 {
		// Traffic is within the limit, so nothing is sampled down and the
		// policy votes always-sample like any other filter-style policy.
		return samplingpolicy.Sampled, sampling.AlwaysSampleThreshold, nil
	}

	scan := scanOTelTracestate(trace.ReceivedBatches)
	threshold := composeThreshold(scan, fraction)
	randomness := sampling.TraceIDToRandomness(id)
	if scan.hasRandomness {
		randomness = scan.randomness
	}
	if threshold.ShouldSample(randomness) {
		return samplingpolicy.Sampled, threshold, nil
	}
	// A non-sampled trace reports always-sample because the threshold is
	// only meaningful for kept traces; the processor ignores it here.
	return samplingpolicy.NotSampled, sampling.AlwaysSampleThreshold, nil
}

// composeThreshold returns the absolute threshold that keeps the given
// fraction of traces arriving with the sampling information in scan.
//
// fraction is relative to arrivals, so it must be composed with the
// probability a trace has already been sampled at: keeping fraction f of
// traces that arrived with probability p means an absolute probability of
// p*f. Traces carrying no incoming threshold are treated as unsampled
// (p = 1), for which the composed threshold is just the one for f.
func composeThreshold(scan tracestateScan, fraction float64) sampling.Threshold {
	incoming := 1.0
	if scan.hasThreshold {
		incoming = scan.threshold.Probability()
	}
	threshold, err := sampling.ProbabilityToThreshold(incoming * fraction)
	if err != nil {
		// The composed probability is below MinSamplingProbability (or the
		// limit is non-positive), which no threshold can express.
		return sampling.NeverSampleThreshold
	}
	return threshold
}

// observe records a trace's cost against the current window, rolling the
// window and recomputing the threshold when it has elapsed, and returns the
// threshold to apply to this trace.
//
// A trace's cost counts toward the window that determines the *next*
// threshold, not the one being returned: the threshold in force always
// reflects fully observed traffic. When a window spans more than one
// decision tick's worth of traces the same threshold is reused, and a window
// that rolls partway through a tick gives later traces a newer threshold.
// Either way each trace is judged by, and reports, the same value.
func (p *predictiveLimiter) observe(cost int64) float64 {
	now := p.now()
	if p.windowStart.IsZero() {
		p.windowStart = now
	}
	if elapsed := now.Sub(p.windowStart); elapsed >= p.window {
		p.fraction = fractionForRate(p.limit, p.windowCost, elapsed)
		p.windowStart = now
		p.windowCost = 0
	}
	p.windowCost += float64(cost)

	return p.fraction
}

// fractionForRate converts an observed volume over an elapsed period into the
// share of arriving traces that keeps throughput at or under limit tokens per
// second. Traffic already at or below the limit is not sampled down at all.
func fractionForRate(limit, observed float64, elapsed time.Duration) float64 {
	if limit <= 0 {
		return 0
	}
	if elapsed <= 0 || observed <= 0 {
		return 1
	}
	observedRate := observed / elapsed.Seconds()
	if observedRate <= limit {
		return 1
	}
	return limit / observedRate
}

func (*predictiveLimiter) IsStateful() bool {
	return true
}
