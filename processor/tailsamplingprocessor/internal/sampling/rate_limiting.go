// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

type rateLimiting struct {
	currentSecond        int64
	spansInCurrentSecond int64
	spansPerSecond       int64
	logger               *zap.Logger
}

var _ PolicyEvaluator = (*rateLimiting)(nil)

// NewRateLimiting creates a policy evaluator the samples all traces.
func NewRateLimiting(settings component.TelemetrySettings, spansPerSecond int64) PolicyEvaluator {
	return &rateLimiting{
		spansPerSecond: spansPerSecond,
		logger:         settings.Logger,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
// This implementation maintains OTEP 235 consistency by updating trace thresholds
// when applying rate limiting decisions.
func (r *rateLimiting) Evaluate(_ context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	r.logger.Debug("Evaluating trace in rate-limiting filter using OTEP 235 threshold consistency")

	currSecond := time.Now().Unix()
	if r.currentSecond != currSecond {
		r.currentSecond = currSecond
		r.spansInCurrentSecond = 0
	}

	spansInSecondIfSampled := r.spansInCurrentSecond + trace.SpanCount.Load()
	if spansInSecondIfSampled < r.spansPerSecond {
		r.spansInCurrentSecond = spansInSecondIfSampled

		// For OTEP 235 consistency, we need to ensure the trace threshold reflects
		// that this trace was sampled. Since rate limiting accepts all traces within
		// the limit, we use AlwaysSampleThreshold to indicate 100% sampling.
		r.updateTraceThreshold(trace, sampling.AlwaysSampleThreshold)

		return Sampled, nil
	}

	// Rate limit exceeded - reject the trace
	// Note: We don't update the threshold here since the trace is not sampled
	return NotSampled, nil
}

// updateTraceThreshold updates the trace's final threshold using OR logic (minimum threshold).
// Since the tail sampler uses OR logic (any policy can trigger sampling),
// we use the least restrictive (minimum) threshold per OTEP 250 AnyOf semantics.
func (r *rateLimiting) updateTraceThreshold(trace *TraceData, policyThreshold sampling.Threshold) {
	if trace.FinalThreshold == nil {
		// First policy to set a threshold
		trace.FinalThreshold = &policyThreshold
	} else {
		// Use the less restrictive (lower) threshold for OR logic
		if sampling.ThresholdGreater(*trace.FinalThreshold, policyThreshold) {
			trace.FinalThreshold = &policyThreshold
		}
	}
}
