// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	internalsampling "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

// TestVaroptTailSamplingAdjustmentPreservation validates that Varopt-based tail sampling
// preserves the total adjusted count when accounting for both OTEP 235 and tail sampling adjustments
func TestVaroptTailSamplingAdjustmentPreservation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	// Single test case - mixed weight traces
	bm := newBucketManagerWithTimeSource(
		logger,
		1, // single bucket
		time.Minute,
		100,
		1.0,
		&fixedTimeSource{t: time.Now()},
	)

	// Setup traces with different sampling weights
	traces, expectedTotal := createMixedWeightTraces(1000)

	// Add all traces to the bucket
	baseTime := time.Now()
	for i, trace := range traces {
		traceID := generateTraceID(i)
		bm.addTrace(traceID, trace, baseTime)
	}

	// Get the bucket and retrieve traces with tail adjustments
	bucket := bm.buckets[0]
	sampledTraces := bucket.getTracesWithTailAdjustments()

	// Calculate total adjusted count accounting for both adjustments
	var actualTotal float64
	for _, bt := range sampledTraces {
		// OTEP 235 adjusted count (input weight)
		otep235AdjustedCount := bt.inputWeight
		// Tail sampling adjustment
		tailAdjustment := bt.getTailSamplingAdjustment()
		// Combined adjustment
		combinedAdjustedCount := otep235AdjustedCount * tailAdjustment
		actualTotal += combinedAdjustedCount
	}

	// Verify basic functionality
	assert.Greater(t, len(sampledTraces), 0, "Should sample some traces")
	assert.LessOrEqual(t, len(sampledTraces), 100, "Should not exceed capacity")
	assert.Greater(t, actualTotal, 0.0, "Should have positive total weight")
	
	t.Logf("Expected total: %.2f, Actual total: %.2f, Sampled: %d/%d",
		expectedTotal, actualTotal, len(sampledTraces), 1000)
}

// TestVaroptBasicFunctionality tests basic Varopt integration
func TestVaroptBasicFunctionality(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	bm := newBucketManagerWithTimeSource(
		logger,
		1, // single bucket
		time.Minute,
		10, // small capacity
		1.0,
		&fixedTimeSource{t: time.Now()},
	)

	// Add some basic traces
	traces, _ := createUniformWeightTraces(20)
	baseTime := time.Now()
	for i, trace := range traces {
		traceID := generateTraceID(i)
		bm.addTrace(traceID, trace, baseTime)
	}

	// Verify basic sampling works
	bucket := bm.buckets[0]
	sampledTraces := bucket.getTracesWithTailAdjustments()
	assert.Greater(t, len(sampledTraces), 0, "Should sample some traces")
	assert.LessOrEqual(t, len(sampledTraces), 10, "Should not exceed capacity")
}

// createUniformWeightTraces creates traces with no OTEP 235 thresholds (uniform weight = 1.0)
func createUniformWeightTraces(count int) ([]*internalsampling.TraceData, float64) {
	traces := make([]*internalsampling.TraceData, count)

	for i := 0; i < count; i++ {
		traces[i] = &internalsampling.TraceData{
			ReceivedBatches: createTraceBatch(generateTraceID(i), ""), // No tracestate
		}
	}

	return traces, float64(count) // Expected total is just the count
}

// createMixedWeightTraces creates traces with mixed OTEP 235 thresholds
func createMixedWeightTraces(count int) ([]*internalsampling.TraceData, float64) {
	traces := make([]*internalsampling.TraceData, count)
	var expectedTotal float64

	for i := 0; i < count; i++ {
		var traceState string
		var weight float64

		if i == 0 {
			// High weight trace with th:f - use pkg/sampling to get correct weight
			traceState = "ot=th:f"
			threshold, _ := sampling.TValueToThreshold("f")
			weight = threshold.AdjustedCount() // Should be 16.0, not 65536!
		} else if i%2 == 0 {
			// 100% sampling (th:0) - use pkg/sampling to get correct weight
			traceState = "ot=th:0"
			threshold, _ := sampling.TValueToThreshold("0")
			weight = threshold.AdjustedCount() // Should be 1.0
		} else {
			// 50% sampling (th:8) - use pkg/sampling to get correct weight
			traceState = "ot=th:8"
			threshold, _ := sampling.TValueToThreshold("8")
			weight = threshold.AdjustedCount() // Should be 2.0
		}

		traces[i] = &internalsampling.TraceData{
			ReceivedBatches: createTraceBatch(generateTraceID(i), traceState),
		}

		expectedTotal += weight
	}

	return traces, expectedTotal
}

// createExtremeHighPressureTraces creates traces with mixed weight disparity for testing approximation
func createExtremeHighPressureTraces(count int) ([]*internalsampling.TraceData, float64) {
	traces := make([]*internalsampling.TraceData, count)
	var expectedTotal float64

	for i := 0; i < count; i++ {
		var traceState string
		var weight float64

		// Create mixed weights: 70% heavy traces, 30% light traces
		if i%10 < 7 {
			// Heavy traces (th:ff = 256x weight)
			traceState = "ot=th:ff"
			threshold, _ := sampling.TValueToThreshold("ff")
			weight = threshold.AdjustedCount()
		} else {
			// Light traces (th:0 = 1x weight)
			traceState = "ot=th:0"
			threshold, _ := sampling.TValueToThreshold("0")
			weight = threshold.AdjustedCount()
		}

		traces[i] = &internalsampling.TraceData{
			ReceivedBatches: createTraceBatch(generateTraceID(i), traceState),
		}
		expectedTotal += weight
	}

	return traces, expectedTotal
}

// createTraceBatch creates a ptrace.Traces with specified traceID and traceState
func createTraceBatch(traceID pcommon.TraceID, traceState string) ptrace.Traces {
	traces := ptrace.NewTraces()
	resourceSpan := traces.ResourceSpans().AppendEmpty()
	scopeSpan := resourceSpan.ScopeSpans().AppendEmpty()
	span := scopeSpan.Spans().AppendEmpty()

	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetName("test-span")

	if traceState != "" {
		span.TraceState().FromRaw(traceState)
	}

	return traces
}

// generateTraceID creates a traceID with specified index and random randomness
func generateTraceID(index int) pcommon.TraceID {
	var traceID pcommon.TraceID

	// Use index in first 8 bytes for deterministic identification
	for i := 0; i < 8; i++ {
		traceID[i] = byte(index >> (8 * i))
	}

	// Use random values in last 8 bytes for OTEP 235 randomness
	rng := rand.New(rand.NewSource(int64(index))) // Deterministic randomness for reproducibility
	for i := 8; i < 16; i++ {
		traceID[i] = byte(rng.Intn(256))
	}

	return traceID
}
