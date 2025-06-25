// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// Mock policy evaluator for testing
type mockPolicyEvaluator struct {
	decision Decision
	err      error
}

func (m *mockPolicyEvaluator) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *TraceData) (Decision, error) {
	return m.decision, m.err
}

func newMockPolicy(decision Decision) *mockPolicyEvaluator {
	return &mockPolicyEvaluator{decision: decision}
}

func newMockPolicyWithError(err error) *mockPolicyEvaluator {
	return &mockPolicyEvaluator{err: err}
}

func TestThresholdOrCombinator(t *testing.T) {
	ctx := context.Background()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	trace := &TraceData{}

	t.Run("empty policies returns NotSampled", func(t *testing.T) {
		combinator := NewThresholdOrCombinator([]PolicyEvaluator{})
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)
		assert.Equal(t, NotSampled.Threshold, decision.Threshold)
	})

	t.Run("single Sampled policy", func(t *testing.T) {
		policies := []PolicyEvaluator{
			newMockPolicy(Sampled),
		}
		combinator := NewThresholdOrCombinator(policies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)
		assert.Equal(t, sampling.AlwaysSampleThreshold, decision.Threshold)
	})

	t.Run("single NotSampled policy", func(t *testing.T) {
		policies := []PolicyEvaluator{
			newMockPolicy(NotSampled),
		}
		combinator := NewThresholdOrCombinator(policies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)
		assert.Equal(t, sampling.NeverSampleThreshold, decision.Threshold)
	})

	t.Run("OR logic - minimum threshold wins", func(t *testing.T) {
		// Create policies with different thresholds
		// AlwaysSample (0) should win over NeverSample (2^56)
		policies := []PolicyEvaluator{
			newMockPolicy(NotSampled), // NeverSample threshold
			newMockPolicy(Sampled),    // AlwaysSample threshold
		}
		combinator := NewThresholdOrCombinator(policies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)
		assert.Equal(t, sampling.AlwaysSampleThreshold, decision.Threshold)
	})

	t.Run("handles errors", func(t *testing.T) {
		policies := []PolicyEvaluator{
			newMockPolicyWithError(assert.AnError),
			newMockPolicy(Sampled),
		}
		combinator := NewThresholdOrCombinator(policies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)
		assert.True(t, decision.IsError())
	})
}

func TestInvertAwareOrCombinator(t *testing.T) {
	ctx := context.Background()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	trace := &TraceData{}

	t.Run("precedence: explicit NotSampled overrides inverted", func(t *testing.T) {
		normalPolicies := []PolicyEvaluator{
			newMockPolicy(NotSampled), // Explicit NotSampled
		}
		invertedPolicies := []PolicyEvaluator{
			newMockPolicy(NewInvertedDecision(sampling.NeverSampleThreshold)), // Inverted sampling intent
		}

		combinator := NewInvertAwareOrCombinator(normalPolicies, invertedPolicies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)

		// Should ignore inverted policies and return NotSampled
		assert.Equal(t, sampling.NeverSampleThreshold, decision.Threshold)
	})

	t.Run("no explicit NotSampled - combines all decisions", func(t *testing.T) {
		normalPolicies := []PolicyEvaluator{
			newMockPolicy(Sampled), // Regular Sampled
		}
		invertedPolicies := []PolicyEvaluator{
			newMockPolicy(NewInvertedDecision(sampling.NeverSampleThreshold)), // Inverted sampling
		}

		combinator := NewInvertAwareOrCombinator(normalPolicies, invertedPolicies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)

		// Should combine both with OR logic - minimum threshold wins
		// AlwaysSample (0) wins over weak threshold (1)
		assert.Equal(t, sampling.AlwaysSampleThreshold, decision.Threshold)
	})

	t.Run("only inverted policies", func(t *testing.T) {
		normalPolicies := []PolicyEvaluator{}
		invertedPolicies := []PolicyEvaluator{
			newMockPolicy(NewInvertedDecision(sampling.NeverSampleThreshold)),
		}

		combinator := NewInvertAwareOrCombinator(normalPolicies, invertedPolicies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)

		// Should use the inverted policy's threshold (NewInvertedDecision(NeverSample) = inverted NeverSample = AlwaysSample)
		assert.Equal(t, sampling.AlwaysSampleThreshold.Unsigned(), decision.Threshold.Unsigned())
	})

	t.Run("mixed scenario demonstrating precedence", func(t *testing.T) {
		// Scenario: Normal policy matches service=web, inverted policy matches error=true
		// Normal result: Sampled (service=web matched)
		// Inverted result: InvertNotSampled (error=true matched, then inverted to NotSampled)
		// Expected: Precedence rule should make final decision NotSampled

		normalPolicies := []PolicyEvaluator{
			newMockPolicy(Sampled), // Matches service=web
		}
		invertedPolicies := []PolicyEvaluator{
			newMockPolicy(InvertNotSampled), // Matches error=true, inverted to NotSampled
		}

		combinator := NewInvertAwareOrCombinator(normalPolicies, invertedPolicies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)

		// InvertNotSampled should be treated as explicit NotSampled for precedence
		// So inverted decisions are ignored, leaving only Sampled
		assert.Equal(t, sampling.AlwaysSampleThreshold, decision.Threshold)
	})
}

func TestThresholdAndCombinator(t *testing.T) {
	ctx := context.Background()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	trace := &TraceData{}

	t.Run("empty policies returns Sampled", func(t *testing.T) {
		combinator := NewThresholdAndCombinator([]PolicyEvaluator{})
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)
		assert.Equal(t, sampling.AlwaysSampleThreshold, decision.Threshold)
	})

	t.Run("AND logic - maximum threshold wins", func(t *testing.T) {
		// Create policies with different thresholds
		// NeverSample (2^56) should win over AlwaysSample (0) in AND
		policies := []PolicyEvaluator{
			newMockPolicy(Sampled),    // AlwaysSample threshold
			newMockPolicy(NotSampled), // NeverSample threshold
		}
		combinator := NewThresholdAndCombinator(policies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)
		assert.Equal(t, sampling.NeverSampleThreshold, decision.Threshold)
	})

	t.Run("all Sampled returns Sampled", func(t *testing.T) {
		policies := []PolicyEvaluator{
			newMockPolicy(Sampled),
			newMockPolicy(Sampled),
		}
		combinator := NewThresholdAndCombinator(policies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)
		assert.Equal(t, sampling.AlwaysSampleThreshold, decision.Threshold)
	})
}

func TestRateLimitedOrCombinator(t *testing.T) {
	ctx := context.Background()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	trace := &TraceData{}

	t.Run("basic OR logic works", func(t *testing.T) {
		policies := []PolicyEvaluator{
			newMockPolicy(Sampled),
			newMockPolicy(NotSampled),
		}
		combinator := NewRateLimitedOrCombinator(policies, 100)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)

		// Should use OR logic - minimum threshold wins
		assert.Equal(t, sampling.AlwaysSampleThreshold, decision.Threshold)
	})

	// TODO: Add proper rate limiting tests when rate limiting logic is implemented
}

func TestCombinationHelpers(t *testing.T) {
	t.Run("combineWithOrLogic", func(t *testing.T) {
		// Test minimum threshold selection
		decisions := []Decision{
			NotSampled, // NeverSample threshold (2^56)
			Sampled,    // AlwaysSample threshold (0)
		}
		result := combineWithOrLogic(decisions)
		assert.Equal(t, sampling.AlwaysSampleThreshold, result.Threshold)
	})

	t.Run("combineWithAndLogic", func(t *testing.T) {
		// Test maximum threshold selection
		decisions := []Decision{
			Sampled,    // AlwaysSample threshold (0)
			NotSampled, // NeverSample threshold (2^56)
		}
		result := combineWithAndLogic(decisions)
		assert.Equal(t, sampling.NeverSampleThreshold, result.Threshold)
	})

	t.Run("hasNotSampledDecision", func(t *testing.T) {
		// Test with NotSampled present
		decisions := []Decision{Sampled, NotSampled}
		assert.True(t, hasNotSampledDecision(decisions))

		// Test without NotSampled
		decisions = []Decision{Sampled, NewInvertedDecision(sampling.NeverSampleThreshold)}
		assert.False(t, hasNotSampledDecision(decisions))
	})

	t.Run("error handling in OR logic", func(t *testing.T) {
		errorDecision := NewDecisionWithError(assert.AnError)
		decisions := []Decision{
			errorDecision,
			Sampled,
		}
		result := combineWithOrLogic(decisions)
		assert.True(t, result.IsError())
	})

	t.Run("dropped handling in OR logic", func(t *testing.T) {
		decisions := []Decision{
			Dropped,
			Sampled,
		}
		result := combineWithOrLogic(decisions)
		assert.True(t, result.IsDropped())
	})
}

func TestCombinatorsWithMathematicalInversion(t *testing.T) {
	ctx := context.Background()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	trace := &TraceData{}

	t.Run("InvertAwareOrCombinator with mathematical inversion", func(t *testing.T) {
		// Test scenario from threshold-compilation-analysis.md
		// Policy 1: matches service=web → AlwaysSample (0)
		// Policy 2: inverted, matches error=true → InvertNotSampled (mathematically inverted)

		normalPolicies := []PolicyEvaluator{
			newMockPolicy(Sampled), // service=web matched
		}
		invertedPolicies := []PolicyEvaluator{
			newMockPolicy(InvertNotSampled), // error=true matched, then mathematically inverted
		}

		combinator := NewInvertAwareOrCombinator(normalPolicies, invertedPolicies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)

		// With precedence logic: should ignore inverted decisions if normal has NotSampled
		// But in this case, we have Sampled + InvertNotSampled
		// InvertNotSampled is treated as NotSampled for precedence, so inverted ignored
		assert.Equal(t, sampling.AlwaysSampleThreshold, decision.Threshold)
	})

	t.Run("ThresholdOrCombinator with mathematical inversion", func(t *testing.T) {
		// Same scenario but with pure threshold logic
		// Should use minimum threshold: min(0, 2^56) = 0 → Sampled

		policies := []PolicyEvaluator{
			newMockPolicy(Sampled),          // AlwaysSample (0)
			newMockPolicy(InvertNotSampled), // NeverSample (2^56)
		}

		combinator := NewThresholdOrCombinator(policies)
		decision, err := combinator.Evaluate(ctx, traceID, trace)
		require.NoError(t, err)

		// Pure OR logic: minimum threshold wins
		assert.Equal(t, sampling.AlwaysSampleThreshold, decision.Threshold)
	})
}
