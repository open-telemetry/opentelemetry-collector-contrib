// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	tracesdk "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type traceStateFilter struct {
	key     string
	logger  *zap.Logger
	matcher func(string) bool
}

var _ PolicyEvaluator = (*traceStateFilter)(nil)

// NewTraceStateFilter creates a policy evaluator that samples all traces with
// the given value by the specific key in the trace_state.
func NewTraceStateFilter(settings component.TelemetrySettings, key string, values []string) PolicyEvaluator {
	// initialize the exact value map
	valuesMap := make(map[string]struct{})
	for _, value := range values {
		// the key-value pair("=" will take one character) in trace_state can't exceed 256 characters
		if value != "" && len(key)+len(value) < 256 {
			valuesMap[value] = struct{}{}
		}
	}
	return &traceStateFilter{
		key:    key,
		logger: settings.Logger,
		matcher: func(toMatch string) bool {
			_, matched := valuesMap[toMatch]
			return matched
		},
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (tsf *traceStateFilter) Evaluate(_ context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	trace.Lock()
	defer trace.Unlock()
	batches := trace.ReceivedBatches

	return hasSpanWithCondition(batches, func(span ptrace.Span) bool {
		traceState, err := tracesdk.ParseTraceState(span.TraceState().AsRaw())
		if err != nil {
			return false
		}
		if ok := tsf.matcher(traceState.Get(tsf.key)); ok {
			return true
		}
		return false
	}), nil
}
