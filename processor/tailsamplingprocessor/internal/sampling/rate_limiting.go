// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"math"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

// emaHalfLife controls the responsiveness of the arrival-rate estimate.
// A half-life of 10 seconds means after 10 seconds of constant traffic
// the EMA has converged to ~50% of the gap between the old and new rate.
const emaHalfLife = 10 * time.Second

type rateLimiting struct {
	// Rate limiter using golang.org/x/time/rate for efficient token bucket implementation.
	limiter        *rate.Limiter
	spansPerSecond float64
	logger         *zap.Logger
	useTraceState  bool

	// EMA state for arrival-rate estimation.
	mu       sync.Mutex
	lastTime time.Time
	emaRate  float64 // exponential moving average of observed spans/sec
}

var (
	_ samplingpolicy.Evaluator          = (*rateLimiting)(nil)
	_ samplingpolicy.ThresholdEvaluator = (*rateLimiting)(nil)
)

// NewRateLimiting creates a policy evaluator that samples traces based on a span limit per second using a token bucket algorithm.
// The bucket capacity defaults to 2x the spans per second to allow for reasonable burst traffic.
func NewRateLimiting(settings component.TelemetrySettings, spansPerSecond int64) samplingpolicy.Evaluator {
	return NewRateLimitingWithBurstCapacity(settings, spansPerSecond, spansPerSecond*2)
}

// NewRateLimitingWithBurstCapacity creates a rate limiting policy evaluator with a configurable
// burst capacity, using a token bucket algorithm. Tokens (spans) refill continuously at
// spansPerSecond and the bucket holds at most burstCapacity tokens. A single trace whose span
// count exceeds the burst capacity will not pass.
func NewRateLimitingWithBurstCapacity(settings component.TelemetrySettings, spansPerSecond, burstCapacity int64) samplingpolicy.Evaluator {
	return &rateLimiting{
		limiter:        rate.NewLimiter(rate.Limit(spansPerSecond), int(burstCapacity)),
		spansPerSecond: float64(spansPerSecond),
		logger:         settings.Logger,
		useTraceState:  metadata.ProcessorTailsamplingprocessorUsetracestateFeatureGate.IsEnabled(),
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision based on token bucket consumption.
func (r *rateLimiting) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	d, _, err := r.EvaluateWithThreshold(ctx, traceID, trace)
	return d, err
}

// EvaluateWithThreshold makes the sampling decision and reports the
// effective OpenTelemetry threshold the policy would advertise on
// outgoing tracestate.
//
// The threshold is estimated from the observed arrival rate using an
// exponential moving average (EMA): p = min(1, spansPerSecond / emaRate).
// When the usetracestate feature gate is disabled the threshold is
// AlwaysSampleThreshold, preserving the pre-existing behavior.
func (r *rateLimiting) EvaluateWithThreshold(_ context.Context, _ pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, sampling.Threshold, error) {
	r.logger.Debug("Evaluating spans in rate-limiting filter")

	now := time.Now()
	spanCount := trace.SpanCount

	r.updateEMA(now, spanCount)

	if r.limiter.AllowN(now, int(spanCount)) {
		return samplingpolicy.Sampled, r.effectiveThreshold(), nil
	}

	return samplingpolicy.NotSampled, r.effectiveThreshold(), nil
}

// updateEMA updates the exponential moving average of the observed
// span arrival rate.  It is safe for concurrent use.
func (r *rateLimiting) updateEMA(now time.Time, spanCount int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastTime.IsZero() {
		// First observation: seed the EMA at the configured rate so the
		// initial threshold is AlwaysSampleThreshold until enough data
		// accumulates.
		r.lastTime = now
		r.emaRate = r.spansPerSecond
		return
	}

	dt := now.Sub(r.lastTime).Seconds()
	if dt <= 0 {
		// Clock didn't advance; accumulate spans without updating rate.
		return
	}

	r.lastTime = now
	instantRate := float64(spanCount) / dt

	// Time-weighted EMA: alpha = 1 - exp(-dt / halfLife).
	// This makes the smoothing independent of the call frequency.
	alpha := 1 - math.Exp(-dt/emaHalfLife.Seconds())
	r.emaRate = alpha*instantRate + (1-alpha)*r.emaRate
}

// effectiveThreshold returns the estimated sampling threshold based
// on the current EMA rate.  When the usetracestate feature gate is
// off it returns AlwaysSampleThreshold (the legacy no-op behavior).
func (r *rateLimiting) effectiveThreshold() sampling.Threshold {
	if !r.useTraceState {
		return sampling.AlwaysSampleThreshold
	}

	r.mu.Lock()
	emaRate := r.emaRate
	r.mu.Unlock()

	if emaRate <= r.spansPerSecond {
		// Not limiting: every trace is kept, adjusted count = 1.
		return sampling.AlwaysSampleThreshold
	}

	p := r.spansPerSecond / emaRate
	th, err := sampling.ProbabilityToThreshold(p)
	if err != nil {
		// p is below MinSamplingProbability — extremely unlikely in
		// practice, but fall back to AlwaysSampleThreshold so we
		// never block a sampled trace's threshold.
		return sampling.AlwaysSampleThreshold
	}
	return th
}

func (*rateLimiting) IsStateful() bool {
	return true
}
