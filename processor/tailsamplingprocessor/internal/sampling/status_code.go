// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type statusCodeFilter struct {
	logger      *zap.Logger
	statusCodes []ptrace.StatusCode
}

var _ PolicyEvaluator = (*statusCodeFilter)(nil)

// NewStatusCodeFilter creates a policy evaluator that samples all traces with
// a given status code.
func NewStatusCodeFilter(settings component.TelemetrySettings, statusCodeString []string) (PolicyEvaluator, error) {
	if len(statusCodeString) == 0 {
		return nil, errors.New("expected at least one status code to filter on")
	}

	statusCodes := make([]ptrace.StatusCode, len(statusCodeString))

	for i := range statusCodeString {
		switch statusCodeString[i] {
		case "OK":
			statusCodes[i] = ptrace.StatusCodeOk
		case "ERROR":
			statusCodes[i] = ptrace.StatusCodeError
		case "UNSET":
			statusCodes[i] = ptrace.StatusCodeUnset
		default:
			return nil, fmt.Errorf("unknown status code %q, supported: OK, ERROR, UNSET", statusCodeString[i])
		}
	}

	return &statusCodeFilter{
		logger:      settings.Logger,
		statusCodes: statusCodes,
	}, nil
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (r *statusCodeFilter) Evaluate(_ context.Context, _ pcommon.TraceID, trace *TraceData) (Decision, error) {
	r.logger.Debug("Evaluating spans in status code filter")

	trace.Lock()
	batches := trace.ReceivedBatches
	trace.Unlock()

	return hasSpanWithCondition(batches, func(span ptrace.Span) bool {
		for _, statusCode := range r.statusCodes {
			if span.Status().Code() == statusCode {
				return true
			}
		}
		return false
	}), nil
}
