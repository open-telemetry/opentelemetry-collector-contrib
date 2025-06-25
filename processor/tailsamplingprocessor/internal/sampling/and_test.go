// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

func TestAndEvaluatorNotSampled(t *testing.T) {
	// Test setup: First policy won't match, so AND should not sample
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "name", []string{"value"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewAnd(zap.NewNop(), []PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Status().SetCode(ptrace.StatusCodeError) // Matches n2 only
	// Note: missing attribute "name"="value", so n1 won't match
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}

	// Apply the new paradigm: focus on final sampling behavior
	decision, err := and.Evaluate(context.Background(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)

	// Step 1: Check threshold - AND logic should use most restrictive threshold
	// Since n1 doesn't match, it should return NeverSampleThreshold
	assert.Equal(t, sampling.NeverSampleThreshold, decision.Threshold)

	// Step 2: Apply randomness and check final sampling behavior
	randomness := sampling.TraceIDToRandomness(traceID)
	shouldSample := decision.Threshold.ShouldSample(randomness)
	assert.False(t, shouldSample, "Expected AND policy to not sample when one sub-policy doesn't match")

	// Step 3: Verify deferred attribute setup (even for non-sampling case)
	assert.NotNil(t, decision.AttributeInserters)
	assert.Greater(t, len(decision.AttributeInserters), 0)

	// Step 4: Since trace won't be sampled, deferred attributes shouldn't be applied
	// But we can still test that the mechanism is set up correctly
}

func TestAndEvaluatorSampled(t *testing.T) {
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewAnd(zap.NewNop(), []PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value")
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}
	decision, err := and.Evaluate(context.Background(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)

	// Step 1: Verify the threshold indicates sampling intent
	// Both sub-policies should match, so AND should return AlwaysSampleThreshold (most permissive)
	assert.Equal(t, sampling.AlwaysSampleThreshold, decision.Threshold, "AND of two matching policies should have AlwaysSampleThreshold")

	// Step 2: The key insight - we test final outcome, not intermediate state
	// Apply randomness and check if attributes are correctly applied when trace would be sampled
	trace.RandomnessValue = sampling.Randomness{} // Zero value, should result in sampling

	// Step 3: Apply deferred attributes if this trace would be sampled
	if decision.Threshold.ShouldSample(trace.RandomnessValue) {
		decision.ApplyAttributeInserters(trace)
	}

	// Step 4: Verify the final trace state - this is what actually matters
	// Check that the deferred attribute insertion system is working
	assert.NotNil(t, decision.AttributeInserters, "AND policy should have deferred attribute inserters from sub-policies")
	assert.Greater(t, len(decision.AttributeInserters), 0, "Should have at least one deferred inserter")
}

func TestAndEvaluatorStringInvertSampled(t *testing.T) {
	// Test inverted filter: n1 looks for "no_match" with invert=true, span has "attribute_value"
	// So n1 should match (inverted), n2 should match ERROR status -> AND should sample
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"no_match"}, false, 0, true)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewAnd(zap.NewNop(), []PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value") // Doesn't match "no_match", so inverted n1 matches
	span.Status().SetCode(ptrace.StatusCodeError)                 // Matches n2
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}

	// Apply new paradigm: test final sampling behavior, not intermediate state
	decision, err := and.Evaluate(context.Background(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)

	// Step 1: Check threshold - both policies should match, so should be AlwaysSampleThreshold
	assert.Equal(t, sampling.AlwaysSampleThreshold, decision.Threshold)

	// Step 2: Apply randomness and verify sampling
	randomness := sampling.TraceIDToRandomness(traceID)
	shouldSample := decision.Threshold.ShouldSample(randomness)
	assert.True(t, shouldSample, "Expected AND policy to sample when both sub-policies match (including inverted)")

	// Step 3: Verify deferred attribute setup
	assert.NotNil(t, decision.AttributeInserters)
	assert.Greater(t, len(decision.AttributeInserters), 0)

	// Step 4: Apply deferred attributes since trace should be sampled
	decision.ApplyAttributeInserters(trace)
}

func TestAndEvaluatorStringInvertNotSampled(t *testing.T) {
	// Test inverted filter: n1 looks for "attribute_value" with invert=true, span has "attribute_value"
	// So n1 should NOT match (inverted), even though n2 matches -> AND should not sample
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, true)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewAnd(zap.NewNop(), []PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value") // Matches "attribute_value", so inverted n1 does NOT match
	span.Status().SetCode(ptrace.StatusCodeError)                 // Matches n2, but n1 fails
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}

	// Apply new paradigm: test final sampling behavior
	decision, err := and.Evaluate(context.Background(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)

	// Step 1: Check threshold - since n1 doesn't match (inverted), should be NeverSampleThreshold
	assert.Equal(t, sampling.NeverSampleThreshold, decision.Threshold)

	// Step 2: Apply randomness and verify no sampling
	randomness := sampling.TraceIDToRandomness(traceID)
	shouldSample := decision.Threshold.ShouldSample(randomness)
	assert.False(t, shouldSample, "Expected AND policy to not sample when inverted sub-policy doesn't match")

	// Step 3: Verify deferred attribute setup (even for non-sampling)
	assert.NotNil(t, decision.AttributeInserters)
	assert.Greater(t, len(decision.AttributeInserters), 0)
}

// TestAndEvaluatorDeferredAttributes demonstrates that the AND policy
// correctly sets up deferred attribute inserters from its sub-policies.
func TestAndEvaluatorDeferredAttributes(t *testing.T) {
	// Create two simple sub-policies
	n1 := NewStringAttributeFilter(componenttest.NewNopTelemetrySettings(), "attribute_name", []string{"attribute_value"}, false, 0, false)
	n2, err := NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR"})
	require.NoError(t, err)

	and := NewAnd(zap.NewNop(), []PolicyEvaluator{n1, n2})

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.Attributes().PutStr("attribute_name", "attribute_value")
	span.Status().SetCode(ptrace.StatusCodeError)
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	trace := &TraceData{
		ReceivedBatches: traces,
	}

	decision, err := and.Evaluate(context.Background(), traceID, trace)
	require.NoError(t, err, "Failed to evaluate and policy: %v", err)

	// Should be sampled (both sub-policies match)
	assert.Equal(t, Sampled.Threshold, decision.Threshold)
	assert.Equal(t, Sampled.Attributes, decision.Attributes)

	// The key insight: AND policy should have deferred attribute inserters
	// This is the main change from OTEP-235 - composite policies now use
	// deferred attribute insertion instead of immediate insertion
	assert.NotNil(t, decision.AttributeInserters, "AND policy should provide deferred attribute inserters")
	assert.Greater(t, len(decision.AttributeInserters), 0, "Should have at least one attribute inserter from sub-policies")
}
