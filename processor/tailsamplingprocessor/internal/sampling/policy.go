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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// TraceData stores the sampling related trace data.
type TraceData struct {
	sync.Mutex
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

	// OTEP 235 fields for consistent probability sampling
	// RandomnessValue extracted from TraceID or explicit rv in TraceState
	RandomnessValue sampling.Randomness
	// FinalThreshold tracks the most restrictive threshold after all policies
	FinalThreshold *sampling.Threshold
	// TraceStatePresent indicates if any spans have TraceState for optimization
	TraceStatePresent bool

	// TODO: For improved consistency, we should validate that all spans in a trace
	// have the same randomness value (derived from TraceID or explicit rv in TraceState).
	// Inconsistent randomness values could indicate upstream sampling inconsistencies.
}

// Decision represents a sampling intent with threshold and metadata (OTEP 250 Sampling Intent pattern).
// This structure replaces the previous enum while maintaining backward compatibility.
type Decision struct {
	// Threshold represents the sampling intent as a threshold value
	Threshold sampling.Threshold
	// Attributes contains additional decision metadata
	Attributes map[string]any
	// Error contains error information if decision failed
	Error error
}

// Legacy compatibility constants that return Decision structs
var (
	// Unspecified indicates that the status of the decision was not set yet.
	Unspecified = Decision{Threshold: sampling.NeverSampleThreshold}
	// Pending indicates that the policy was not evaluated yet.
	Pending = Decision{Threshold: sampling.NeverSampleThreshold, Attributes: map[string]any{"status": "pending"}}
	// Sampled is used to indicate that the decision was already taken to sample the data.
	Sampled = Decision{Threshold: sampling.AlwaysSampleThreshold, Attributes: map[string]any{"sampled": true}}
	// NotSampled is used to indicate that the decision was already taken to not sample the data.
	NotSampled = Decision{Threshold: sampling.NeverSampleThreshold}
	// Dropped is used to indicate that a trace should be dropped regardless of all other decisions.
	Dropped = Decision{Threshold: sampling.NeverSampleThreshold, Attributes: map[string]any{"dropped": true}}
	// Error is used to indicate that policy evaluation was not succeeded.
	ErrorDecision = Decision{Threshold: sampling.NeverSampleThreshold, Attributes: map[string]any{"error": true}}
	// InvertSampled is used on the invert match flow and indicates weak sampling intent.
	// This has a small non-zero threshold so it can be overridden by explicit NotSampled decisions.
	// Represents "sample this unless something else says not to".
	InvertSampled = Decision{Threshold: createWeakSampleThreshold(), Attributes: map[string]any{"inverted": true}}
	// InvertNotSampled is used on the invert match flow and indicates to not sample the data.
	// This represents mathematically inverted threshold: when original condition would be "always sample",
	// inversion makes it "never sample".
	InvertNotSampled = NewInvertedDecision(sampling.AlwaysSampleThreshold)
)

// createWeakSampleThreshold creates a small non-zero threshold for weak sampling intent.
// This allows InvertSampled to be overridden by explicit NotSampled decisions in OR logic.
func createWeakSampleThreshold() sampling.Threshold {
	// Use threshold value 1 out of MaxAdjustedCount to represent weak sampling intent
	threshold, err := sampling.UnsignedToThreshold(1)
	if err != nil {
		// Fallback to AlwaysSampleThreshold if creation fails
		return sampling.AlwaysSampleThreshold
	}
	return threshold
}

// InvertThreshold mathematically inverts a threshold for OTEP 250 compliance.
// Inverted threshold = MaxAdjustedCount - threshold
func InvertThreshold(t sampling.Threshold) sampling.Threshold {
	inverted, err := sampling.UnsignedToThreshold(sampling.MaxAdjustedCount - t.Unsigned())
	if err != nil {
		// This should never happen for valid thresholds
		return sampling.NeverSampleThreshold
	}
	return inverted
}

// NewInvertedDecision creates a Decision with mathematically inverted threshold.
// This implements proper OTEP 250 threshold inversion rather than boolean flags.
func NewInvertedDecision(originalThreshold sampling.Threshold) Decision {
	return Decision{
		Threshold:  InvertThreshold(originalThreshold),
		Attributes: map[string]any{"inverted": true},
	}
}

// NewDecisionWithThreshold creates a new Decision with the specified threshold.
func NewDecisionWithThreshold(threshold sampling.Threshold) Decision {
	return Decision{Threshold: threshold}
}

// NewDecisionWithError creates a new Decision representing an error condition.
func NewDecisionWithError(err error) Decision {
	return Decision{
		Threshold:  sampling.NeverSampleThreshold,
		Error:      err,
		Attributes: map[string]any{"error": true},
	}
}

// isZeroValue returns true if this is the zero value Decision{}.
// Zero-value should be treated as unspecified, not as AlwaysSampleThreshold.
func (d Decision) isZeroValue() bool {
	return d.Threshold == (sampling.Threshold{}) && d.Attributes == nil && d.Error == nil
}

// IsSampled returns true if this decision represents a sampling intent.
// This provides backward compatibility for boolean decision logic.
func (d Decision) IsSampled() bool {
	// Zero-value Decision{} should be treated as unspecified, not sampled
	if d.isZeroValue() {
		return false
	}
	return d.Threshold != sampling.NeverSampleThreshold && d.Error == nil
}

// IsError returns true if this decision represents an error condition.
func (d Decision) IsError() bool {
	return d.Error != nil
}

// IsDropped returns true if this decision represents a dropped trace.
func (d Decision) IsDropped() bool {
	if d.Attributes == nil {
		return false
	}
	dropped, ok := d.Attributes["dropped"].(bool)
	return ok && dropped
}

// IsNotSampled returns true if this decision represents not sampling (but not dropped).
func (d Decision) IsNotSampled() bool {
	// Zero-value Decision{} is considered unspecified, not NotSampled
	if d.isZeroValue() {
		return false
	}
	return d.Threshold == sampling.NeverSampleThreshold && !d.IsError() && !d.IsDropped() && !d.IsInverted()
}

// IsInvertSampled returns true if this decision represents inverted sampling.
func (d Decision) IsInvertSampled() bool {
	return d.IsSampled() && d.IsInverted()
}

// IsInvertNotSampled returns true if this decision represents inverted not sampling.
func (d Decision) IsInvertNotSampled() bool {
	return !d.IsSampled() && d.IsInverted()
}

// IsInverted returns true if this decision is from an inverted policy.
func (d Decision) IsInverted() bool {
	if d.Attributes == nil {
		return false
	}
	inverted, ok := d.Attributes["inverted"].(bool)
	return ok && inverted
}

// IsUnspecified returns true if this decision is unspecified/pending.
func (d Decision) IsUnspecified() bool {
	// Zero-value Decision{} should be treated as unspecified
	if d.isZeroValue() {
		return true
	}
	// Only explicit Unspecified or Pending decisions should return true
	if d.Attributes != nil {
		if status, ok := d.Attributes["status"]; ok && status == "pending" {
			return true
		}
	}
	return false
}

// ShouldSample determines if a trace should be sampled based on the decision threshold and randomness.
// This implements the OTEP 250 final sampling decision logic.
func (d Decision) ShouldSample(randomness sampling.Randomness) bool {
	if d.Error != nil || d.IsDropped() {
		return false
	}
	return d.Threshold.ShouldSample(randomness)
}

// PolicyEvaluator implements a tail-based sampling policy evaluator,
// which makes a sampling decision for a given trace when requested.
type PolicyEvaluator interface {
	// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
	Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error)
}
