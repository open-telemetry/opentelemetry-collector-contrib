// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	internalsampling "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

// TestBottomKEstimatorAccuracy tests the Bottom-K estimator formula with uniformly distributed randomness
func TestBottomKEstimatorAccuracy(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Test parameters
	totalTraces := 1000
	maxRandomness := uint64(0x00FFFFFFFFFFFFFF) // 56-bit max value

	// Test different sample sizes to verify the estimator accuracy
	testCases := []struct {
		name       string
		sampleSize int
		tolerance  float64 // Allowed percentage error
	}{
		{"Sample_100", 100, 0.15}, // 15% tolerance for smaller samples
		{"Sample_250", 250, 0.10}, // 10% tolerance
		{"Sample_500", 500, 0.08}, // 8% tolerance
		{"Sample_750", 750, 0.06}, // 6% tolerance for larger samples
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create bucket manager for testing
			bm := newBucketManagerWithTimeSource(
				logger,
				1,                     // single bucket
				time.Minute,           // doesn't matter for this test
				uint64(tc.sampleSize), // max traces per bucket = sample size
				1.0,                   // no overage factor
				&fixedTimeSource{time.Now()},
			)
			// Generate traces with uniformly distributed randomness values
			traces := generateUniformRandomnessTraces(totalTraces, maxRandomness)

			// Add all traces to the bucket (this will trigger Bottom-K compaction)
			bucket := bm.buckets[0]

			for _, traceItem := range traces {
				bucketTrace := &bucketTrace{
					traceID:         traceItem.traceID,
					randomnessValue: traceItem.randomness,
					trace:           traceItem.traceData,
				}

				bucket.mutex.Lock()
				bucket.traces = append(bucket.traces, bucketTrace)

				// Trigger compaction when we exceed the limit
				if len(bucket.traces) > tc.sampleSize {
					bm.performBottomKCompaction(bucket)
				}
				bucket.mutex.Unlock()
			}

			// Get the final sampled traces
			sampledTraces := bucket.getAllTraces()
			require.Equal(t, tc.sampleSize, len(sampledTraces), "Should have exactly the sample size")

			t.Logf("Found %d sampled traces", len(sampledTraces))

			// Calculate the minimum randomness value among kept traces
			minRandomness := uint64(math.MaxUint64)
			for _, sampledTrace := range sampledTraces {
				if sampledTrace.randomnessValue < minRandomness {
					minRandomness = sampledTrace.randomnessValue
				}
			}

			t.Logf("Minimum randomness value: %d", minRandomness)

			// Calculate the Bottom-K threshold using our estimator
			threshold := bm.calculateBottomKThreshold(minRandomness, uint64(tc.sampleSize))

			t.Logf("Calculated threshold: %s (Probability: %f, AdjustedCount: %f)",
				threshold.TValue(), threshold.Probability(), threshold.AdjustedCount())

			// For each sampled trace, calculate its adjusted count and sum them
			totalAdjustedCount := 0.0
			for range sampledTraces {
				// The adjusted count for each trace should be based on the threshold
				adjustedCount := threshold.AdjustedCount()
				totalAdjustedCount += adjustedCount
			}

			// The total adjusted count should approximately equal the original population size
			expectedTotal := float64(totalTraces)
			actualTotal := totalAdjustedCount

			// Calculate the error percentage
			errorPercent := math.Abs(actualTotal-expectedTotal) / expectedTotal

			t.Logf("Sample size: %d, Expected total: %.2f, Actual total: %.2f, Error: %.2f%%",
				tc.sampleSize, expectedTotal, actualTotal, errorPercent*100)

			// Verify the estimator accuracy
			assert.True(t, errorPercent <= tc.tolerance,
				"Bottom-K estimator error %.2f%% exceeds tolerance %.2f%% for sample size %d",
				errorPercent*100, tc.tolerance*100, tc.sampleSize)

			// Additional verification: check that the threshold is reasonable
			expectedSamplingRate := float64(tc.sampleSize) / float64(totalTraces)
			actualSamplingRate := threshold.Probability()

			t.Logf("Expected sampling rate: %.3f, Actual sampling rate: %.3f",
				expectedSamplingRate, actualSamplingRate)

			// The sampling rate should be reasonably close (within 50% tolerance for this check)
			samplingRateError := math.Abs(actualSamplingRate-expectedSamplingRate) / expectedSamplingRate
			assert.True(t, samplingRateError <= 0.5,
				"Sampling rate error %.2f%% is too high", samplingRateError*100)
		})
	}
}

// TestBottomKEstimatorEdgeCases tests edge cases in the Bottom-K estimator
func TestBottomKEstimatorEdgeCases(t *testing.T) {
	logger := zaptest.NewLogger(t)
	bm := newBucketManagerWithTimeSource(
		logger, 1, time.Minute, 100, 1.0, &fixedTimeSource{time.Now()})

	testCases := []struct {
		name              string
		k                 uint64
		minRandomness     uint64
		expectedThreshold sampling.Threshold
	}{
		{
			name:              "Zero K",
			k:                 0,
			minRandomness:     0x00FFFFFFFFFFFF00,
			expectedThreshold: sampling.NeverSampleThreshold,
		},
		{
			name:              "Zero randomness",
			k:                 10,
			minRandomness:     0,
			expectedThreshold: sampling.AlwaysSampleThreshold,
		},
		{
			name:              "Maximum randomness",
			k:                 10,
			minRandomness:     0x00FFFFFFFFFFFFFF,
			expectedThreshold: sampling.NeverSampleThreshold, // Should be very restrictive
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			threshold := bm.calculateBottomKThreshold(tc.minRandomness, tc.k)
			assert.Equal(t, tc.expectedThreshold, threshold)
		})
	}
}

// Helper types and functions

type testTrace struct {
	traceID    pcommon.TraceID
	randomness uint64
	traceData  *internalsampling.TraceData
}

type fixedTimeSource struct {
	t time.Time
}

func (f *fixedTimeSource) Now() time.Time {
	return f.t
}

// generateUniformRandomnessTraces creates traces with evenly distributed randomness values
func generateUniformRandomnessTraces(count int, maxRandomness uint64) []testTrace {
	traces := make([]testTrace, count)

	for i := 0; i < count; i++ {
		// Calculate uniformly distributed randomness value
		randomness := uint64(float64(i) / float64(count-1) * float64(maxRandomness))

		// Create a trace ID that encodes this randomness value
		traceID := pcommon.NewTraceIDEmpty()

		// Encode the randomness in the lower 8 bytes of the trace ID
		// This matches how extractRandomnessValue works
		for j := 0; j < 8; j++ {
			traceID[8+j] = byte(randomness >> (8 * j))
		}

		// Create trace data with TraceState containing the randomness value
		traceData := &internalsampling.TraceData{
			ReceivedBatches: createTraceWithRandomness(traceID, randomness),
		}

		traces[i] = testTrace{
			traceID:    traceID,
			randomness: randomness,
			traceData:  traceData,
		}
	}

	return traces
}

// createTraceWithRandomness creates a trace with explicit randomness in TraceState
func createTraceWithRandomness(traceID pcommon.TraceID, randomness uint64) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.NewSpanIDEmpty())
	span.SetName("test-span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Millisecond)))

	// Set TraceState with explicit randomness value (ot=rv:xxxxxxxxxxxxx)
	// Format the randomness as a 14-character hex string (56 bits)
	randomnessHex := fmt.Sprintf("%014x", randomness)
	traceState := fmt.Sprintf("ot=rv:%s", randomnessHex)
	span.TraceState().FromRaw(traceState)

	return traces
}

// TestBottomKCompactionBehavior tests that the compaction correctly keeps the highest randomness values
func TestBottomKCompactionBehavior(t *testing.T) {
	logger := zaptest.NewLogger(t)

	bm := newBucketManagerWithTimeSource(
		logger,
		1, // single bucket
		time.Minute,
		10, // keep only 10 traces
		1.0,
		&fixedTimeSource{time.Now()},
	)

	bucket := bm.buckets[0]

	// Add 20 traces with known randomness values
	expectedKeptValues := []uint64{
		0x00FFFFFFFFFFFFFF, // highest
		0x00FFFFFFFFFFFE00,
		0x00FFFFFFFFFFE000,
		0x00FFFFFFFFFF0000,
		0x00FFFFFFFF000000,
		0x00FFFFFFF0000000,
		0x00FFFFFF00000000,
		0x00FFFFF000000000,
		0x00FFFF0000000000,
		0x00FFF00000000000, // 10th highest
	}

	// Add these in random order plus some lower values
	allValues := append([]uint64{}, expectedKeptValues...)
	allValues = append(allValues, []uint64{
		0x00FF000000000000,
		0x00F0000000000000,
		0x0080000000000000,
		0x0040000000000000,
		0x0020000000000000,
		0x0010000000000000,
		0x0008000000000000,
		0x0004000000000000,
		0x0002000000000000,
		0x0001000000000000, // lowest
	}...)

	// Add all traces
	for i, randomness := range allValues {
		traceID := pcommon.NewTraceIDEmpty()
		traceID[8] = byte(i) // Just to make them unique

		bucketTrace := &bucketTrace{
			traceID:         traceID,
			randomnessValue: randomness,
			trace:           &internalsampling.TraceData{},
		}

		bucket.mutex.Lock()
		bucket.traces = append(bucket.traces, bucketTrace)

		// Trigger compaction when we exceed the limit
		if len(bucket.traces) > 10 {
			bm.performBottomKCompaction(bucket)
		}
		bucket.mutex.Unlock()
	}

	// Verify we kept exactly the highest 10 values
	keptTraces := bucket.getAllTraces()
	require.Equal(t, 10, len(keptTraces))

	keptValues := make([]uint64, len(keptTraces))
	for i, trace := range keptTraces {
		keptValues[i] = trace.randomnessValue
	}

	// Sort to compare
	assert.ElementsMatch(t, expectedKeptValues, keptValues,
		"Should keep exactly the 10 traces with highest randomness values")
}
