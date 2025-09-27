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

type booleanAttributeFilter struct {
	key         string
	value       bool
	logger      *zap.Logger
	invertMatch bool
}

var _ samplingpolicy.Evaluator = (*booleanAttributeFilter)(nil)

// NewBooleanAttributeFilter creates a policy evaluator that samples all traces with
// the given attribute that match the supplied boolean value.
func NewBooleanAttributeFilter(settings component.TelemetrySettings, key string, value, invertMatch bool) samplingpolicy.Evaluator {
	return &booleanAttributeFilter{
		key:         key,
		value:       value,
		logger:      settings.Logger,
		invertMatch: invertMatch,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (baf *booleanAttributeFilter) Evaluate(_ context.Context, _ pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	trace.Lock()
	defer trace.Unlock()
	batches := trace.ReceivedBatches

	if baf.invertMatch {
		return invertHasResourceOrSpanWithCondition(
			batches,
			func(resource pcommon.Resource) bool {
				if v, ok := resource.Attributes().Get(baf.key); ok {
					value := v.Bool()
					return value != baf.value
				}
				return true
			},
			func(span ptrace.Span) bool {
				if v, ok := span.Attributes().Get(baf.key); ok {
					value := v.Bool()
					return value != baf.value
				}
				return true
			},
		), nil
	}
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
