// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

type spanCount struct {
	logger   *zap.Logger
	minSpans int32
	maxSpans int32
}

var _ PolicyEvaluator = (*spanCount)(nil)

// NewSpanCount creates a policy evaluator sampling traces with more than one span per trace
func NewSpanCount(settings component.TelemetrySettings, minSpans, maxSpans int32) PolicyEvaluator {
	return &spanCount{
		logger:   settings.Logger,
		minSpans: minSpans,
		maxSpans: maxSpans,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (c *spanCount) Evaluate(_ context.Context, _ pcommon.TraceID, traceData *TraceData) (Decision, error) {
	c.logger.Debug("Evaluating spans counts in filter")

	spanCount := int(traceData.SpanCount.Load())
	switch {
	case c.maxSpans == 0 && spanCount >= int(c.minSpans):
		return Sampled, nil
	case spanCount >= int(c.minSpans) && spanCount <= int(c.maxSpans):
		return Sampled, nil
	default:
		return NotSampled, nil
	}
}
