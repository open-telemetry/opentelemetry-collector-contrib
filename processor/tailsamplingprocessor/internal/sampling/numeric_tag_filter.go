// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"math"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

type NumericValue struct {
	Value int64 `mapstructure:"value"`
}

type numericAttributeFilter struct {
	key         string
	minValue    configoptional.Optional[NumericValue]
	maxValue    configoptional.Optional[NumericValue]
	logger      *zap.Logger
	invertMatch bool
}

var _ samplingpolicy.Evaluator = (*numericAttributeFilter)(nil)

// NewNumericAttributeFilter creates a policy evaluator that samples all traces with
// the given attribute in the given numeric range. If minValue is nil, it will use math.MinInt64.
// If maxValue is nil, it will use math.MaxInt64. At least one of minValue or maxValue must be set.
func NewNumericAttributeFilter(settings component.TelemetrySettings, key string, minValue, maxValue configoptional.Optional[NumericValue], invertMatch bool) samplingpolicy.Evaluator {
	if !minValue.HasValue() && !maxValue.HasValue() {
		settings.Logger.Error("At least one of minValue or maxValue must be set")
		return nil
	} else if minValue.HasValue() && maxValue.HasValue() && minValue.Get().Value > maxValue.Get().Value {
		settings.Logger.Error("minValue must be less than or equal to maxValue")
		return nil
	}
	return &numericAttributeFilter{
		key:         key,
		minValue:    minValue,
		maxValue:    maxValue,
		logger:      settings.Logger,
		invertMatch: invertMatch,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (naf *numericAttributeFilter) Evaluate(_ context.Context, _ pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	trace.Lock()
	defer trace.Unlock()
	batches := trace.ReceivedBatches

	// Get the effective min/max values
	minVal := int64(math.MinInt64)
	if naf.minValue.HasValue() {
		minVal = naf.minValue.Get().Value
	}
	maxVal := int64(math.MaxInt64)
	if naf.maxValue.HasValue() {
		maxVal = naf.maxValue.Get().Value
	}

	if naf.invertMatch {
		return invertHasResourceOrSpanWithCondition(
			batches,
			func(resource pcommon.Resource) bool {
				if v, ok := resource.Attributes().Get(naf.key); ok {
					value := v.Int()
					if value >= minVal && value <= maxVal {
						return false
					}
				}
				return true
			},
			func(span ptrace.Span) bool {
				if v, ok := span.Attributes().Get(naf.key); ok {
					value := v.Int()
					if value >= minVal && value <= maxVal {
						return false
					}
				}
				return true
			},
		), nil
	}
	return hasResourceOrSpanWithCondition(
		batches,
		func(resource pcommon.Resource) bool {
			if v, ok := resource.Attributes().Get(naf.key); ok {
				value := v.Int()
				if value >= minVal && value <= maxVal {
					return true
				}
			}
			return false
		},
		func(span ptrace.Span) bool {
			if v, ok := span.Attributes().Get(naf.key); ok {
				value := v.Int()
				if value >= minVal && value <= maxVal {
					return true
				}
			}
			return false
		},
	), nil
}
