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

// AttributeInserter represents a function that can insert attributes into trace data
// when a sampling decision is applied. This supports OTEP 250's deferred attribute pattern.
type AttributeInserter func(*TraceData)

// Decision represents a sampling intent with threshold and metadata (OTEP 250 Sampling Intent pattern).
// This structure replaces the previous enum while maintaining backward compatibility.
type Decision struct {
	// Threshold represents the sampling intent as a threshold value
	Threshold sampling.Threshold
	// Attributes contains additional decision metadata (for backward compatibility)
	Attributes map[string]any
	// Error contains error information if decision failed
	Error error
	// AttributeInserters contains functions to insert attributes when sampling decision is applied
	// This implements OTEP 250's deferred attribute pattern for tail sampling
	AttributeInserters []AttributeInserter
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
	// InvertNotSampled is used when mathematically inverting AlwaysSampleThreshold.
	// This represents proper OTEP 250 mathematical inversion: AlwaysSample -> NeverSample
	InvertNotSampled = NewInvertedDecision(sampling.AlwaysSampleThreshold)

	// TODO: Remove this temporary constant - used for test compatibility during refactor
	// This should be replaced with proper threshold-based decisions in tests
	InvertSampled = NewInvertedDecision(sampling.NeverSampleThreshold)
)

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
	return Decision{
		Threshold:  threshold,
		Attributes: make(map[string]any), // Ensure non-nil to avoid zero-value confusion
	}
}

// NewDecisionWithError creates a new Decision representing an error condition.
func NewDecisionWithError(err error) Decision {
	return Decision{
		Threshold:  sampling.NeverSampleThreshold,
		Error:      err,
		Attributes: map[string]any{"error": true},
	}
}

// NewDecisionWithAttributeInserters creates a new Decision with the specified threshold and attribute inserters.
func NewDecisionWithAttributeInserters(threshold sampling.Threshold, inserters ...AttributeInserter) Decision {
	return Decision{
		Threshold:          threshold,
		Attributes:         make(map[string]any),
		AttributeInserters: inserters,
	}
}

// NewSampledDecisionWithAttributes creates a Sampled decision with additional attribute inserters.
// This preserves the {"sampled": true} attribute for backward compatibility.
func NewSampledDecisionWithAttributes(inserters ...AttributeInserter) Decision {
	return Decision{
		Threshold:          sampling.AlwaysSampleThreshold,
		Attributes:         map[string]any{"sampled": true},
		AttributeInserters: inserters,
	}
}

// NewNotSampledDecisionWithAttributes creates a NotSampled decision with additional attribute inserters.
func NewNotSampledDecisionWithAttributes(inserters ...AttributeInserter) Decision {
	return Decision{
		Threshold:          sampling.NeverSampleThreshold,
		Attributes:         nil, // NotSampled has nil attributes for backward compatibility
		AttributeInserters: inserters,
	}
}

// isZeroValue returns true if this is the zero value Decision{}.
// Zero-value should be treated as unspecified, not as AlwaysSampleThreshold.
func (d Decision) isZeroValue() bool {
	return d.Threshold == (sampling.Threshold{}) && d.Attributes == nil && d.Error == nil
}

// IsSampled returns true if this decision represents a sampling intent.
// This provides backward compatibility for boolean decision logic.
// NOTE: This method is deprecated in favor of ShouldSample(randomness) which implements
// proper OTEP 235 threshold comparison: (randomness >= threshold) = sampled
func (d Decision) IsSampled() bool {
	// Zero-value Decision{} should be treated as unspecified, not sampled
	if d.isZeroValue() {
		return false
	}

	// Error conditions are never sampled
	if d.Error != nil {
		return false
	}

	// For backward compatibility only: assume AlwaysSampleThreshold means "sample"
	// In the new threshold paradigm, use ShouldSample(randomness) instead
	return d.Threshold == sampling.AlwaysSampleThreshold
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

// ApplyAttributeInserters applies all deferred attribute inserters to the trace data.
// This should be called after the final sampling decision is made and randomness is available.
func (d Decision) ApplyAttributeInserters(trace *TraceData) {
	for _, inserter := range d.AttributeInserters {
		if inserter != nil {
			inserter(trace)
		}
	}
}

// CombineAttributeInserters combines attribute inserters from multiple decisions.
// This is used when combining decisions in composite policies.
func CombineAttributeInserters(decisions ...Decision) []AttributeInserter {
	var combined []AttributeInserter
	for _, decision := range decisions {
		combined = append(combined, decision.AttributeInserters...)
	}
	return combined
}

// CombineAttributeInserterFuncs combines multiple AttributeInserter functions into one
func CombineAttributeInserterFuncs(inserters ...AttributeInserter) AttributeInserter {
	return func(trace *TraceData) {
		for _, inserter := range inserters {
			if inserter != nil {
				inserter(trace)
			}
		}
	}
}

// Common attribute inserter functions for tail sampling patterns

// PolicyNameInserter creates an attribute inserter that records which policy sampled the trace.
func PolicyNameInserter(policyName string) AttributeInserter {
	return func(trace *TraceData) {
		SetAttrOnScopeSpans(trace, "tailsampling.policy", policyName)
	}
}

// CompositePolicyNameInserter creates an attribute inserter that records which composite sub-policy sampled the trace.
func CompositePolicyNameInserter(subPolicyName string) AttributeInserter {
	return func(trace *TraceData) {
		SetAttrOnScopeSpans(trace, "tailsampling.composite_policy", subPolicyName)
	}
}

// CustomAttributeInserter creates an attribute inserter for custom key-value pairs.
func CustomAttributeInserter(key, value string) AttributeInserter {
	return func(trace *TraceData) {
		SetAttrOnScopeSpans(trace, key, value)
	}
}

// SubPolicyDecision represents a sub-policy's threshold intent and attribute inserter.
// This is used to track which sub-policies contributed to a composite decision
// so that attributes can be applied only from policies that actually sample given randomness.
type SubPolicyDecision struct {
	Threshold         sampling.Threshold
	AttributeInserter AttributeInserter
	PolicyName        string // For debugging/tracing
}

// DeferredAttributeInserter creates an inserter that applies attributes only from
// sub-policies that would sample given the provided randomness value.
// This implements the core OTEP 250 deferred pattern for OR/AND composite decisions.
func NewDeferredAttributeInserter(subDecisions []SubPolicyDecision, combineLogic string) AttributeInserter {
	return func(trace *TraceData) {
		if trace.RandomnessValue == (sampling.Randomness{}) {
			// No randomness available, can't determine which policies would sample
			return
		}

		for _, subDecision := range subDecisions {
			// Check if this sub-policy would actually sample given the randomness
			if subDecision.Threshold.ShouldSample(trace.RandomnessValue) {
				// This sub-policy contributed to the sampling decision, apply its attributes
				if subDecision.AttributeInserter != nil {
					subDecision.AttributeInserter(trace)
				}
			}
		}
	}
}

// PolicyEvaluator implements a tail-based sampling policy evaluator,
// which makes a sampling decision for a given trace when requested.
type PolicyEvaluator interface {
	// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
	Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error)
}

// Helper functions for creating decisions with common attribute patterns

// NewSampledDecisionWithPolicyName creates a Sampled decision that inserts a policy name attribute
func NewSampledDecisionWithPolicyName(policyName string) Decision {
	inserter := func(trace *TraceData) {
		SetAttrOnScopeSpans(trace, "tailsampling.policy", policyName)
	}
	return Decision{
		Threshold:          sampling.AlwaysSampleThreshold,
		Attributes:         map[string]any{"sampled": true},
		AttributeInserters: []AttributeInserter{inserter},
	}
}

// NewSampledDecisionWithCompositePolicy creates a Sampled decision that inserts a composite policy name
func NewSampledDecisionWithCompositePolicy(policyName string) Decision {
	inserter := func(trace *TraceData) {
		SetAttrOnScopeSpans(trace, "tailsampling.composite_policy", policyName)
	}
	return Decision{
		Threshold:          sampling.AlwaysSampleThreshold,
		Attributes:         map[string]any{"sampled": true},
		AttributeInserters: []AttributeInserter{inserter},
	}
}
