// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type duration struct {
	logger           *zap.Logger
	lowerThresholdMs int64
	upperThresholdMs int64
}

var _ PolicyEvaluator = (*duration)(nil)

// NewDuration creates a policy evaluator sampling traces with a duration higher than a configured threshold
func NewDuration(settings component.TelemetrySettings, lowerThresholdMs int64, upperThresholdMs int64) PolicyEvaluator {
	return &duration{
		logger:           settings.Logger,
		lowerThresholdMs: lowerThresholdMs,
		upperThresholdMs: upperThresholdMs,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (l *duration) Evaluate(_ context.Context, _ pcommon.TraceID, traceData *TraceData) (Decision, error) {
	l.logger.Debug("Evaluating spans in duration filter")

	traceData.Lock()
	defer traceData.Unlock()
	batches := traceData.ReceivedBatches

	var minTime pcommon.Timestamp
	var maxTime pcommon.Timestamp

	return hasSpanWithCondition(batches, func(span ptrace.Span) bool {
		if minTime == 0 || span.StartTimestamp() < minTime {
			minTime = span.StartTimestamp()
		}
		if maxTime == 0 || span.EndTimestamp() > maxTime {
			maxTime = span.EndTimestamp()
		}

		duration := maxTime.AsTime().Sub(minTime.AsTime())
		return (l.lowerThresholdMs < duration.Milliseconds() && duration.Milliseconds() <= l.upperThresholdMs)
	}), nil
}
