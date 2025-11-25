// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

type latency struct {
	logger           *zap.Logger
	thresholdMs      int64
	upperThresholdMs int64
}

var _ samplingpolicy.Evaluator = (*latency)(nil)

// NewLatency creates a policy evaluator sampling traces with a duration greater than a configured threshold
func NewLatency(settings component.TelemetrySettings, thresholdMs, upperThresholdMs int64) samplingpolicy.Evaluator {
	return &latency{
		logger:           settings.Logger,
		thresholdMs:      thresholdMs,
		upperThresholdMs: upperThresholdMs,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (l *latency) Evaluate(_ context.Context, _ pcommon.TraceID, traceData *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	l.logger.Debug("Evaluating spans in latency filter")

	batches := traceData.ReceivedBatches

	return hasSpanWithCondition(batches, l.condition()), nil
}

func (l *latency) EarlyEvaluate(_ context.Context, _ pcommon.TraceID, batch ptrace.ResourceSpans, _ *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	// If an upper threshold is set we don't know if there will be a future
	// span that will cause a not sampled decision.
	if l.upperThresholdMs > 0 {
		return samplingpolicy.Unspecified, nil
	}
	return batchHasSpanWithCondition(batch, l.condition()), nil
}

func (l *latency) condition() func(span ptrace.Span) bool {
	var minTime pcommon.Timestamp
	var maxTime pcommon.Timestamp

	return func(span ptrace.Span) bool {
		if minTime == 0 || span.StartTimestamp() < minTime {
			minTime = span.StartTimestamp()
		}
		if maxTime == 0 || span.EndTimestamp() > maxTime {
			maxTime = span.EndTimestamp()
		}

		duration := maxTime.AsTime().Sub(minTime.AsTime())
		if l.upperThresholdMs == 0 {
			return duration.Milliseconds() >= l.thresholdMs
		}
		return (l.thresholdMs < duration.Milliseconds() && duration.Milliseconds() <= l.upperThresholdMs)
	}
}
