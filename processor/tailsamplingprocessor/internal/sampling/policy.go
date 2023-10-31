// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TraceData stores the sampling related trace data.
type TraceData struct {
	sync.Mutex
	// Decisions gives the current status of the sampling decision for each policy.
	Decisions []Decision
	// Arrival time the first span for the trace was received.
	ArrivalTime time.Time
	// Decisiontime time when sampling decision was taken.
	DecisionTime time.Time
	// SpanCount track the number of spans on the trace.
	SpanCount *atomic.Int64
	// ReceivedBatches stores all the batches received for the trace.
	ReceivedBatches ptrace.Traces
	// FinalDecision.
	FinalDecision Decision
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
	// Dropped is used when data needs to be purged before the sampling policy
	// had a chance to evaluate it.
	Dropped
	// Error is used to indicate that policy evaluation was not succeeded.
	Error
	// InvertSampled is used on the invert match flow and indicates to sample
	// the data.
	InvertSampled
	// InvertNotSampled is used on the invert match flow and indicates to not
	// sample the data.
	InvertNotSampled
)

// PolicyEvaluator implements a tail-based sampling policy evaluator,
// which makes a sampling decision for a given trace when requested.
type PolicyEvaluator interface {
	// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
	Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error)
}
