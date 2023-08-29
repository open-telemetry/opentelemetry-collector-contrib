// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
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
func (r *rateLimiting) Evaluate(_ context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	r.logger.Debug("Evaluating spans in rate-limiting filter")
	currSecond := time.Now().Unix()
	if r.currentSecond != currSecond {
		r.currentSecond = currSecond
		r.spansInCurrentSecond = 0
	}

	spansInSecondIfSampled := r.spansInCurrentSecond + trace.SpanCount.Load()
	if spansInSecondIfSampled < r.spansPerSecond {
		r.spansInCurrentSecond = spansInSecondIfSampled
		return Sampled, nil
	}

	return NotSampled, nil
}
