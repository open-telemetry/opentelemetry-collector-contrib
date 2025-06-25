// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// SamplingCombinator defines the interface for different composition semantics.
// Combinators replace global precedence rules with localized semantic types.
type SamplingCombinator interface {
	// Evaluate combines multiple policy decisions using specific composition semantics
	Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error)
}

// ThresholdOrCombinator implements pure OTEP 250 OR logic using minimum threshold.
// This combinator uses pure threshold mathematics without precedence rules.
type ThresholdOrCombinator struct {
	policies []PolicyEvaluator
}

// NewThresholdOrCombinator creates a new pure OTEP 250 OR combinator.
func NewThresholdOrCombinator(policies []PolicyEvaluator) *ThresholdOrCombinator {
	return &ThresholdOrCombinator{
		policies: policies,
	}
}

// Evaluate implements OR logic by taking the minimum threshold (most permissive).
// This follows pure OTEP 250 semantics without precedence overrides.
func (c *ThresholdOrCombinator) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error) {
	if len(c.policies) == 0 {
		return NotSampled, nil
	}

	var decisions []Decision
	var firstError error

	// Collect decisions from all policies
	for _, policy := range c.policies {
		decision, err := policy.Evaluate(ctx, traceID, trace)
		if err != nil && firstError == nil {
			firstError = err
		}
		decisions = append(decisions, decision)
	}

	if firstError != nil {
		return NewDecisionWithError(fmt.Errorf("policy evaluation error: %w", firstError)), nil
	}

	// Apply pure OR logic: minimum threshold wins (most permissive)
	return combineWithOrLogic(decisions), nil
}

// InvertAwareOrCombinator implements OR logic with inverted clause precedence for backward compatibility.
// This combinator preserves legacy behavior where explicit NotSampled overrides inverted decisions.
type InvertAwareOrCombinator struct {
	normalPolicies   []PolicyEvaluator
	invertedPolicies []PolicyEvaluator
}

// NewInvertAwareOrCombinator creates a new precedence-aware OR combinator.
func NewInvertAwareOrCombinator(normalPolicies, invertedPolicies []PolicyEvaluator) *InvertAwareOrCombinator {
	return &InvertAwareOrCombinator{
		normalPolicies:   normalPolicies,
		invertedPolicies: invertedPolicies,
	}
}

// Evaluate implements OR logic with inverted clause precedence.
// Follows the specific precedence rule: "Inverted sampling only if no explicit not-sampling"
func (c *InvertAwareOrCombinator) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error) {
	var normalDecisions []Decision
	var invertedDecisions []Decision
	var firstError error

	// Collect decisions from normal policies
	for _, policy := range c.normalPolicies {
		decision, err := policy.Evaluate(ctx, traceID, trace)
		if err != nil && firstError == nil {
			firstError = err
		}
		normalDecisions = append(normalDecisions, decision)
	}

	// Collect decisions from inverted policies
	for _, policy := range c.invertedPolicies {
		decision, err := policy.Evaluate(ctx, traceID, trace)
		if err != nil && firstError == nil {
			firstError = err
		}
		invertedDecisions = append(invertedDecisions, decision)
	}

	if firstError != nil {
		return NewDecisionWithError(fmt.Errorf("policy evaluation error: %w", firstError)), nil
	}

	// Apply the specific precedence rule for this combinator type:
	// "Inverted sampling only if no explicit not-sampling"
	hasExplicitNotSampled := hasNotSampledDecision(normalDecisions)
	if hasExplicitNotSampled {
		// Ignore inverted decisions when there's explicit not-sampling
		return combineWithOrLogic(normalDecisions), nil
	}

	// Combine both normal and inverted with pure OR logic
	allDecisions := append(normalDecisions, invertedDecisions...)
	return combineWithOrLogic(allDecisions), nil
}

// ThresholdAndCombinator implements pure OTEP 250 AND logic using maximum threshold.
type ThresholdAndCombinator struct {
	policies []PolicyEvaluator
}

// NewThresholdAndCombinator creates a new pure OTEP 250 AND combinator.
func NewThresholdAndCombinator(policies []PolicyEvaluator) *ThresholdAndCombinator {
	return &ThresholdAndCombinator{
		policies: policies,
	}
}

// Evaluate implements AND logic by taking the maximum threshold (most restrictive).
func (c *ThresholdAndCombinator) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error) {
	if len(c.policies) == 0 {
		return Sampled, nil // Empty AND is true
	}

	var decisions []Decision
	var firstError error

	// Collect decisions from all policies
	for _, policy := range c.policies {
		decision, err := policy.Evaluate(ctx, traceID, trace)
		if err != nil && firstError == nil {
			firstError = err
		}
		decisions = append(decisions, decision)
	}

	if firstError != nil {
		return NewDecisionWithError(fmt.Errorf("policy evaluation error: %w", firstError)), nil
	}

	// Apply pure AND logic: maximum threshold wins (most restrictive)
	return combineWithAndLogic(decisions), nil
}

// RateLimitedOrCombinator implements OR logic with rate limiting.
type RateLimitedOrCombinator struct {
	policies    []PolicyEvaluator
	rateLimit   int
	sampleCount *int32 // Use atomic operations in practice
}

// NewRateLimitedOrCombinator creates a new rate-limited OR combinator.
func NewRateLimitedOrCombinator(policies []PolicyEvaluator, rateLimit int) *RateLimitedOrCombinator {
	var count int32
	return &RateLimitedOrCombinator{
		policies:    policies,
		rateLimit:   rateLimit,
		sampleCount: &count,
	}
}

// Evaluate implements rate-limited OR logic.
func (c *RateLimitedOrCombinator) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error) {
	if len(c.policies) == 0 {
		return NotSampled, nil
	}

	var decisions []Decision
	var firstError error

	// Collect decisions from all policies
	for _, policy := range c.policies {
		decision, err := policy.Evaluate(ctx, traceID, trace)
		if err != nil && firstError == nil {
			firstError = err
		}
		decisions = append(decisions, decision)
	}

	if firstError != nil {
		return NewDecisionWithError(fmt.Errorf("policy evaluation error: %w", firstError)), nil
	}

	// Apply OR logic first
	orResult := combineWithOrLogic(decisions)

	// Apply rate limiting if result would be sampled
	if orResult.IsSampled() {
		// TODO: Implement proper atomic rate limiting
		// For now, return the OR result without rate limiting
		// In a full implementation, this would check against rate limits
	}

	return orResult, nil
}

// Helper functions for threshold combination logic

// combineWithOrLogic implements pure OR logic by taking minimum threshold.
func combineWithOrLogic(decisions []Decision) Decision {
	if len(decisions) == 0 {
		return NotSampled
	}

	// Start with the maximum threshold (most restrictive)
	minThreshold := sampling.NeverSampleThreshold
	hasError := false
	hasDropped := false

	var firstError error
	var subPolicyDecisions []SubPolicyDecision

	for _, decision := range decisions {
		// Handle error decisions - check both Error field and error attribute
		if decision.IsError() || decision.Error != nil {
			hasError = true
			if firstError == nil {
				firstError = decision.Error
			}
			continue
		}

		// Handle dropped decisions
		if decision.IsDropped() {
			hasDropped = true
			continue
		}

		// Take minimum threshold (most permissive wins in OR)
		if decision.Threshold.Unsigned() < minThreshold.Unsigned() {
			minThreshold = decision.Threshold
		}

		// Collect sub-policy decisions for deferred attribute insertion
		// Each sub-policy contributes its threshold and attribute inserters
		if len(decision.AttributeInserters) > 0 {
			// If the decision has multiple inserters, combine them
			combinedInserter := CombineAttributeInserterFuncs(decision.AttributeInserters...)
			subPolicyDecisions = append(subPolicyDecisions, SubPolicyDecision{
				Threshold:         decision.Threshold,
				AttributeInserter: combinedInserter,
				PolicyName:        "or-subpolicy", // TODO: could get actual policy name if available
			})
		}
	}

	// Precedence: Errors and drops override sampling decisions
	if hasError {
		if firstError != nil {
			return NewDecisionWithError(firstError)
		}
		return ErrorDecision
	}
	if hasDropped {
		return Dropped
	}

	// Create decision with deferred attribute insertion
	result := NewDecisionWithThreshold(minThreshold)
	if len(subPolicyDecisions) > 0 {
		result.AttributeInserters = []AttributeInserter{
			NewDeferredAttributeInserter(subPolicyDecisions, "OR"),
		}
	}
	return result
}

// combineWithAndLogic implements pure AND logic by taking maximum threshold.
func combineWithAndLogic(decisions []Decision) Decision {
	if len(decisions) == 0 {
		return Sampled // Empty AND is true
	}

	// Start with the minimum threshold (most permissive)
	maxThreshold := sampling.AlwaysSampleThreshold
	hasError := false
	hasDropped := false

	var firstError error
	var subPolicyDecisions []SubPolicyDecision

	for _, decision := range decisions {
		// Handle error decisions - check both Error field and error attribute
		if decision.IsError() || decision.Error != nil {
			hasError = true
			if firstError == nil {
				firstError = decision.Error
			}
			continue
		}

		// Handle dropped decisions
		if decision.IsDropped() {
			hasDropped = true
			continue
		}

		// Take maximum threshold (most restrictive wins in AND)
		if decision.Threshold.Unsigned() > maxThreshold.Unsigned() {
			maxThreshold = decision.Threshold
		}

		// Collect sub-policy decisions for deferred attribute insertion
		// Each sub-policy contributes its threshold and attribute inserters
		if len(decision.AttributeInserters) > 0 {
			// If the decision has multiple inserters, combine them
			combinedInserter := CombineAttributeInserterFuncs(decision.AttributeInserters...)
			subPolicyDecisions = append(subPolicyDecisions, SubPolicyDecision{
				Threshold:         decision.Threshold,
				AttributeInserter: combinedInserter,
				PolicyName:        "and-subpolicy", // TODO: could get actual policy name if available
			})
		}
	}

	// Precedence: Errors and drops override sampling decisions
	if hasError {
		if firstError != nil {
			return NewDecisionWithError(firstError)
		}
		return ErrorDecision
	}
	if hasDropped {
		return Dropped
	}

	// Create decision with deferred attribute insertion
	result := NewDecisionWithThreshold(maxThreshold)
	if len(subPolicyDecisions) > 0 {
		result.AttributeInserters = []AttributeInserter{
			NewDeferredAttributeInserter(subPolicyDecisions, "AND"),
		}
	}
	return result
}

// hasNotSampledDecision checks if any decision is explicit NotSampled.
func hasNotSampledDecision(decisions []Decision) bool {
	for _, decision := range decisions {
		if decision.IsNotSampled() {
			return true
		}
	}
	return false
}
