// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// TestPhase6_PerSpanThresholdTracking tests the core Phase 6 functionality
func TestPhase6_PerSpanThresholdTracking(t *testing.T) {
	tsm := NewTraceStateManager()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	// Create a trace with multiple spans having different original thresholds
	trace := createTraceWithMixedThresholds()

	// Initialize OTEP 235 fields with per-span tracking
	tsm.InitializeTraceData(context.Background(), traceID, trace)

	// Verify per-span threshold tracking was initialized
	assert.NotNil(t, trace.SpanThresholds, "SpanThresholds should be initialized")
	assert.Equal(t, 3, len(trace.SpanThresholds), "Should track 3 spans")

	// Verify individual span threshold information
	spans := getSpansFromTrace(trace)
	for _, span := range spans {
		spanID := span.SpanID()
		spanInfo, exists := trace.SpanThresholds[spanID]
		require.True(t, exists, "Span %s should have threshold info", spanID.String())
		assert.Equal(t, spanID, spanInfo.SpanID, "SpanID should match")
		assert.Nil(t, spanInfo.FinalThreshold, "FinalThreshold should be nil initially")
	}

	// Simulate a sampling decision with a specific final threshold
	finalThreshold := sampling.AlwaysSampleThreshold // th:0 (100% sampling)
	err := tsm.UpdateTraceState(trace, finalThreshold)
	require.NoError(t, err, "UpdateTraceState should not error")

	// Verify final thresholds were set for all spans according to OTEP 235 equalizing pattern
	spanIndex := 0
	for spanID, spanInfo := range trace.SpanThresholds {
		require.NotNil(t, spanInfo.FinalThreshold, "FinalThreshold should be set after update")

		// Each span should get the more restrictive of its original threshold vs constraint
		if spanIndex == 0 {
			// Root span: th:0 vs th:0 -> th:0 (no change)
			assert.Equal(t, finalThreshold, *spanInfo.FinalThreshold, "Root span should match constraint")
		} else {
			// Child spans: th:c vs th:0 -> th:c (more restrictive original preserved)
			originalThreshold, _ := sampling.TValueToThreshold("c")
			assert.Equal(t, originalThreshold, *spanInfo.FinalThreshold, "Child span should keep more restrictive original threshold")
		}
		spanIndex++
		_ = spanID // Avoid unused variable warning
	}
}

// TestPhase6_AdjustedCountCalculation tests the per-span adjusted count calculation
func TestPhase6_AdjustedCountCalculation(t *testing.T) {
	tsm := NewTraceStateManager()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	// Create a trace with spans having specific thresholds matching the worked example
	trace := createWorkedExampleTrace()

	// Initialize OTEP 235 fields
	tsm.InitializeTraceData(context.Background(), traceID, trace)

	// Simulate rate limiting policy decision: final threshold of ~1% (th:fd70a3d70a3d7)
	finalThreshold, err := sampling.ProbabilityToThreshold(0.01) // ~1% sampling
	require.NoError(t, err, "Should create 1% threshold")

	// Update trace state with final threshold
	err = tsm.UpdateTraceState(trace, finalThreshold)
	require.NoError(t, err, "UpdateTraceState should not error")

	// Get spans to test adjusted count calculations
	spans := getSpansFromTrace(trace)
	require.Equal(t, 3, len(spans), "Should have 3 spans")

	// Test adjusted count calculations per the worked example
	for _, span := range spans {
		spanID := span.SpanID()
		adjustedCount := tsm.CalculateSpanAdjustedCount(spanID, trace)

		traceState := span.TraceState().AsRaw()
		if traceState == "ot=th:0" {
			// Root span: original th:0 (100%) → final th:fd... (~1%)
			// Adjusted count = 100% / 1% = 100.0
			assert.InDelta(t, 100.0, adjustedCount, 5.0, "Root span should have adjusted count ~100")
		} else if traceState == "ot=th:c" {
			// Child span: original th:c (25%) → final th:fd... (~1%)
			// Adjusted count = 25% / 1% = 25.0
			assert.InDelta(t, 25.0, adjustedCount, 2.0, "Child span should have adjusted count ~25")
		}
	}
}

// TestPhase6_WorkedExampleScenario tests the complete worked example scenario
func TestPhase6_WorkedExampleScenario(t *testing.T) {
	tsm := NewTraceStateManager()

	// Test case 1: Error trace (should get th:0)
	errorTrace := createErrorTrace()
	traceID1 := pcommon.TraceID([16]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	tsm.InitializeTraceData(context.Background(), traceID1, errorTrace)

	// Error policy decides to sample at 100%
	errorFinalThreshold := sampling.AlwaysSampleThreshold
	err := tsm.UpdateTraceState(errorTrace, errorFinalThreshold)
	require.NoError(t, err)

	// Verify error trace spans all get adjusted count = 1.0
	errorSpans := getSpansFromTrace(errorTrace)
	for _, span := range errorSpans {
		adjustedCount := tsm.CalculateSpanAdjustedCount(span.SpanID(), errorTrace)
		assert.Equal(t, 1.0, adjustedCount, "Error trace spans should have adjusted count = 1.0")
	}

	// Test case 2: Normal trace (should get ~1% final threshold)
	normalTrace := createNormalTrace()
	traceID2 := pcommon.TraceID([16]byte{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	tsm.InitializeTraceData(context.Background(), traceID2, normalTrace)

	// Rate limiting policy decides to sample at ~1%
	normalFinalThreshold, err := sampling.ProbabilityToThreshold(0.01)
	require.NoError(t, err)
	err = tsm.UpdateTraceState(normalTrace, normalFinalThreshold)
	require.NoError(t, err)

	// Verify normal trace spans get appropriate adjusted counts
	normalSpans := getSpansFromTrace(normalTrace)
	for _, span := range normalSpans {
		adjustedCount := tsm.CalculateSpanAdjustedCount(span.SpanID(), normalTrace)

		traceState := span.TraceState().AsRaw()
		if traceState == "ot=th:0" {
			// Root span: 100% → 1% = 100x adjustment
			assert.InDelta(t, 100.0, adjustedCount, 5.0, "Normal root span adjusted count")
		} else if traceState == "ot=th:c" {
			// Child span: 25% → 1% = 25x adjustment
			assert.InDelta(t, 25.0, adjustedCount, 2.0, "Normal child span adjusted count")
		}
	}
}

// TestPhase6_SpanThresholdAccess tests the external access to span threshold information
func TestPhase6_SpanThresholdAccess(t *testing.T) {
	tsm := NewTraceStateManager()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	trace := createTraceWithMixedThresholds()
	tsm.InitializeTraceData(context.Background(), traceID, trace)

	// Get span threshold information
	spanThresholds := tsm.GetSpanThresholds(trace)
	assert.Equal(t, 3, len(spanThresholds), "Should return threshold info for 3 spans")

	// Verify returned data is a copy (modifications don't affect original)
	for spanID, info := range spanThresholds {
		info.FinalThreshold = &sampling.AlwaysSampleThreshold // Modify copy

		// Original should be unchanged
		originalInfo := trace.SpanThresholds[spanID]
		assert.Nil(t, originalInfo.FinalThreshold, "Original should not be modified")
	}
}

// Helper functions for test data creation

func createTraceWithMixedThresholds() *TraceData {
	traces := ptrace.NewTraces()

	// Add root span with th:0 (100% sampling)
	rs1 := traces.ResourceSpans().AppendEmpty()
	ss1 := rs1.ScopeSpans().AppendEmpty()
	span1 := ss1.Spans().AppendEmpty()
	span1.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span1.SetSpanID(pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0}))
	span1.SetName("root-span")
	span1.TraceState().FromRaw("ot=th:0")

	// Add child span with th:c (25% sampling)
	span2 := ss1.Spans().AppendEmpty()
	span2.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span2.SetSpanID(pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0}))
	span2.SetName("child-span-1")
	span2.TraceState().FromRaw("ot=th:c")

	// Add another child span with th:c (25% sampling)
	span3 := ss1.Spans().AppendEmpty()
	span3.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span3.SetSpanID(pcommon.SpanID([8]byte{3, 0, 0, 0, 0, 0, 0, 0}))
	span3.SetName("child-span-2")
	span3.TraceState().FromRaw("ot=th:c")

	return &TraceData{
		ArrivalTime:     time.Now(),
		SpanCount:       &atomic.Int64{},
		ReceivedBatches: traces,
	}
}

func createWorkedExampleTrace() *TraceData {
	return createTraceWithMixedThresholds() // Same structure as worked example
}

func createErrorTrace() *TraceData {
	trace := createTraceWithMixedThresholds()

	// Mark one span as having an error status
	spans := getSpansFromTrace(trace)
	if len(spans) > 0 {
		spans[0].Status().SetCode(ptrace.StatusCodeError)
		spans[0].Status().SetMessage("Test error")
	}

	return trace
}

func createNormalTrace() *TraceData {
	return createTraceWithMixedThresholds() // Normal trace with no errors
}

func getSpansFromTrace(trace *TraceData) []ptrace.Span {
	var spans []ptrace.Span
	batches := trace.ReceivedBatches

	for i := 0; i < batches.ResourceSpans().Len(); i++ {
		rs := batches.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				spans = append(spans, ss.Spans().At(k))
			}
		}
	}

	return spans
}
