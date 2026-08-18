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

// TraceID returns the trace ID carried by the trace's spans, or the zero
// ID if the trace has no spans. All spans in a trace share the same trace
// ID, so the first span found is authoritative.
func (td *TraceData) TraceID() pcommon.TraceID {
	rss := td.ReceivedBatches.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			if spans := sss.At(j).Spans(); spans.Len() > 0 {
				return spans.At(0).TraceID()
			}
		}
	}
	return pcommon.TraceID{}
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

// BatchEvaluator is implemented by policies whose threshold depends on the
// whole group of traces eligible for a decision at once, rather than a
// single trace in isolation. rate_limiting is the canonical example: to
// report a consistent threshold it must sort the group by randomness and
// cut where the span budget runs out.
//
// It does not decide any trace itself -- it only updates the threshold its
// ordinary EvaluateWithThreshold calls compare against afterward. Deciding
// stays in the normal per-trace, per-policy loop, so a trace a
// higher-priority policy already decided (a drop policy, or an earlier
// match under sample_on_first_match) never reaches EvaluateWithThreshold
// and correctly never spends budget. The caller must exclude such traces
// from the batch too, so they don't shift the threshold for traces that
// will actually be asked.
type BatchEvaluator interface {
	ThresholdEvaluator
	// CalculateThreshold is called once per tick with every trace that will
	// actually reach EvaluateWithThreshold this tick, before any of those
	// calls happen. It updates internal state (typically just the
	// threshold) and decides nothing itself.
	CalculateThreshold(ctx context.Context, batch []*TraceData)
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
