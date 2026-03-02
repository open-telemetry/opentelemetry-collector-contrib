// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

type traceFlags struct {
	logger *zap.Logger
}

var _ samplingpolicy.Evaluator = (*traceFlags)(nil)

// NewTraceFlags creates a policy evaluator that samples all traces with the sampled flag set in the trace flags.
func NewTraceFlags(settings component.TelemetrySettings) samplingpolicy.Evaluator {
	return &traceFlags{
		logger: settings.Logger,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
// Spans are sampled if any span in the trace has the sampled flag set in the trace flags.
func (tf *traceFlags) Evaluate(_ context.Context, _ pcommon.TraceID, td *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	tf.logger.Debug("Evaluating spans in trace-flags filter")

	return hasSpanWithCondition(td.ReceivedBatches, func(span ptrace.Span) bool {
		// Get first 8 bits for trace flags byte from span flags, then bit mask for sampled flag.
		return (byte(span.Flags()) & byte(trace.FlagsSampled)) != 0
	}), nil
}
