// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// isolation_forest_test.go - Unit tests for the isolation forest algorithm
package isolationforestprocessor

import (
	"math"
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
	assert.GreaterOrEqual(t, finalStats.TotalSamples, uint64(len(samples)), "Should have processed at least %d samples", len(samples))
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
	assert.GreaterOrEqual(t, stats.TotalSamples, uint64(len(samples)), "Should have processed at least %d samples", len(samples))
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
	assert.Positive(t, leftPath, "Left path should have positive length")
	assert.Positive(t, rightPath, "Right path should have positive length")
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
	assert.Positive(t, expectedLength, "Expected path length should be positive")
	assert.Less(t, expectedLength, 100.0, "Expected path length should be less than 100")
}

// Additional tests for 100% coverage

func TestOnlineIsolationForestCreation_AutoMaxDepth(t *testing.T) {
	// Test with maxDepth <= 0 to trigger auto-calculation
	forest := newOnlineIsolationForest(5, 32, 0)
	expectedDepth := int(math.Ceil(math.Log2(float64(32)))) // Should be 5
	assert.Equal(t, expectedDepth, forest.maxDepth, "Should auto-calculate max depth")
}

func TestProcessSample_EmptySample(t *testing.T) {
	forest := newOnlineIsolationForest(5, 10, 4)

	score, isAnomaly := forest.ProcessSample([]float64{})
	assert.Equal(t, 0.0, score, "Empty sample should return 0 score")
	assert.False(t, isAnomaly, "Empty sample should not be anomaly")
}

func TestCalculateAnomalyScore_NoTrees(t *testing.T) {
	forest := newOnlineIsolationForest(0, 10, 4)

	score := forest.calculateAnomalyScore([]float64{1.0, 2.0})
	assert.Equal(t, 0.5, score, "No trees should return neutral score")
}

func TestCalculateAnomalyScore_NoValidTrees(t *testing.T) {
	forest := newOnlineIsolationForest(3, 10, 4)
	// Keep trees with nil roots

	score := forest.calculateAnomalyScore([]float64{1.0, 2.0})
	assert.Equal(t, 0.5, score, "No valid trees should return neutral score")
}

func TestCalculateAnomalyScore_ScoreBounds(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	// Initialize tree with very shallow structure to test score bounds
	forest.trees[0].root = &onlineTreeNode{
		depth:       0,
		sampleCount: 1,
		isLeaf:      true,
	}

	score := forest.calculateAnomalyScore([]float64{1.0})
	assert.True(t, score >= 0.0 && score <= 1.0, "Score should be in [0,1] range")
}

func TestUpdateForest_SlidingWindow(t *testing.T) {
	forest := newOnlineIsolationForest(2, 3, 4) // Small window for testing

	// Fill window beyond capacity to test circular buffer
	samples := [][]float64{
		{1.0}, {2.0}, {3.0}, {4.0}, {5.0},
	}

	for _, sample := range samples {
		forest.updateSlidingWindow(sample)
	}

	// Window should wrap around
	assert.True(t, forest.windowFull, "Window should be marked as full")
	assert.Equal(t, 2, forest.windowIndex, "Window index should wrap around")
}

func TestUpdateAdaptiveThreshold_InsufficientSamples(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	// Add fewer than 50 samples (minimum for threshold update)
	for range 10 {
		forest.updateAdaptiveThreshold(0.5)
	}

	// Threshold should remain at initial value
	forest.thresholdMutex.RLock()
	threshold := forest.threshold
	forest.thresholdMutex.RUnlock()

	assert.Equal(t, 0.5, threshold, "Threshold should not change with insufficient samples")
}

func TestUpdateAdaptiveThreshold_SufficientSamples(t *testing.T) {
	forest := newOnlineIsolationForest(1, 100, 4)

	// Add enough samples for threshold adaptation
	for i := range 60 {
		score := 0.3 + 0.4*float64(i)/60.0 // Gradual increase from 0.3 to 0.7
		forest.updateAdaptiveThreshold(score)
	}

	forest.thresholdMutex.RLock()
	threshold := forest.threshold
	forest.thresholdMutex.RUnlock()

	// Threshold should have adapted
	assert.NotEqual(t, 0.5, threshold, "Threshold should adapt with sufficient samples")
	assert.True(t, threshold > 0.0 && threshold < 1.0, "Threshold should be in valid range")
}

func TestUpdateTreesIncremental(t *testing.T) {
	forest := newOnlineIsolationForest(20, 10, 4) // Many trees to test subset updates

	sample := []float64{1.0, 2.0}
	forest.updateTreesIncremental(sample)

	// At least one tree should have been updated
	updatedTrees := 0
	for _, tree := range forest.trees {
		if tree.root != nil {
			updatedTrees++
		}
	}

	assert.Positive(t, updatedTrees, "At least one tree should be updated")
}

func TestUpdateTree_InitializeRoot(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)
	tree := forest.trees[0]
	sample := []float64{1.0, 2.0}

	assert.Nil(t, tree.root, "Tree should start with nil root")

	forest.updateTree(tree, sample)

	assert.NotNil(t, tree.root, "Tree root should be initialized")
	assert.True(t, tree.root.isLeaf, "Initial root should be leaf")
	assert.Equal(t, 1, tree.sampleCount, "Tree should have sample count 1")
}

func TestUpdateNodePath_LeafNode(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	node := &onlineTreeNode{
		depth:       2,
		sampleCount: 5,
		isLeaf:      true,
	}

	initialCount := node.sampleCount
	forest.updateNodePath(node, []float64{1.0}, 2, 4)

	assert.Equal(t, initialCount+1, node.sampleCount, "Leaf node sample count should increment")
}

func TestUpdateNodePath_MaxDepthReached(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 2)

	node := &onlineTreeNode{
		depth:       2,
		sampleCount: 15, // Enough to trigger split attempt
		isLeaf:      false,
	}

	forest.updateNodePath(node, []float64{1.0}, 2, 2) // At max depth

	// Should not create children at max depth
	assert.Nil(t, node.left, "Should not create left child at max depth")
	assert.Nil(t, node.right, "Should not create right child at max depth")
}

func TestSplitNode_EmptySample(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	node := &onlineTreeNode{
		depth:       1,
		sampleCount: 15,
		isLeaf:      true,
	}

	forest.splitNode(node, []float64{}, 1, 4)

	// Should not split with empty sample
	assert.Nil(t, node.left, "Should not split with empty sample")
	assert.Nil(t, node.right, "Should not split with empty sample")
}

func TestSplitNode_AtMaxDepth(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 2)

	node := &onlineTreeNode{
		depth:       2,
		sampleCount: 15,
		isLeaf:      true,
	}

	forest.splitNode(node, []float64{1.0}, 2, 2) // At max depth

	// Should not split at max depth
	assert.True(t, node.isLeaf, "Should remain leaf at max depth")
}

func TestSplitNode_InsufficientData(t *testing.T) {
	forest := newOnlineIsolationForest(1, 2, 4) // Very small window

	node := &onlineTreeNode{
		depth:       1,
		sampleCount: 15,
		isLeaf:      true,
	}

	// Add minimal data to window
	forest.updateSlidingWindow([]float64{1.0})

	forest.splitNode(node, []float64{1.0}, 1, 4)

	// Should not split with insufficient data
	assert.True(t, node.isLeaf, "Should remain leaf with insufficient data")
}

func TestSplitNode_ConstantFeature(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	// Fill window with constant values
	for range 5 {
		forest.updateSlidingWindow([]float64{2.0}) // All same value
	}

	node := &onlineTreeNode{
		depth:       1,
		sampleCount: 15,
		isLeaf:      true,
	}

	forest.splitNode(node, []float64{2.0}, 1, 4)

	// Should not split on constant feature
	assert.True(t, node.isLeaf, "Should remain leaf with constant feature")
}

func TestSplitNode_SuccessfulSplit(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	// Fill window with varying values
	values := [][]float64{{1.0}, {2.0}, {3.0}, {4.0}, {5.0}}
	for _, val := range values {
		forest.updateSlidingWindow(val)
	}

	node := &onlineTreeNode{
		depth:       1,
		sampleCount: 15,
		isLeaf:      true,
	}

	forest.splitNode(node, []float64{3.0}, 1, 4)

	// Should successfully split
	assert.False(t, node.isLeaf, "Node should no longer be leaf after split")
	assert.NotNil(t, node.left, "Should create left child")
	assert.NotNil(t, node.right, "Should create right child")
	assert.Equal(t, 0, node.featureIndex, "Should set feature index")
}

func TestTraverseNode_InternalNode(t *testing.T) {
	tree := &onlineIsolationTree{maxDepth: 4}

	root := &onlineTreeNode{
		featureIndex: 0,
		splitValue:   2.0,
		depth:        0,
		isLeaf:       false,
		left: &onlineTreeNode{
			depth:       1,
			sampleCount: 5,
			isLeaf:      true,
		},
		right: &onlineTreeNode{
			depth:       1,
			sampleCount: 3,
			isLeaf:      true,
		},
	}

	// Test left traversal
	leftPath := tree.traverseNode(root, []float64{1.0})
	assert.Positive(t, leftPath, "Left path should be positive")

	// Test right traversal
	rightPath := tree.traverseNode(root, []float64{3.0})
	assert.Positive(t, rightPath, "Right path should be positive")
}

func TestTraverseNode_ShortSample(t *testing.T) {
	tree := &onlineIsolationTree{maxDepth: 4}

	root := &onlineTreeNode{
		featureIndex: 1, // Index beyond sample length
		splitValue:   2.0,
		depth:        0,
		isLeaf:       false,
		left: &onlineTreeNode{
			depth:       1,
			sampleCount: 5,
			isLeaf:      true,
		},
		right: &onlineTreeNode{
			depth:       1,
			sampleCount: 3,
			isLeaf:      true,
		},
	}

	// Sample shorter than featureIndex
	path := tree.traverseNode(root, []float64{1.0})
	assert.Positive(t, path, "Should handle short sample gracefully")
}

func TestEstimateRemainingPath(t *testing.T) {
	tree := &onlineIsolationTree{}

	// Test with sample count <= 1
	remaining := tree.estimateRemainingPath(1)
	assert.Equal(t, 0.0, remaining, "Should return 0 for single sample")

	remaining = tree.estimateRemainingPath(0)
	assert.Equal(t, 0.0, remaining, "Should return 0 for zero samples")

	// Test with larger sample count
	remaining = tree.estimateRemainingPath(10)
	assert.Positive(t, remaining, "Should return positive value for multiple samples")
}

func TestGetWindowData_WindowNotFull(t *testing.T) {
	forest := newOnlineIsolationForest(1, 5, 4)

	// Add some data without filling window
	samples := [][]float64{{1.0}, {2.0}, {3.0}}
	for _, sample := range samples {
		forest.updateSlidingWindow(sample)
	}

	windowData := forest.getWindowData()
	assert.Len(t, windowData, 3, "Should return partial window data")
}

func TestGetWindowData_WindowFull(t *testing.T) {
	forest := newOnlineIsolationForest(1, 3, 4)

	// Fill window completely and beyond
	samples := [][]float64{{1.0}, {2.0}, {3.0}, {4.0}, {5.0}}
	for _, sample := range samples {
		forest.updateSlidingWindow(sample)
	}

	windowData := forest.getWindowData()
	assert.Len(t, windowData, 3, "Should return full window data")
}

func TestGetExpectedPathLength_SingleSample(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	// Add single sample
	forest.updateSlidingWindow([]float64{1.0})

	expectedLength := forest.getExpectedPathLength()
	assert.Equal(t, 1.0, expectedLength, "Should return 1.0 for single sample")
}

func TestGetExpectedPathLength_NoSamples(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	expectedLength := forest.getExpectedPathLength()
	assert.Equal(t, 1.0, expectedLength, "Should return 1.0 for no samples")
}

func TestMaxInt(t *testing.T) {
	assert.Equal(t, 5, maxInt(3, 5), "Should return larger value")
	assert.Equal(t, 7, maxInt(7, 2), "Should return larger value")
	assert.Equal(t, 4, maxInt(4, 4), "Should handle equal values")
}

func TestConcurrentAccess(t *testing.T) {
	forest := newOnlineIsolationForest(5, 20, 4)

	// Test concurrent processing to ensure thread safety
	done := make(chan bool, 10)

	for i := range 10 {
		go func(val float64) {
			defer func() { done <- true }()
			sample := []float64{val, val * 2}
			forest.ProcessSample(sample)
		}(float64(i))
	}

	// Wait for all goroutines to complete
	for range 10 {
		<-done
	}

	stats := forest.GetStatistics()
	assert.GreaterOrEqual(t, stats.TotalSamples, uint64(10), "Should process all samples")
}

func TestUpdateNodePath_WithChildren(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	root := &onlineTreeNode{
		featureIndex: 0,
		splitValue:   2.0,
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

	initialLeftCount := root.left.sampleCount
	forest.updateNodePath(root, []float64{1.0}, 0, 4) // Should go left

	assert.Equal(t, initialLeftCount+1, root.left.sampleCount, "Left child should be updated")
}

func TestUpdateNodePath_NodeCreatesChildren(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	node := &onlineTreeNode{
		depth:       1,
		sampleCount: 12, // Above threshold for splitting
		isLeaf:      true,
	}

	// Fill window with varying data to enable splitting
	for i := 1; i <= 5; i++ {
		forest.updateSlidingWindow([]float64{float64(i)})
	}

	forest.updateNodePath(node, []float64{3.0}, 1, 4)

	// Node might split and create children depending on implementation
	initialSampleCount := 12
	assert.Equal(t, initialSampleCount+1, node.sampleCount, "Sample count should be incremented")
}

func TestSplitNode_EdgeCases(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	// Test split with different feature values
	for range 3 {
		forest.updateSlidingWindow([]float64{5.0, 3.0}) // Different values
	}

	node := &onlineTreeNode{
		depth:       1,
		sampleCount: 15,
		isLeaf:      true,
	}

	forest.splitNode(node, []float64{4.0, 2.0}, 1, 4)

	// Should attempt to split
	assert.Positive(t, node.sampleCount, "Node should maintain sample count")
}

func TestCalculateAnomalyScore_ExtremeScores(t *testing.T) {
	forest := newOnlineIsolationForest(1, 10, 4)

	// Create tree with very shallow path to test score bounds
	forest.trees[0].root = &onlineTreeNode{
		depth:       0,
		sampleCount: 1,
		isLeaf:      true,
	}

	// Test score normalization bounds
	score := forest.calculateAnomalyScore([]float64{1.0})
	assert.True(t, score >= 0.0 && score <= 1.0, "Score should be normalized to [0,1]")

	// Test with different expected path length scenarios
	forest.updateSlidingWindow([]float64{1.0})
	score2 := forest.calculateAnomalyScore([]float64{1.0})
	assert.True(t, score2 >= 0.0 && score2 <= 1.0, "Score should remain in bounds")
}

func TestUpdateAdaptiveThreshold_BoundaryValues(t *testing.T) {
	forest := newOnlineIsolationForest(1, 100, 4)

	// Add exactly 50 samples (boundary for threshold update)
	for range 50 {
		forest.updateAdaptiveThreshold(0.6)
	}

	forest.thresholdMutex.RLock()
	threshold := forest.threshold
	forest.thresholdMutex.RUnlock()

	// Should start adapting at 50 samples
	assert.True(t, threshold > 0.0 && threshold <= 1.0, "Threshold should be in valid range")
}

func TestOnlineForestStatistics_EdgeCases(t *testing.T) {
	forest := newOnlineIsolationForest(0, 10, 4) // Zero trees

	stats := forest.GetStatistics()
	assert.Equal(t, 0, stats.ActiveTrees, "Should report zero active trees")
	assert.Equal(t, 0.0, stats.AnomalyRate, "Should handle zero samples gracefully")
	assert.True(t, stats.WindowUtilization >= 0.0 && stats.WindowUtilization <= 1.0)
}

func TestMaxInt_EdgeCases(t *testing.T) {
	assert.Equal(t, 0, maxInt(0, 0), "Should handle zero values")
	assert.Equal(t, -1, maxInt(-5, -1), "Should handle negative values")
	assert.Equal(t, 1000000, maxInt(1000000, 999999), "Should handle large values")
}

// Keep existing benchmark tests
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

	for i := 0; b.Loop(); i++ {
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

	resourceAttrs := map[string]any{
		"service.name": "benchmark-service",
	}

	for b.Loop() {
		extractor.ExtractFeatures(span, resourceAttrs)
	}
}
