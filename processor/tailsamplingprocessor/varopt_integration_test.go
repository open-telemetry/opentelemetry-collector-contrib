// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"fmt"
	"math"
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
	tests := []struct {
		name            string
		inputTraceCount int
		bucketCapacity  int
		trials          int
		tolerance       float64
		setupTraces     func(traceCount int) ([]*internalsampling.TraceData, float64)
		description     string
	}{
		{
			name:            "uniform_weight_traces",
			inputTraceCount: 1000,
			bucketCapacity:  100,
			trials:          5,
			tolerance:       0, // 0% tolerance
			setupTraces: func(traceCount int) ([]*internalsampling.TraceData, float64) {
				traces, total := createUniformWeightTraces(traceCount)
				return traces, total
			},
			description: "All traces have uniform weight (no OTEP 235 thresholds)",
		},
		{
			name:            "mixed_weight_traces",
			inputTraceCount: 1000,
			bucketCapacity:  100,
			trials:          5,
			tolerance:       0.0, // 0% tolerance
			setupTraces: func(traceCount int) ([]*internalsampling.TraceData, float64) {
				return createMixedWeightTraces(traceCount)
			},
			description: "Mixed traces: 100% sampling (th:0) and 50% sampling (th:8)",
		},
		{
			name:            "high_pressure_extreme_weights",
			inputTraceCount: 5000,
			bucketCapacity:  10, // Very small capacity to force heavy approximation
			trials:          10,
			tolerance:       0, // 0% tolerance
			setupTraces: func(traceCount int) ([]*internalsampling.TraceData, float64) {
				return createExtremeHighPressureTraces(traceCount)
			},
			description: "Extreme approximation pressure: many ultra-heavy traces competing for very few slots",
		},
		{
			name:            "severe_approximation_pressure",
			inputTraceCount: 10000,
			bucketCapacity:  5, // Extremely small capacity - 1 in 2000 sampling
			trials:          10,
			tolerance:       0.0, // 0% tolerance
			setupTraces: func(traceCount int) ([]*internalsampling.TraceData, float64) {
				return createExtremeHighPressureTraces(traceCount)
			},
			description: "Severe approximation pressure: 10k traces compressed to 5 slots",
		},
		{
			name:            "balanced_medium_pressure",
			inputTraceCount: 2000,
			bucketCapacity:  50, // 1 in 40 sampling ratio
			trials:          8,
			tolerance:       0.0, // 0% tolerance
			setupTraces: func(traceCount int) ([]*internalsampling.TraceData, float64) {
				return createMixedWeightTraces(traceCount)
			},
			description: "Medium approximation pressure: 2k traces compressed to 50 slots",
		},
		{
			name:            "gradient_probability_pressure",
			inputTraceCount: 10000,
			bucketCapacity:  10, // Very aggressive: 1000 traces compressed to 10 slots
			trials:          15,
			tolerance:       0.0000000000001, // very small tol
			setupTraces: func(traceCount int) ([]*internalsampling.TraceData, float64) {
				return createGradientProbabilityTraces(traceCount)
			},
			description: "Gradient probability pressure: 1000 traces with gradual probability differences (1/110 to 1/90) compressed to 10 slots",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)

			// Run multiple trials to test convergence
			var totalErrors []float64
			var lastBM *bucketManager

			for trial := 0; trial < tt.trials; trial++ {
				t.Logf("Trial %d/%d: %s", trial+1, tt.trials, tt.description)

				// Create bucket manager with appropriate capacity
				bm := newBucketManagerWithTimeSource(
					logger,
					1, // single bucket for simplicity
					time.Minute,
					uint64(tt.bucketCapacity),
					1.0,
					&fixedTimeSource{t: time.Now()},
				)
				lastBM = bm

				// Setup traces with different sampling weights
				traces, expectedTotal := tt.setupTraces(tt.inputTraceCount)

				// Add all traces to the bucket - ensure unique traceIDs to avoid duplicates
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

				// Calculate error with high precision
				absoluteError := math.Abs(actualTotal - expectedTotal)
				errorPct := absoluteError / expectedTotal * 100
				totalErrors = append(totalErrors, errorPct)

				t.Logf("Trial %d: Expected total: %.2f, Actual total: %.2f, Absolute error: %.10f, Error: %.8f%%, Sampled: %d/%d",
					trial+1, expectedTotal, actualTotal, absoluteError, errorPct, len(sampledTraces), tt.inputTraceCount)

				// For severe approximation pressure, show all output weights
				if tt.name == "severe_approximation_pressure" && trial == 0 {
					t.Logf("=== SEVERE APPROXIMATION PRESSURE: ALL OUTPUT WEIGHTS (Trial %d) ===", trial+1)
					for i, bt := range sampledTraces {
						otep235Weight := bt.inputWeight
						varopWeight := bt.varopAdjustedWeight
						tailAdjustment := bt.getTailSamplingAdjustment()
						finalWeight := otep235Weight * tailAdjustment

						t.Logf("  [%d] InputWeight=%.1f, VaropWeight=%.6f, TailAdj=%.6f, Final=%.6f",
							i+1, otep235Weight, varopWeight, tailAdjustment, finalWeight)
					}
					t.Logf("=== END OUTPUT WEIGHTS ===")
				}
			}

			// Calculate average error across trials
			var avgError float64
			for _, err := range totalErrors {
				avgError += err
			}
			avgError /= float64(len(totalErrors))

			t.Logf("Average error across %d trials: %.8f%% (tolerance: %.2f%%)",
				tt.trials, avgError*100, tt.tolerance*100)

			// Verify that average error is within tolerance
			assert.LessOrEqual(t, avgError, tt.tolerance,
				"Average adjusted count preservation error %.2f%% exceeds tolerance %.2f%%",
				avgError*100, tt.tolerance*100)

			// At least some traces should be sampled
			bucket := lastBM.buckets[0]
			sampledTraces := bucket.getTracesWithTailAdjustments()
			assert.Greater(t, len(sampledTraces), 0,
				"No traces were sampled")
		})
	}
}

// TestVaroptRandomnessWithBooleanAttributes tests that Varopt sampling preserves
// the distribution of boolean attributes when sampling uniform unweighted traces.
// This follows the pattern from varopt tests to verify unbiased sampling.
func TestVaroptRandomnessWithBooleanAttributes(t *testing.T) {
	tests := []struct {
		name            string
		populationSize  int
		sampleSize      int
		trials          int
		attributeName   string
		chiSquaredAlpha float64 // significance level for chi-squared test
		description     string
	}{
		{
			name:            "balanced_sampling_medium",
			populationSize:  1000,
			sampleSize:      100,
			trials:          50,
			attributeName:   "test_boolean",
			chiSquaredAlpha: 0.05, // 95% confidence
			description:     "1000 traces sampled to 100, testing boolean distribution",
		},
		{
			name:            "balanced_sampling_aggressive",
			populationSize:  5000,
			sampleSize:      50,
			trials:          30,
			attributeName:   "is_important",
			chiSquaredAlpha: 0.05,
			description:     "5000 traces sampled to 50, testing boolean distribution",
		},
		{
			name:            "balanced_sampling_extreme",
			populationSize:  10000,
			sampleSize:      20,
			trials:          25,
			attributeName:   "feature_flag",
			chiSquaredAlpha: 0.05,
			description:     "10000 traces sampled to 20, testing boolean distribution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)

			// Track results across trials for chi-squared test
			var trueCountsPerTrial []int
			var falseCountsPerTrial []int
			var totalDeviations []float64

			for trial := 0; trial < tt.trials; trial++ {
				t.Logf("Trial %d/%d: %s", trial+1, tt.trials, tt.description)

				// Create bucket manager
				bm := newBucketManagerWithTimeSource(
					logger,
					1, // single bucket
					time.Minute,
					uint64(tt.sampleSize),
					1.0,
					&fixedTimeSource{t: time.Now()},
				)

				// Create uniform unweighted traces with boolean attributes
				traces, trueCount, falseCount := createUniformTracesWithBooleanAttribute(
					tt.populationSize, tt.attributeName)

				t.Logf("Population: %d true, %d false (total: %d)",
					trueCount, falseCount, tt.populationSize)

				// Add all traces to the bucket
				baseTime := time.Now()
				for i, trace := range traces {
					traceID := generateTraceID(i)
					bm.addTrace(traceID, trace, baseTime)
				}

				// Get sampled traces
				bucket := bm.buckets[0]
				sampledTraces := bucket.getTracesWithTailAdjustments()

				// Count boolean attribute distribution in sample
				sampledTrueCount := 0
				sampledFalseCount := 0

				for _, bt := range sampledTraces {
					hasTrue := extractBooleanAttribute(bt, tt.attributeName)
					if hasTrue {
						sampledTrueCount++
					} else {
						sampledFalseCount++
					}
				}

				sampledTotal := sampledTrueCount + sampledFalseCount
				assert.Equal(t, len(sampledTraces), sampledTotal,
					"Sample count mismatch")

				// Calculate percentages
				expectedTrueRatio := float64(trueCount) / float64(tt.populationSize)
				actualTrueRatio := float64(sampledTrueCount) / float64(sampledTotal)

				deviation := math.Abs(actualTrueRatio - expectedTrueRatio)
				totalDeviations = append(totalDeviations, deviation)

				trueCountsPerTrial = append(trueCountsPerTrial, sampledTrueCount)
				falseCountsPerTrial = append(falseCountsPerTrial, sampledFalseCount)

				t.Logf("Trial %d: Sample=%d (true=%d, false=%d), Expected true ratio=%.3f, Actual=%.3f, Deviation=%.4f",
					trial+1, sampledTotal, sampledTrueCount, sampledFalseCount,
					expectedTrueRatio, actualTrueRatio, deviation)
			}

			// Calculate average deviation
			var avgDeviation float64
			for _, dev := range totalDeviations {
				avgDeviation += dev
			}
			avgDeviation /= float64(len(totalDeviations))

			// Perform chi-squared test to validate randomness
			chiSquaredStatistic, pValue := calculateChiSquaredTest(trueCountsPerTrial, falseCountsPerTrial)

			t.Logf("=== RANDOMNESS TEST RESULTS ===")
			t.Logf("Average deviation from expected ratio: %.4f", avgDeviation)
			t.Logf("Chi-squared statistic: %.4f", chiSquaredStatistic)
			t.Logf("P-value: %.6f", pValue)
			t.Logf("Alpha (significance level): %.2f", tt.chiSquaredAlpha)

			// Verify randomness: p-value should be > alpha (fail to reject null hypothesis)
			assert.Greater(t, pValue, tt.chiSquaredAlpha,
				"Chi-squared test suggests non-random sampling (p=%.6f < α=%.2f). "+
					"This indicates the sampler may be biased.", pValue, tt.chiSquaredAlpha)

			// Verify reasonable average deviation (should be small for large sample sizes)
			maxExpectedDeviation := 0.2 // 20% maximum average deviation
			if tt.sampleSize >= 50 {
				maxExpectedDeviation = 0.15 // 15% for larger samples
			}
			if tt.sampleSize >= 100 {
				maxExpectedDeviation = 0.10 // 10% for very large samples
			}

			assert.LessOrEqual(t, avgDeviation, maxExpectedDeviation,
				"Average deviation %.4f exceeds expected maximum %.4f for sample size %d",
				avgDeviation, maxExpectedDeviation, tt.sampleSize)

			t.Logf("✓ Randomness test passed: sampling appears unbiased")
		})
	}
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

// createHighPressureTraces creates traces designed to force Varopt approximation
func createHighPressureTraces(count int) ([]*internalsampling.TraceData, float64) {
	traces := make([]*internalsampling.TraceData, count)
	var expectedTotal float64

	// Create extreme weight disparity to force approximation
	// 50% very heavy traces (th:ff = 256x weight)
	// 30% heavy traces (th:f = 16x weight)
	// 20% light traces (th:0 = 1x weight)

	heavyCount := count / 2                        // 50% ultra-heavy
	mediumCount := (count * 3) / 10                // 30% heavy
	lightCount := count - heavyCount - mediumCount // remaining 20% light

	traceIndex := 0

	// Ultra-heavy traces (th:ff) - creates massive approximation pressure
	for i := 0; i < heavyCount; i++ {
		traceState := "ot=th:ff" // th:ff = 256x weight
		threshold, _ := sampling.TValueToThreshold("ff")
		weight := threshold.AdjustedCount() // Should be 256.0

		traces[traceIndex] = &internalsampling.TraceData{
			ReceivedBatches: createTraceBatch(generateTraceID(traceIndex), traceState),
		}
		expectedTotal += weight
		traceIndex++
	}

	// Medium-heavy traces (th:f)
	for i := 0; i < mediumCount; i++ {
		traceState := "ot=th:f"
		threshold, _ := sampling.TValueToThreshold("f")
		weight := threshold.AdjustedCount() // Should be 16.0

		traces[traceIndex] = &internalsampling.TraceData{
			ReceivedBatches: createTraceBatch(generateTraceID(traceIndex), traceState),
		}
		expectedTotal += weight
		traceIndex++
	}

	// Light traces (th:0)
	for i := 0; i < lightCount; i++ {
		traceState := "ot=th:0"
		threshold, _ := sampling.TValueToThreshold("0")
		weight := threshold.AdjustedCount() // Should be 1.0

		traces[traceIndex] = &internalsampling.TraceData{
			ReceivedBatches: createTraceBatch(generateTraceID(traceIndex), traceState),
		}
		expectedTotal += weight
		traceIndex++
	}

	return traces, expectedTotal
}

// createExtremeHighPressureTraces creates traces with extreme weight disparity to force maximum approximation
func createExtremeHighPressureTraces(count int) ([]*internalsampling.TraceData, float64) {
	traces := make([]*internalsampling.TraceData, count)
	var expectedTotal float64

	// Create massive weight disparity to force approximation
	// 60% ultra-ultra-heavy traces (th:ffff = 65536x weight)
	// 25% ultra-heavy traces (th:fff = 4096x weight)
	// 10% heavy traces (th:ff = 256x weight)
	// 5% light traces (th:0 = 1x weight)

	ultraUltraHeavyCount := (count * 6) / 10                                  // 60% ultra-ultra-heavy
	ultraHeavyCount := (count * 25) / 100                                     // 25% ultra-heavy
	heavyCount := count / 10                                                  // 10% heavy
	lightCount := count - ultraUltraHeavyCount - ultraHeavyCount - heavyCount // remaining 5% light

	traceIndex := 0

	// Ultra-ultra-heavy traces (th:ffff) - creates extreme approximation pressure
	for i := 0; i < ultraUltraHeavyCount; i++ {
		traceState := "ot=th:ffff" // th:ffff = 65536x weight
		threshold, _ := sampling.TValueToThreshold("ffff")
		weight := threshold.AdjustedCount() // Should be 65536.0

		traces[traceIndex] = &internalsampling.TraceData{
			ReceivedBatches: createTraceBatch(generateTraceID(traceIndex), traceState),
		}
		expectedTotal += weight
		traceIndex++
	}

	// Ultra-heavy traces (th:fff)
	for i := 0; i < ultraHeavyCount; i++ {
		traceState := "ot=th:fff" // th:fff = 4096x weight
		threshold, _ := sampling.TValueToThreshold("fff")
		weight := threshold.AdjustedCount() // Should be 4096.0

		traces[traceIndex] = &internalsampling.TraceData{
			ReceivedBatches: createTraceBatch(generateTraceID(traceIndex), traceState),
		}
		expectedTotal += weight
		traceIndex++
	}

	// Heavy traces (th:ff)
	for i := 0; i < heavyCount; i++ {
		traceState := "ot=th:ff" // th:ff = 256x weight
		threshold, _ := sampling.TValueToThreshold("ff")
		weight := threshold.AdjustedCount() // Should be 256.0

		traces[traceIndex] = &internalsampling.TraceData{
			ReceivedBatches: createTraceBatch(generateTraceID(traceIndex), traceState),
		}
		expectedTotal += weight
		traceIndex++
	}

	// Light traces (th:0)
	for i := 0; i < lightCount; i++ {
		traceState := "ot=th:0"
		threshold, _ := sampling.TValueToThreshold("0")
		weight := threshold.AdjustedCount() // Should be 1.0

		traces[traceIndex] = &internalsampling.TraceData{
			ReceivedBatches: createTraceBatch(generateTraceID(traceIndex), traceState),
		}
		expectedTotal += weight
		traceIndex++
	}

	return traces, expectedTotal
}

// createGradientProbabilityTraces creates traces with gradually changing sampling thresholds
// from 1/110 to 1/90 to test approximation with subtle weight differences
func createGradientProbabilityTraces(count int) ([]*internalsampling.TraceData, float64) {
	traces := make([]*internalsampling.TraceData, count)
	var expectedTotal float64

	for i := 0; i < count; i++ {
		probability := max(0.1, rand.Float64())

		// Convert probability to threshold using pkg/sampling
		threshold, err := sampling.ProbabilityToThreshold(probability)
		if i == 0 {
			threshold, err = sampling.ProbabilityToThreshold(1e-8)
		} else if i == 1 {
			threshold, err = sampling.ProbabilityToThreshold(1e-6)
		} else if i == 2 {
			threshold, err = sampling.ProbabilityToThreshold(1e-4)
		}
		if err != nil {
			panic(fmt.Sprint("no fallback", probability, err))
		}

		// Get the adjusted count (weight) for this trace
		weight := threshold.AdjustedCount()

		// Create tracestate with the threshold value
		traceState := "ot=th:" + threshold.TValue()

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

// createUniformTracesWithBooleanAttribute creates traces with uniform weight (no OTEP 235)
// and attaches boolean attributes with equal true/false distribution
func createUniformTracesWithBooleanAttribute(count int, attributeName string) ([]*internalsampling.TraceData, int, int) {
	traces := make([]*internalsampling.TraceData, count)

	trueCount := count / 2
	falseCount := count - trueCount // Handle odd counts

	// Create traces with alternating boolean values to ensure balance
	for i := 0; i < count; i++ {
		boolValue := i < trueCount // First half get true, second half get false

		traces[i] = &internalsampling.TraceData{
			ReceivedBatches: createTraceBatchWithBooleanAttribute(
				generateTraceID(i), "", attributeName, boolValue),
		}
	}

	// Shuffle to remove any ordering bias
	rng := rand.New(rand.NewSource(int64(count)))
	rng.Shuffle(len(traces), func(i, j int) {
		traces[i], traces[j] = traces[j], traces[i]
	})

	return traces, trueCount, falseCount
}

// createTraceBatchWithBooleanAttribute creates a ptrace.Traces with a boolean attribute
func createTraceBatchWithBooleanAttribute(traceID pcommon.TraceID, traceState, attributeName string, boolValue bool) ptrace.Traces {
	traces := ptrace.NewTraces()
	resourceSpan := traces.ResourceSpans().AppendEmpty()
	scopeSpan := resourceSpan.ScopeSpans().AppendEmpty()
	span := scopeSpan.Spans().AppendEmpty()

	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetName("test-span")

	// Add boolean attribute to span
	span.Attributes().PutBool(attributeName, boolValue)

	if traceState != "" {
		span.TraceState().FromRaw(traceState)
	}

	return traces
}

// extractBooleanAttribute extracts a boolean attribute from a sampled trace
func extractBooleanAttribute(bt *bucketTrace, attributeName string) bool {
	// Navigate through the trace structure to find the attribute
	for i := 0; i < bt.trace.ReceivedBatches.ResourceSpans().Len(); i++ {
		resourceSpan := bt.trace.ReceivedBatches.ResourceSpans().At(i)

		for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
			scopeSpan := resourceSpan.ScopeSpans().At(j)

			for k := 0; k < scopeSpan.Spans().Len(); k++ {
				span := scopeSpan.Spans().At(k)

				if value, exists := span.Attributes().Get(attributeName); exists {
					return value.Bool()
				}
			}
		}
	}

	// Default to false if attribute not found
	return false
}

// calculateChiSquaredTest performs a chi-squared test for randomness
// H0: The sampling is random (true/false distribution matches expected)
// H1: The sampling is not random
func calculateChiSquaredTest(trueCountsPerTrial, falseCountsPerTrial []int) (float64, float64) {
	numTrials := len(trueCountsPerTrial)

	// Calculate overall statistics
	var totalTrue, totalFalse int
	for i := 0; i < numTrials; i++ {
		totalTrue += trueCountsPerTrial[i]
		totalFalse += falseCountsPerTrial[i]
	}

	expectedTruePerTrial := float64(totalTrue) / float64(numTrials)
	expectedFalsePerTrial := float64(totalFalse) / float64(numTrials)

	// Calculate chi-squared statistic
	var chiSquared float64
	for i := 0; i < numTrials; i++ {
		trueObs := float64(trueCountsPerTrial[i])
		falseObs := float64(falseCountsPerTrial[i])

		chiSquared += math.Pow(trueObs-expectedTruePerTrial, 2) / expectedTruePerTrial
		chiSquared += math.Pow(falseObs-expectedFalsePerTrial, 2) / expectedFalsePerTrial
	}

	// Degrees of freedom = numTrials - 1 (for each category)
	degreesOfFreedom := float64(numTrials - 1)

	// Approximate p-value using incomplete gamma function
	// For simplicity, we'll use a basic approximation
	// In a real implementation, you'd use a proper statistical library
	pValue := approximateChiSquaredPValue(chiSquared, degreesOfFreedom)

	return chiSquared, pValue
}

// approximateChiSquaredPValue provides a rough approximation of chi-squared p-value
// This is a simplified implementation for testing purposes
func approximateChiSquaredPValue(chiSquared, degreesOfFreedom float64) float64 {
	// For df > 30, chi-squared approaches normal distribution
	// Use Wilson-Hilferty transformation for better approximation

	if degreesOfFreedom > 30 {
		// Normal approximation
		mean := degreesOfFreedom
		variance := 2 * degreesOfFreedom
		standardized := (chiSquared - mean) / math.Sqrt(variance)

		// Approximate p-value using complementary error function
		return 0.5 * math.Erfc(standardized/math.Sqrt(2))
	}

	// For smaller df, use a lookup table approach
	// These are rough approximations for common significance levels
	critical05 := degreesOfFreedom + 1.96*math.Sqrt(2*degreesOfFreedom) // α = 0.05
	critical01 := degreesOfFreedom + 2.58*math.Sqrt(2*degreesOfFreedom) // α = 0.01

	if chiSquared < critical05 {
		return 0.5 // p > 0.05
	} else if chiSquared < critical01 {
		return 0.02 // 0.01 < p < 0.05
	} else {
		return 0.005 // p < 0.01
	}
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
