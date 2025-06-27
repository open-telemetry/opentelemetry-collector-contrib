// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

type rateLimiting struct {
	spansPerSecond int64
	logger         *zap.Logger
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
// For Bottom-K compliance, rate limiting policies return AlwaysSample with policy metadata
// and the actual rate enforcement is handled by the bucket manager's second pass.
func (r *rateLimiting) Evaluate(_ context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	r.logger.Debug("Evaluating trace in rate-limiting filter for Bottom-K processing")

	// In Bottom-K approach, rate limiting policies always return AlwaysSample
	// with policy identification. The actual rate limiting is enforced by
	// the bucket manager's reservoir sampling in a second pass.
	decision := Decision{
		Threshold: sampling.AlwaysSampleThreshold,
		Attributes: map[string]any{
			"rate_limiting_policy": true,
			"spans_per_second":     r.spansPerSecond,
			"policy_type":          "rate_limiting",
		},
	}

	return decision, nil
}
