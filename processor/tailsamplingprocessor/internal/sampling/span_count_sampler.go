// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

type spanCount struct {
	logger   *zap.Logger
	minSpans int32
	maxSpans int32
}

var _ samplingpolicy.Evaluator = (*spanCount)(nil)

// NewSpanCount creates a policy evaluator sampling traces with more than one span per trace
func NewSpanCount(settings component.TelemetrySettings, minSpans, maxSpans int32) samplingpolicy.Evaluator {
	return &spanCount{
		logger:   settings.Logger,
		minSpans: minSpans,
		maxSpans: maxSpans,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (c *spanCount) Evaluate(_ context.Context, _ pcommon.TraceID, traceData *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	c.logger.Debug("Evaluating spans counts in filter")

	spanCount := int(traceData.SpanCount.Load())
	switch {
	case c.maxSpans == 0 && spanCount >= int(c.minSpans):
		return samplingpolicy.Sampled, nil
	case spanCount >= int(c.minSpans) && spanCount <= int(c.maxSpans):
		return samplingpolicy.Sampled, nil
	default:
		return samplingpolicy.NotSampled, nil
	}
}
