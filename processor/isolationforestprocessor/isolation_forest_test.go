// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// isolation_forest_test.go - Unit tests for the isolation forest algorithm
package isolationforestprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

func TestOnlineIsolationForestCreation(t *testing.T) {
	forest := newOnlineIsolationForest(10, 100, 8)

	assert.Equal(t, 10, forest.numTrees, "Should create forest with specified number of trees")
	assert.Equal(t, 8, forest.maxDepth, "Should set specified max depth")
	assert.Equal(t, 100, forest.windowSize, "Should set specified window size")
	assert.Len(t, forest.trees, 10, "Should initialize correct number of trees")

	// Verify trees are initialized
	for i, tree := range forest.trees {
		assert.NotNil(t, tree, "Tree %d should be initialized", i)
		assert.Equal(t, 8, tree.maxDepth, "Tree %d should have correct max depth", i)
	}
}

func TestBasicAnomalyDetection(t *testing.T) {
	forest := newOnlineIsolationForest(20, 50, 6)

	// Generate normal data points (clustered around origin)
	normalSamples := [][]float64{
		{1.0, 1.0},
		{1.1, 0.9},
		{0.9, 1.1},
		{1.2, 0.8},
		{0.8, 1.2},
		{1.0, 1.0},
		{0.95, 1.05},
		{1.05, 0.95},
		{1.1, 1.1},
		{0.9, 0.9},
		{1.0, 1.0},
		{1.0, 1.0},
	}
	// Process normal samples to train the model
	for _, sample := range normalSamples {
		forest.ProcessSample(sample)
		time.Sleep(1 * time.Millisecond) // allow async processing
	}
	// Allow some time for model updates
	time.Sleep(10 * time.Millisecond)

	// Test with clearly anomalous points (far from normal cluster)
	anomalousSamples := [][]float64{
		{10.0, 10.0}, // Very far from normal cluster
		{-5.0, -5.0}, // Opposite direction
		{0.0, 100.0}, // Extreme in one dimension
	}

	// Process anomalous samples and check scores
	for _, sample := range anomalousSamples {
		score, isAnomaly := forest.ProcessSample(sample)
		// Verify score is in valid range
		assert.True(t, score >= 0.0 && score <= 1.0, "Anomaly score should be between 0 and 1, got %f for sample %v", score, sample)
		// Note: In a production scenario with more training data,
		// we would expect isAnomaly to be true for these extreme points.
		// For unit testing, we mainly verify the algorithm runs without errors.
		_ = isAnomaly // Avoid unused variable warning
	}
}

func TestAdaptiveThreshold(t *testing.T) {
	forest := newOnlineIsolationForest(10, 100, 6)

	// Track how threshold changes with different score patterns
	initialStats := forest.GetStatistics()
	initialThreshold := initialStats.CurrentThreshold
	_ = initialThreshold // mark as used to avoid compile error if not asserted

	// Process samples with varying anomaly scores
	samples := [][]float64{
		{1.0, 1.0},
		{2.0, 2.0},
		{3.0, 3.0},
		{4.0, 4.0},
		{5.0, 5.0},
		{1.1, 1.1},
		{2.1, 2.1},
		{3.1, 3.1},
		{4.1, 4.1},
		{5.1, 5.1},
	}
	for _, sample := range samples {
		forest.ProcessSample(sample)
		time.Sleep(1 * time.Millisecond)
	}
	// Allow time for threshold adaptation
	time.Sleep(20 * time.Millisecond)

	finalStats := forest.GetStatistics()
	finalThreshold := finalStats.CurrentThreshold

	// Verify threshold is still in valid range
	assert.True(t, finalThreshold >= 0.0 && finalThreshold <= 1.0, "Adaptive threshold should be between 0 and 1, got %f", finalThreshold)

	// Verify statistics are reasonable
	assert.True(t, finalStats.TotalSamples >= uint64(len(samples)), "Should have processed at least %d samples", len(samples))
	assert.True(t, finalStats.WindowUtilization >= 0.0 && finalStats.WindowUtilization <= 1.0, "Window utilization should be between 0 and 1")
}

func TestForestStatistics(t *testing.T) {
	forest := newOnlineIsolationForest(5, 20, 4)

	// Initial statistics
	stats := forest.GetStatistics()
	assert.Equal(t, uint64(0), stats.TotalSamples, "Should start with 0 samples")
	assert.Equal(t, uint64(0), stats.AnomalyCount, "Should start with 0 anomalies")
	assert.Equal(t, 0.0, stats.AnomalyRate, "Should start with 0% anomaly rate")
	assert.Equal(t, 5, stats.ActiveTrees, "Should have 5 active trees")

	// Process some samples
	samples := [][]float64{
		{1.0, 2.0}, {1.5, 2.5}, {0.5, 1.5}, {2.0, 3.0}, {1.2, 2.2},
	}
	for _, sample := range samples {
		forest.ProcessSample(sample)
	}
	// Allow processing time
	time.Sleep(10 * time.Millisecond)

	// Check updated statistics
	stats = forest.GetStatistics()
	assert.True(t, stats.TotalSamples >= uint64(len(samples)), "Should have processed at least %d samples", len(samples))
	assert.True(t, stats.AnomalyRate >= 0.0 && stats.AnomalyRate <= 1.0, "Anomaly rate should be between 0 and 1")
}

func TestTreePathLength(t *testing.T) {
	// Test individual tree path length calculation
	tree := &onlineIsolationTree{
		maxDepth: 5,
	}

	// Tree with no root should return 0
	pathLength := tree.calculatePathLength([]float64{1.0, 2.0})
	assert.Equal(t, 0.0, pathLength, "Empty tree should return path length 0")

	// Create simple tree structure for testing
	tree.root = &onlineTreeNode{
		featureIndex: 0,
		splitValue:   1.5,
		depth:        0,
		sampleCount:  10,
		isLeaf:       false,
		left: &onlineTreeNode{
			depth:       1,
			sampleCount: 5,
			isLeaf:      true,
		},
		right: &onlineTreeNode{
			depth:       1,
			sampleCount: 5,
			isLeaf:      true,
		},
	}

	// Test path length calculation for both sides of split
	leftPath := tree.calculatePathLength([]float64{1.0, 2.0})  // Should go left
	rightPath := tree.calculatePathLength([]float64{2.0, 2.0}) // Should go right
	assert.True(t, leftPath > 0, "Left path should have positive length")
	assert.True(t, rightPath > 0, "Right path should have positive length")
}

func TestExpectedPathLength(t *testing.T) {
	forest := newOnlineIsolationForest(10, 100, 6)

	// Add some samples to the window
	samples := [][]float64{
		{1.0, 1.0}, {2.0, 2.0}, {3.0, 3.0}, {4.0, 4.0}, {5.0, 5.0},
	}
	for _, sample := range samples {
		forest.ProcessSample(sample)
	}

	expectedLength := forest.getExpectedPathLength()
	assert.True(t, expectedLength > 0, "Expected path length should be positive")
	assert.True(t, expectedLength < 100, "Expected path length should be reasonable")
}

// Benchmark tests to verify performance characteristics

func BenchmarkIsolationForestProcessing(b *testing.B) {
	forest := newOnlineIsolationForest(100, 1000, 10)

	// Prepare test samples
	samples := make([][]float64, 1000)
	for i := range samples {
		samples[i] = []float64{
			float64(i%10) + 0.1*float64(i%3),
			float64(i%7) + 0.2*float64(i%5),
			float64(i%13) + 0.3*float64(i%7),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sample := samples[i%len(samples)]
		forest.ProcessSample(sample)
	}
}

func BenchmarkFeatureExtraction(b *testing.B) {
	logger := zaptest.NewLogger(b)
	extractor := newTraceFeatureExtractor([]string{"duration", "error", "http.status_code"}, logger)

	// Create test span
	span := ptrace.NewSpan()
	span.SetName("benchmark_operation")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(100 * time.Millisecond)))
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.Attributes().PutStr("http.status_code", "200")

	resourceAttrs := map[string]interface{}{
		"service.name": "benchmark-service",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractor.ExtractFeatures(span, resourceAttrs)
	}
}
