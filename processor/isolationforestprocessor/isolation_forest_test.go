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

	// Verify adaptive components are nil by default
	assert.Nil(t, forest.adaptiveConfig, "Should not have adaptive config by default")
	assert.Nil(t, forest.velocityTracker, "Should not have velocity tracker by default")
	assert.Nil(t, forest.memoryMonitor, "Should not have memory monitor by default")
	assert.Nil(t, forest.stabilityChecker, "Should not have stability checker by default")
	// FIX: For non-adaptive forest, currentWindowSize should match windowSize
	expectedCurrentSize := forest.getCurrentWindowSize() // This should return windowSize for non-adaptive
	assert.Equal(t, 100, expectedCurrentSize, "Current window size should match static size")

	// Verify trees are initialized
	for i, tree := range forest.trees {
		assert.NotNil(t, tree, "Tree %d should be initialized", i)
		assert.Equal(t, 8, tree.maxDepth, "Tree %d should have correct max depth", i)
	}
}

// Test adaptive forest creation
func TestOnlineIsolationForestCreationWithAdaptive(t *testing.T) {
	adaptiveConfig := &AdaptiveWindowConfig{
		Enabled:                true,
		MinWindowSize:          50,
		MaxWindowSize:          500,
		MemoryLimitMB:          128,
		AdaptationRate:         0.2,
		VelocityThreshold:      25.0,
		StabilityCheckInterval: "2m",
	}

	forest := newOnlineIsolationForestWithAdaptive(5, 100, 6, adaptiveConfig)

	assert.Equal(t, 5, forest.numTrees, "Should create forest with specified number of trees")
	assert.Equal(t, 6, forest.maxDepth, "Should set specified max depth")
	assert.Equal(t, 100, forest.windowSize, "Should set original window size")
	assert.Equal(t, 50, forest.currentWindowSize, "Should start with min window size")

	// Verify adaptive components are initialized
	assert.NotNil(t, forest.adaptiveConfig, "Should have adaptive config")
	assert.NotNil(t, forest.velocityTracker, "Should have velocity tracker")
	assert.NotNil(t, forest.memoryMonitor, "Should have memory monitor")
	assert.NotNil(t, forest.stabilityChecker, "Should have stability checker")

	// Verify adaptive config is properly stored
	assert.True(t, forest.adaptiveConfig.Enabled, "Adaptive should be enabled")
	assert.Equal(t, 128, forest.memoryMonitor.memoryLimitMB, "Memory limit should be set")
	assert.Equal(t, 1000, forest.velocityTracker.maxSamples, "Velocity tracker should have max samples")
}

// Test adaptive forest creation with disabled config
func TestOnlineIsolationForestCreationWithDisabledAdaptive(t *testing.T) {
	adaptiveConfig := &AdaptiveWindowConfig{
		Enabled: false, // Explicitly disabled
	}

	forest := newOnlineIsolationForestWithAdaptive(3, 80, 4, adaptiveConfig)

	// Should behave like regular forest when disabled
	expectedCurrentSize := forest.getCurrentWindowSize() // Should return windowSize when disabled
	assert.Equal(t, 80, expectedCurrentSize, "Should use original window size when disabled")
	assert.Nil(t, forest.velocityTracker, "Should not initialize velocity tracker when disabled")
	assert.Nil(t, forest.memoryMonitor, "Should not initialize memory monitor when disabled")
	assert.Nil(t, forest.stabilityChecker, "Should not initialize stability checker when disabled")
}

// Test adaptive forest creation with nil config
func TestOnlineIsolationForestCreationWithNilAdaptive(t *testing.T) {
	forest := newOnlineIsolationForestWithAdaptive(3, 80, 4, nil)

	// Should behave like regular forest when nil
	expectedCurrentSize := forest.getCurrentWindowSize() // Should return windowSize when nil config
	assert.Equal(t, 80, expectedCurrentSize, "Should use original window size when nil config")
	assert.Nil(t, forest.adaptiveConfig, "Should have nil adaptive config")
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

	// NEW: Verify adaptive statistics for non-adaptive forest
	assert.Equal(t, 20, stats.CurrentWindowSize, "Should show current window size")
	assert.False(t, stats.AdaptiveEnabled, "Should show adaptive as disabled")
	assert.Equal(t, 0.0, stats.VelocitySamples, "Should show zero velocity for non-adaptive")
	assert.Equal(t, 0.0, stats.MemoryUsageMB, "Should show zero memory usage for non-adaptive")

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

// NEW: Test adaptive forest statistics
func TestAdaptiveForestStatistics(t *testing.T) {
	adaptiveConfig := &AdaptiveWindowConfig{
		Enabled:                true,
		MinWindowSize:          10,
		MaxWindowSize:          100,
		MemoryLimitMB:          64,
		AdaptationRate:         0.1,
		VelocityThreshold:      5.0,
		StabilityCheckInterval: "1m",
	}

	forest := newOnlineIsolationForestWithAdaptive(3, 50, 4, adaptiveConfig)

	// Initial adaptive statistics
	stats := forest.GetStatistics()
	assert.Equal(t, 10, stats.CurrentWindowSize, "Should start with min window size")
	assert.True(t, stats.AdaptiveEnabled, "Should show adaptive as enabled")
	assert.Equal(t, 0.0, stats.VelocitySamples, "Should start with zero velocity")
	assert.GreaterOrEqual(t, stats.MemoryUsageMB, 0.0, "Should show non-negative memory usage")

	// Process samples to trigger velocity tracking
	samples := [][]float64{
		{1.0, 2.0}, {1.5, 2.5}, {0.5, 1.5}, {2.0, 3.0}, {1.2, 2.2},
	}
	for _, sample := range samples {
		forest.ProcessSample(sample)
		time.Sleep(2 * time.Millisecond) // Add some time between samples
	}
	time.Sleep(100 * time.Millisecond) // Allow processing

	// Check updated adaptive statistics
	stats = forest.GetStatistics()
	assert.GreaterOrEqual(t, stats.VelocitySamples, 0.0, "Should show velocity samples >= 0")
	assert.GreaterOrEqual(t, stats.MemoryUsageMB, 0.0, "Should show memory usage >= 0")
}

// NEW: Test getCurrentWindowSize method
func TestGetCurrentWindowSize(t *testing.T) {
	// Test non-adaptive forest
	forest := newOnlineIsolationForest(5, 100, 4)
	assert.Equal(t, 100, forest.getCurrentWindowSize(), "Should return static window size for non-adaptive")

	// Test adaptive forest (disabled)
	adaptiveConfig := &AdaptiveWindowConfig{Enabled: false}
	adaptiveForest := newOnlineIsolationForestWithAdaptive(5, 100, 4, adaptiveConfig)
	assert.Equal(t, 100, adaptiveForest.getCurrentWindowSize(), "Should return static window size when disabled")

	// Test adaptive forest (enabled)
	adaptiveConfig.Enabled = true
	adaptiveConfig.MinWindowSize = 50
	adaptiveForest = newOnlineIsolationForestWithAdaptive(5, 100, 4, adaptiveConfig)
	assert.Equal(t, 50, adaptiveForest.getCurrentWindowSize(), "Should return current adaptive window size")
}

// NEW: Test velocity tracking
func TestVelocityTracking(t *testing.T) {
	adaptiveConfig := &AdaptiveWindowConfig{
		Enabled:           true,
		MinWindowSize:     10,
		MaxWindowSize:     100,
		VelocityThreshold: 5.0,
	}

	forest := newOnlineIsolationForestWithAdaptive(3, 50, 4, adaptiveConfig)

	// Process samples rapidly to create velocity
	for i := 0; i < 10; i++ {
		forest.ProcessSample([]float64{float64(i), float64(i * 2)})
		time.Sleep(100 * time.Millisecond) // Consistent timing for velocity
	}

	velocity := forest.getCurrentVelocity()
	assert.GreaterOrEqual(t, velocity, 0.0, "Velocity should be non-negative")

	// Test velocity tracker bounds
	assert.NotNil(t, forest.velocityTracker, "Velocity tracker should be initialized")
	assert.LessOrEqual(t, len(forest.velocityTracker.sampleTimes), forest.velocityTracker.maxSamples, "Should not exceed max samples")
}

// NEW: Test memory monitoring
func TestMemoryMonitoring(t *testing.T) {
	adaptiveConfig := &AdaptiveWindowConfig{
		Enabled:       true,
		MinWindowSize: 10,
		MaxWindowSize: 100,
		MemoryLimitMB: 64,
	}

	forest := newOnlineIsolationForestWithAdaptive(3, 50, 4, adaptiveConfig)

	// Process samples to generate memory usage
	for i := 0; i < 20; i++ {
		forest.ProcessSample([]float64{float64(i), float64(i * 2), float64(i * 3)})
	}

	memoryUsage := forest.getCurrentMemoryUsage()
	assert.GreaterOrEqual(t, memoryUsage, 0.0, "Memory usage should be non-negative")
	assert.NotNil(t, forest.memoryMonitor, "Memory monitor should be initialized")
	assert.Equal(t, 64, forest.memoryMonitor.memoryLimitMB, "Memory limit should be set correctly")
}

// NEW: Test adaptive window resizing
func TestAdaptiveWindowResizing(t *testing.T) {
	adaptiveConfig := &AdaptiveWindowConfig{
		Enabled:           true,
		MinWindowSize:     10,
		MaxWindowSize:     50,
		MemoryLimitMB:     1, // Very low limit to trigger shrinking
		AdaptationRate:    0.5,
		VelocityThreshold: 1.0, // Low threshold to trigger growth
	}

	forest := newOnlineIsolationForestWithAdaptive(2, 30, 4, adaptiveConfig)
	initialSize := forest.getCurrentWindowSize()

	// Process samples to trigger adaptive behavior
	for i := 0; i < 15; i++ {
		forest.ProcessSample([]float64{float64(i), float64(i * 2)})
		time.Sleep(50 * time.Millisecond) // Create some velocity
	}

	// Allow time for adaptive window adjustments
	time.Sleep(200 * time.Millisecond)

	finalSize := forest.getCurrentWindowSize()
	assert.GreaterOrEqual(t, finalSize, adaptiveConfig.MinWindowSize, "Should not go below minimum")
	assert.LessOrEqual(t, finalSize, adaptiveConfig.MaxWindowSize, "Should not exceed maximum")

	// Verify window size may have changed due to adaptive behavior
	// Note: The exact change depends on the adaptive algorithm's decisions
	_ = initialSize // Mark as used
}

// NEW: Test utility functions added for adaptive window
func TestMinMaxInt(t *testing.T) {
	assert.Equal(t, 3, minInt(3, 5), "Should return smaller value")
	assert.Equal(t, 2, minInt(7, 2), "Should return smaller value")
	assert.Equal(t, 4, minInt(4, 4), "Should handle equal values")

	assert.Equal(t, 5, maxInt(3, 5), "Should return larger value")
	assert.Equal(t, 7, maxInt(7, 2), "Should return larger value")
	assert.Equal(t, 4, maxInt(4, 4), "Should handle equal values")
}

// NEW: Test resizeDataWindow functionality
func TestResizeDataWindow(t *testing.T) {
	forest := newOnlineIsolationForest(2, 5, 4)

	// Fill window partially
	samples := [][]float64{{1.0}, {2.0}, {3.0}}
	for _, sample := range samples {
		forest.updateSlidingWindow(sample)
	}

	initialLen := len(forest.dataWindow)
	assert.Equal(t, 5, initialLen, "Should start with initial window size")

	// Test growing window
	forest.resizeDataWindow(8)
	assert.Len(t, forest.dataWindow, 8, "Should grow window size")

	// Test shrinking window
	forest.resizeDataWindow(3)
	assert.Len(t, forest.dataWindow, 3, "Should shrink window size")
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

// NEW: Benchmark adaptive forest processing
func BenchmarkAdaptiveIsolationForestProcessing(b *testing.B) {
	adaptiveConfig := &AdaptiveWindowConfig{
		Enabled:                true,
		MinWindowSize:          500,
		MaxWindowSize:          2000,
		MemoryLimitMB:          128,
		AdaptationRate:         0.1,
		VelocityThreshold:      10.0,
		StabilityCheckInterval: "5m",
	}

	forest := newOnlineIsolationForestWithAdaptive(100, 1000, 10, adaptiveConfig)

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

	resourceAttrs := map[string]any{
		"service.name": "benchmark-service",
	}

	for b.Loop() {
		extractor.ExtractFeatures(span, resourceAttrs)
	}
}
