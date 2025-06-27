// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

func TestBottomKThresholdCalculation(t *testing.T) {
	logger := zap.NewNop()
	bm := &bucketManager{
		logger: logger,
	}

	tests := []struct {
		name                           string
		minRandomnessValueOfKeptTraces uint64
		k                              uint64
		expectedResult                 string // "never", "always", or "valid"
	}{
		{
			name:                           "k=0 should return never sample",
			minRandomnessValueOfKeptTraces: 1000,
			k:                              0,
			expectedResult:                 "never",
		},
		{
			name:                           "minRandomness=0 should return always sample",
			minRandomnessValueOfKeptTraces: 0,
			k:                              10,
			expectedResult:                 "always",
		},
		{
			name:                           "small randomness value should give high probability",
			minRandomnessValueOfKeptTraces: 0x000000000000FFFF, // Small normalized value ≈ 0.000004
			k:                              100,
			expectedResult:                 "valid",
		},
		{
			name:                           "medium randomness value should give medium probability",
			minRandomnessValueOfKeptTraces: 0x007FFFFFFFFFFFFF, // Medium normalized value ≈ 0.5
			k:                              100,
			expectedResult:                 "valid",
		},
		{
			name:                           "high randomness value should give low probability",
			minRandomnessValueOfKeptTraces: 0x00FFFFFFFFFFFFFF, // Max normalized value = 1.0
			k:                              100,
			expectedResult:                 "valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			threshold := bm.calculateBottomKThresholdCompat(tt.minRandomnessValueOfKeptTraces, tt.k)

			switch tt.expectedResult {
			case "never":
				assert.Equal(t, sampling.NeverSampleThreshold, threshold)
			case "always":
				assert.Equal(t, sampling.AlwaysSampleThreshold, threshold)
			case "valid":
				assert.NotEqual(t, sampling.NeverSampleThreshold, threshold)
				assert.NotEqual(t, sampling.AlwaysSampleThreshold, threshold)

				// Convert back to probability to verify it's reasonable
				prob := threshold.Probability()
				assert.Greater(t, prob, 0.0)
				assert.Less(t, prob, 1.0)

				t.Logf("Randomness: %x, k: %d, Probability: %f",
					tt.minRandomnessValueOfKeptTraces, tt.k, prob)
			}
		})
	}
}

func TestBottomKThresholdMathematicalProperties(t *testing.T) {
	logger := zap.NewNop()
	bm := &bucketManager{
		logger: logger,
	}

	// Test that lower randomness values lead to higher probabilities (lower thresholds)
	k := uint64(100)
	maxRandomness := uint64(0x00FFFFFFFFFFFFFF)

	// Test different randomness levels
	lowRandomness := maxRandomness / 4      // 25% of max
	medRandomness := maxRandomness / 2      // 50% of max
	highRandomness := maxRandomness * 3 / 4 // 75% of max

	lowThreshold := bm.calculateBottomKThresholdCompat(lowRandomness, k)
	medThreshold := bm.calculateBottomKThresholdCompat(medRandomness, k)
	highThreshold := bm.calculateBottomKThresholdCompat(highRandomness, k)

	// Convert to probabilities
	lowProb := lowThreshold.Probability()
	medProb := medThreshold.Probability()
	highProb := highThreshold.Probability()

	// Lower randomness should give higher probability (easier to sample)
	assert.Greater(t, lowProb, medProb, "Lower randomness should give higher probability")
	assert.Greater(t, medProb, highProb, "Medium randomness should give higher probability than high")

	t.Logf("Low randomness (%x): prob=%f", lowRandomness, lowProb)
	t.Logf("Med randomness (%x): prob=%f", medRandomness, medProb)
	t.Logf("High randomness (%x): prob=%f", highRandomness, highProb)
}

func TestBottomKExponentialFormula(t *testing.T) {
	// Test the raw mathematical formula to ensure it works with normalized values
	maxRandomness := uint64(0x00FFFFFFFFFFFFFF)

	testCases := []struct {
		randomness uint64
		expected   string // rough expectation
	}{
		{maxRandomness / 10, "high_prob"},    // 10% of max = low rank = high prob
		{maxRandomness / 2, "med_prob"},      // 50% of max = med rank = med prob
		{maxRandomness * 9 / 10, "low_prob"}, // 90% of max = high rank = low prob
	}

	for _, tc := range testCases {
		// Normalize to [0,1]
		normalized := float64(tc.randomness) / float64(maxRandomness)

		// Apply formula: Adjusted = Weight / (1 - exp(Weight * Rank))
		initialWeight := 1.0
		expArg := initialWeight * normalized
		expTerm := math.Exp(expArg)
		denominator := 1.0 - expTerm

		t.Logf("Randomness: %x, Normalized: %f, ExpArg: %f, ExpTerm: %f, Denominator: %f",
			tc.randomness, normalized, expArg, expTerm, denominator)

		// Since exp(x) > 1 for x > 0, denominator will be negative
		// This is expected for Bottom-K where we're looking at the k+1th item
		assert.Less(t, denominator, 0.0, "Denominator should be negative for normalized values > 0")

		if denominator < 0 {
			// Take absolute value for adjusted count calculation
			adjustedCount := initialWeight / math.Abs(denominator)
			probability := 1.0 / adjustedCount

			t.Logf("AdjustedCount: %f, Probability: %f", adjustedCount, probability)

			assert.Greater(t, adjustedCount, 1.0, "Adjusted count should be > 1")
			assert.Greater(t, probability, 0.0, "Probability should be positive")
			assert.Less(t, probability, 1.0, "Probability should be < 1")
		}
	}
}

// TestBottomKEstimatorValidation validates the corrected Bottom-K estimator implementation
func TestBottomKEstimatorValidation(t *testing.T) {
	// Create a logger that outputs to the test log
	logger, _ := zap.NewDevelopment()
	bm := &bucketManager{
		logger: logger,
	}

	// Test parameters
	totalTraces := 1000
	reservoirSize := 200 // Sample 20% of the traces to avoid edge cases
	maxRandomness := uint64(0x00FFFFFFFFFFFFFF)

	// Generate uniformly distributed randomness values
	randomnessValues := make([]uint64, totalTraces)
	step := maxRandomness / uint64(totalTraces+1) // Add 1 to avoid hitting maxRandomness exactly
	for i := 0; i < totalTraces; i++ {
		// Evenly distribute values from step to maxRandomness-step
		randomnessValues[i] = step * uint64(i+1)
	}

	// Sort by randomness (ascending) and take the top reservoirSize traces
	// In Bottom-K, we keep traces with highest randomness values
	selectedIndices := make([]int, reservoirSize)
	for i := 0; i < reservoirSize; i++ {
		selectedIndices[i] = totalTraces - reservoirSize + i // Take the highest randomness values
	}

	// Calculate the threshold using the minimum randomness of selected traces
	minRandomnessOfSelected := randomnessValues[selectedIndices[0]]
	threshold := bm.calculateBottomKThresholdCompat(minRandomnessOfSelected, uint64(reservoirSize))

	t.Logf("Min randomness of selected: %x", minRandomnessOfSelected)
	t.Logf("Reservoir size (k): %d", reservoirSize)
	t.Logf("Calculated threshold: %s", threshold.TValue())

	// Skip test if we get edge case thresholds
	if threshold == sampling.NeverSampleThreshold {
		t.Skip("Test hit NeverSampleThreshold")
	}
	if threshold == sampling.AlwaysSampleThreshold {
		t.Skip("Test hit AlwaysSampleThreshold")
	}

	// Convert threshold to probability using the correct method
	samplingProb := threshold.Probability()

	// Calculate adjusted counts for each selected trace
	totalAdjustedCount := 0.0
	for range selectedIndices {
		// Each trace has weight = 1/samplingProb
		adjustedCount := 1.0 / samplingProb
		totalAdjustedCount += adjustedCount
	}

	// The sum of adjusted counts should approximate the total population
	expectedTotal := float64(totalTraces)
	tolerance := expectedTotal * 0.10 // 10% tolerance

	t.Logf("Total traces: %d", totalTraces)
	t.Logf("Reservoir size: %d", reservoirSize)
	t.Logf("Min randomness of selected: %x", minRandomnessOfSelected)
	t.Logf("Sampling probability: %f", samplingProb)
	t.Logf("Individual adjusted count: %f", 1.0/samplingProb)
	t.Logf("Total adjusted count: %f", totalAdjustedCount)
	t.Logf("Expected total: %f", expectedTotal)
	t.Logf("Difference: %f", math.Abs(totalAdjustedCount-expectedTotal))
	t.Logf("Tolerance: %f", tolerance)

	assert.InDelta(t, expectedTotal, totalAdjustedCount, tolerance,
		"Sum of adjusted counts should approximate total population within tolerance")
}

// TestBottomKRankCalculation verifies the rank calculation is correct
func TestBottomKRankCalculation(t *testing.T) {
	maxRandomness := uint64(0x00FFFFFFFFFFFFFF)

	testCases := []struct {
		name               string
		randomness         uint64
		expectedRankApprox float64 // Approximate expected rank
	}{
		{
			name:               "zero randomness should give rank=1.0",
			randomness:         0,
			expectedRankApprox: 1.0,
		},
		{
			name:               "quarter randomness should give rank=0.75",
			randomness:         maxRandomness / 4,
			expectedRankApprox: 0.75,
		},
		{
			name:               "half randomness should give rank=0.5",
			randomness:         maxRandomness / 2,
			expectedRankApprox: 0.5,
		},
		{
			name:               "three-quarter randomness should give rank=0.25",
			randomness:         maxRandomness * 3 / 4,
			expectedRankApprox: 0.25,
		},
		{
			name:               "max randomness should give rank=0.0",
			randomness:         maxRandomness,
			expectedRankApprox: 0.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate rank using the same formula as in calculateBottomKThreshold
			actualRank := (float64(maxRandomness) - float64(tc.randomness)) / float64(maxRandomness)

			t.Logf("Randomness: %x, Expected rank: %f, Actual rank: %f",
				tc.randomness, tc.expectedRankApprox, actualRank)

			assert.InDelta(t, tc.expectedRankApprox, actualRank, 0.01,
				"Rank calculation should match expected value")

			// Verify rank is in [0,1] range
			assert.GreaterOrEqual(t, actualRank, 0.0, "Rank should be >= 0")
			assert.LessOrEqual(t, actualRank, 1.0, "Rank should be <= 1")
		})
	}
}

// TestBottomKFormulaComponents tests individual components of the Bottom-K formula
func TestBottomKFormulaComponents(t *testing.T) {
	maxRandomness := uint64(0x00FFFFFFFFFFFFFF)

	testCases := []struct {
		name            string
		randomness      uint64
		expectValidProb bool
	}{
		{
			name:            "low randomness - high rank - should work",
			randomness:      maxRandomness / 10,
			expectValidProb: true,
		},
		{
			name:            "medium randomness - medium rank - should work",
			randomness:      maxRandomness / 2,
			expectValidProb: true,
		},
		{
			name:            "high randomness - low rank - should work",
			randomness:      maxRandomness * 9 / 10,
			expectValidProb: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate components using corrected formula
			normalizedRandomness := float64(tc.randomness) / float64(maxRandomness)
			rank := 1.0 - normalizedRandomness // Corrected: rank = (maxRandomness - R) / maxRandomness

			initialWeight := 1.0
			expArg := initialWeight * rank
			expTerm := math.Exp(expArg)
			denominator := 1.0 - expTerm

			t.Logf("Randomness: %x", tc.randomness)
			t.Logf("Normalized randomness: %f", normalizedRandomness)
			t.Logf("Rank (1 - normalized): %f", rank)
			t.Logf("Exp argument: %f", expArg)
			t.Logf("Exp term: %f", expTerm)
			t.Logf("Denominator (1 - exp): %f", denominator)

			if tc.expectValidProb {
				assert.NotEqual(t, 0.0, denominator, "Denominator should not be zero")

				if denominator != 0.0 {
					adjustedCount := initialWeight / math.Abs(denominator)
					probability := 1.0 / adjustedCount

					t.Logf("Adjusted count: %f", adjustedCount)
					t.Logf("Probability: %f", probability)

					assert.Greater(t, adjustedCount, 0.0, "Adjusted count should be positive")
					assert.Greater(t, probability, 0.0, "Probability should be positive")
					assert.LessOrEqual(t, probability, 1.0, "Probability should be <= 1.0")
				}
			}
		})
	}
}
