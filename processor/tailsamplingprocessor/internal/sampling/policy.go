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

// BottomKDeferredData contains metadata needed for deferred Bottom-K threshold calculation
type BottomKDeferredData struct {
	// IsRateLimited indicates this trace needs Bottom-K rate limiting threshold calculation
	IsRateLimited bool
	// MinRandomnessInBucket is the minimum randomness value among all rate-limited traces in the bucket
	MinRandomnessInBucket uint64
	// TracesInBucket is the count of rate-limited traces in the bucket (K value for Bottom-K)
	TracesInBucket uint64
}

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

	// Bottom-K deferred threshold calculation fields
	// BottomKMetadata contains information needed for deferred threshold calculation
	BottomKMetadata *BottomKDeferredData

	// Varopt tail sampling multiplier for OTEP 235 compliant rate limiting
	// This represents the adjustment factor from Varopt rate limiting
	TailSamplingMultiplier *float64
}

// AttributeInserter represents a function that can insert attributes into trace data
// when a sampling decision is applied.
type AttributeInserter func(*TraceData)

// Decision represents a sampling intent with threshold and metadata.
type Decision struct {
	// Threshold represents the sampling intent as a threshold value
	Threshold sampling.Threshold
	// Attributes contains additional decision metadata
	Attributes map[string]any
	// Error contains error information if decision failed
	Error error
	// AttributeInserters contains functions to insert attributes when sampling decision is applied
	AttributeInserters []AttributeInserter
}

// Decision constants for common sampling decisions

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
func NewSampledDecisionWithAttributes(inserters ...AttributeInserter) Decision {
	return Decision{
		Threshold:          sampling.AlwaysSampleThreshold,
		Attributes:         make(map[string]any), // Consistent with NewDecisionWithThreshold
		AttributeInserters: inserters,
	}
}

// NewNotSampledDecisionWithAttributes creates a NotSampled decision with additional attribute inserters.
func NewNotSampledDecisionWithAttributes(inserters ...AttributeInserter) Decision {
	return Decision{
		Threshold:          sampling.NeverSampleThreshold,
		Attributes:         nil, // NotSampled has nil attributes
		AttributeInserters: inserters,
	}
}

// ShouldSample determines if a trace should be sampled based on the decision threshold and randomness.
// This implements the OTEP 250 final sampling decision logic.
func (d Decision) ShouldSample(randomness sampling.Randomness) bool {
	if d.Error != nil {
		return false
	}
	// NeverSampleThreshold means dropped
	if d.Threshold == sampling.NeverSampleThreshold {
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
		Attributes:         make(map[string]any), // Consistent with NewDecisionWithThreshold
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
		Attributes:         make(map[string]any), // Consistent with NewDecisionWithThreshold
		AttributeInserters: []AttributeInserter{inserter},
	}
}

// NewInvertedDecision creates a new Decision with mathematically inverted threshold.
// This implements OTEP 250 mathematical inversion: inverted = MaxAdjustedCount - original.
func NewInvertedDecision(threshold sampling.Threshold) Decision {
	invertedUnsigned := sampling.MaxAdjustedCount - threshold.Unsigned()
	invertedThreshold, err := sampling.UnsignedToThreshold(invertedUnsigned)
	if err != nil {
		// Fallback to AlwaysSampleThreshold if inversion fails
		invertedThreshold = sampling.AlwaysSampleThreshold
	}
	return Decision{
		Threshold:  invertedThreshold,
		Attributes: make(map[string]any), // Ensure non-nil to avoid zero-value confusion
	}
}
