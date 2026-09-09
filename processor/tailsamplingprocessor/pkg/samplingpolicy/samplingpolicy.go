// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package samplingpolicy // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// TraceData stores the sampling related trace data.
type TraceData struct {
	// SpanCount track the number of spans on the trace.
	SpanCount int64
	// SizeBytes is how many bytes we have accumulated for the trace.
	SizeBytes uint64
	// ReceivedBatches stores all the batches received for the trace.
	ReceivedBatches ptrace.Traces
}

// Decision gives the status of sampling decision.
type Decision int32

const (
	// Unspecified indicates that the status of the decision was not set yet.
	Unspecified Decision = iota
	// Pending indicates that the policy was not evaluated yet.
	Pending
	// Sampled is used to indicate that the decision was already taken
	// to sample the data.
	Sampled
	// NotSampled is used to indicate that the decision was already taken
	// to not sample the data.
	NotSampled
	// Dropped is used to indicate that a trace should be dropped regardless of
	// all other decisions.
	Dropped
	// Error is used to indicate that policy evaluation was not succeeded.
	Error
	// InvertSampled is used on the invert match flow and indicates to sample
	// the data.
	//
	// Deprecated: Drop policies should be used instead of invert decisions.
	InvertSampled
	// InvertNotSampled is used on the invert match flow and indicates to not
	// sample the data.
	//
	// Deprecated: Drop policies should be used instead of invert decisions.
	InvertNotSampled
)

// String returns a string representation of the Decision.
func (d Decision) String() string {
	switch d {
	case Unspecified:
		return "unspecified"
	case Pending:
		return "pending"
	case Sampled:
		return "sampled"
	case NotSampled:
		return "not_sampled"
	case Dropped:
		return "dropped"
	case Error:
		return "error"
	case InvertSampled:
		return "invert_sampled"
	case InvertNotSampled:
		return "invert_not_sampled"
	default:
		return "unknown"
	}
}

// Evaluator implements a tail-based sampling policy evaluator,
// which makes a sampling decision for a given trace when requested.
type Evaluator interface {
	// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
	Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error)
	// IsStateful reports whether decisions can depend on prior evaluations/state.
	IsStateful() bool
}

// ThresholdEvaluator is implemented by policies that can report the
// effective OpenTelemetry sampling threshold they would advertise on
// outgoing tracestate. Implementing this is optional: policies that
// do not report a threshold are treated by the processor as Sampled
// with sampling.AlwaysSampleThreshold (i.e., always-sample for
// matching items, dominates downstream min(th) reduction).
type ThresholdEvaluator interface {
	Evaluator
	// EvaluateWithThreshold returns the policy's Decision along with
	// the effective Threshold. The Threshold is only meaningful when
	// Decision is Sampled.
	EvaluateWithThreshold(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, sampling.Threshold, error)
}

type Extension interface {
	NewEvaluator(policyName string, cfg map[string]any) (Evaluator, error)
}

// AsThresholdEvaluator returns e as a ThresholdEvaluator. If e
// already implements ThresholdEvaluator it is returned unchanged;
// otherwise e is wrapped in an adapter that reports
// sampling.AlwaysSampleThreshold (i.e., "sampled with no
// quantifiable threshold") whenever Evaluate returns Sampled. This
// lets callers query an effective sampling threshold from any
// Evaluator without per-call type assertions.
func AsThresholdEvaluator(e Evaluator) ThresholdEvaluator {
	if te, ok := e.(ThresholdEvaluator); ok {
		return te
	}
	return &decisionAdapter{Evaluator: e}
}

// decisionAdapter wraps a plain Evaluator so it satisfies
// ThresholdEvaluator.
type decisionAdapter struct {
	Evaluator
}

var _ ThresholdEvaluator = (*decisionAdapter)(nil)

func (a *decisionAdapter) EvaluateWithThreshold(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, sampling.Threshold, error) {
	d, err := a.Evaluate(ctx, traceID, trace)
	return d, sampling.AlwaysSampleThreshold, err
}
