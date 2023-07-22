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

type numericAttributeFilter struct {
	key                string
	minValue, maxValue int64
	logger             *zap.Logger
}

var _ PolicyEvaluator = (*numericAttributeFilter)(nil)

// NewNumericAttributeFilter creates a policy evaluator that samples all traces with
// the given attribute in the given numeric range.
func NewNumericAttributeFilter(settings component.TelemetrySettings, key string, minValue, maxValue int64) PolicyEvaluator {
	return &numericAttributeFilter{
		key:      key,
		minValue: minValue,
		maxValue: maxValue,
		logger:   settings.Logger,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (naf *numericAttributeFilter) Evaluate(_ context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	trace.Lock()
	defer trace.Unlock()
	batches := trace.ReceivedBatches

	return hasSpanWithCondition(batches, func(span ptrace.Span) bool {
		if v, ok := span.Attributes().Get(naf.key); ok {
			value := v.Int()
			if value >= naf.minValue && value <= naf.maxValue {
				return true
			}
		}
		return false
	}), nil
}
