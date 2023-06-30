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

type booleanAttributeFilter struct {
	key    string
	value  bool
	logger *zap.Logger
}

var _ PolicyEvaluator = (*booleanAttributeFilter)(nil)

// NewBooleanAttributeFilter creates a policy evaluator that samples all traces with
// the given attribute that match the supplied boolean value.
func NewBooleanAttributeFilter(settings component.TelemetrySettings, key string, value bool) PolicyEvaluator {
	return &booleanAttributeFilter{
		key:    key,
		value:  value,
		logger: settings.Logger,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (baf *booleanAttributeFilter) Evaluate(_ context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	trace.Lock()
	batches := trace.ReceivedBatches
	trace.Unlock()

	return hasResourceOrSpanWithCondition(
		batches,
		func(resource pcommon.Resource) bool {
			if v, ok := resource.Attributes().Get(baf.key); ok {
				value := v.Bool()
				return value == baf.value
			}
			return false
		},
		func(span ptrace.Span) bool {
			if v, ok := span.Attributes().Get(baf.key); ok {
				value := v.Bool()
				return value == baf.value
			}
			return false
		}), nil
}
