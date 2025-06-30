// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
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
			tolerance:       0.15, // 15% tolerance
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
			tolerance:       0.20, // 20% tolerance (higher due to more variance)
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
			tolerance:       0.50, // 50% tolerance - expect significant approximation error
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
			tolerance:       0.75, // 75% tolerance - expect very high approximation error
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
			tolerance:       0.35, // 35% tolerance for medium approximation pressure
			setupTraces: func(traceCount int) ([]*internalsampling.TraceData, float64) {
				return createMixedWeightTraces(traceCount)
			},
			description: "Medium approximation pressure: 2k traces compressed to 50 slots",
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
				var heavyWeightCount, lightWeightCount, ultraHeavyWeightCount, ultraUltraHeavyWeightCount int
				var highWeightTraceFound, ultraHighWeightTraceFound, ultraUltraHighWeightTraceFound bool
				var highWeightContribution, ultraHighWeightContribution, ultraUltraHighWeightContribution float64

				for _, bt := range sampledTraces {
					// OTEP 235 adjusted count (input weight)
					otep235AdjustedCount := bt.inputWeight

					// Tail sampling adjustment
					tailAdjustment := bt.getTailSamplingAdjustment()

					// Debug: Show weight details for all traces in severe approximation pressure test, first 10 for others
					showDebug := trial == 0 && ((tt.name == "severe_approximation_pressure") || (heavyWeightCount+lightWeightCount+ultraHeavyWeightCount+ultraUltraHeavyWeightCount) < 10)
					if showDebug {
						// Get the TraceState from the actual trace data
						var traceStateStr string
						if bt.trace != nil {
							bt.trace.Lock()
							if bt.trace.ReceivedBatches.SpanCount() > 0 {
								resourceSpans := bt.trace.ReceivedBatches.ResourceSpans()
								if resourceSpans.Len() > 0 {
									scopeSpans := resourceSpans.At(0).ScopeSpans()
									if scopeSpans.Len() > 0 {
										spans := scopeSpans.At(0).Spans()
										if spans.Len() > 0 {
											traceStateStr = spans.At(0).TraceState().AsRaw()
										}
									}
								}
							}
							bt.trace.Unlock()
						}

						t.Logf("DEBUG: TraceID=%s, TraceState='%s', InputWeight=%.6f, VaropAdjustedWeight=%.6f, TailAdjustment=%.6f, FinalWeight=%.6f",
							bt.traceID, traceStateStr, otep235AdjustedCount, bt.varopAdjustedWeight, tailAdjustment, otep235AdjustedCount*tailAdjustment)
					}

					// Check for different weight categories
					isUltraUltraHighWeight := otep235AdjustedCount > 4000.0                              // th:ffff traces (65536.0)
					isUltraHighWeight := otep235AdjustedCount > 1000.0 && otep235AdjustedCount <= 4000.0 // th:fff traces (4096.0)
					isHighWeight := otep235AdjustedCount > 250.0 && otep235AdjustedCount <= 1000.0       // th:ff traces (256.0)
					isMediumWeight := otep235AdjustedCount > 15.0 && otep235AdjustedCount <= 250.0       // th:f traces (16.0)

					if isUltraUltraHighWeight {
						ultraUltraHighWeightTraceFound = true
						ultraUltraHighWeightContribution = otep235AdjustedCount * tailAdjustment
						ultraUltraHeavyWeightCount++
					} else if isUltraHighWeight {
						ultraHighWeightTraceFound = true
						ultraHighWeightContribution = otep235AdjustedCount * tailAdjustment
						ultraHeavyWeightCount++
					} else if isHighWeight {
						highWeightTraceFound = true
						highWeightContribution = otep235AdjustedCount * tailAdjustment
						heavyWeightCount++
					} else if isMediumWeight {
						heavyWeightCount++ // Group medium with heavy for simplicity
					} else {
						lightWeightCount++
					}

					// Combined adjustment
					combinedAdjustedCount := otep235AdjustedCount * tailAdjustment
					actualTotal += combinedAdjustedCount
				}

				// Calculate error with high precision
				absoluteError := math.Abs(actualTotal - expectedTotal)
				errorPct := absoluteError / expectedTotal * 100
				totalErrors = append(totalErrors, errorPct)

				// Summary of high-weight trace impact
				if ultraUltraHighWeightTraceFound {
					ultraUltraHighWeightPercentage := (ultraUltraHighWeightContribution / actualTotal) * 100
					t.Logf("ULTRA-ULTRA-HEAVY SUMMARY: Found=%v, Contribution=%.3f, Percentage of total=%.1f%%",
						ultraUltraHighWeightTraceFound, ultraUltraHighWeightContribution, ultraUltraHighWeightPercentage)
				}
				if ultraHighWeightTraceFound {
					ultraHighWeightPercentage := (ultraHighWeightContribution / actualTotal) * 100
					t.Logf("ULTRA-HEAVY SUMMARY: Found=%v, Contribution=%.3f, Percentage of total=%.1f%%",
						ultraHighWeightTraceFound, ultraHighWeightContribution, ultraHighWeightPercentage)
				}
				if highWeightTraceFound {
					highWeightPercentage := (highWeightContribution / actualTotal) * 100
					t.Logf("HIGH-WEIGHT SUMMARY: Found=%v, Contribution=%.3f, Percentage of total=%.1f%%",
						highWeightTraceFound, highWeightContribution, highWeightPercentage)
				}
				if !highWeightTraceFound && !ultraHighWeightTraceFound && !ultraUltraHighWeightTraceFound && tt.name != "uniform_weight_traces" {
					t.Logf("HIGH-WEIGHT SUMMARY: NO HEAVY TRACES FOUND IN SAMPLE!")
				}

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

				// For mixed weight test, verify that heavy weights are less likely
				if tt.name == "mixed_weight_traces" || tt.name == "high_pressure_extreme_weights" {
					t.Logf("Trial %d: Ultra-heavy: %d, Heavy: %d, Light: %d (of %d sampled)",
						trial+1, ultraHeavyWeightCount, heavyWeightCount, lightWeightCount, len(sampledTraces))
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
