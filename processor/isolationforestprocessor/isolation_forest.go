// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// isolation_forest.go - Core isolation forest algorithm implementation
package isolationforestprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor"

import (
	"math"
	rand "math/rand/v2"
	"sync"
	"time"
)

// onlineIsolationForest represents an isolation forest that can learn incrementally
// from streaming data. Unlike traditional isolation forests that require batch training,
// this implementation updates its models continuously as new data arrives.
type onlineIsolationForest struct {
	// Core configuration
	numTrees   int // Number of trees in the forest
	maxDepth   int // Maximum depth for trees
	windowSize int // Size of sliding window for recent data

	// Trees and their associated data
	trees      []*onlineIsolationTree // Collection of online isolation trees
	treesMutex sync.RWMutex           // Protects concurrent access to trees

	// Sliding window management for incremental learning
	dataWindow  [][]float64 // Recent data points for tree updates
	windowIndex int         // Current position in circular buffer
	windowFull  bool        // Whether the window has been filled once
	windowMutex sync.RWMutex

	// Adaptive threshold management
	scoreHistory   []float64 // Recent anomaly scores for threshold adaptation
	threshold      float64   // Current adaptive threshold
	thresholdMutex sync.RWMutex

	// Statistics and monitoring
	totalSamples uint64 // Total number of samples processed
	anomalyCount uint64 // Total number of anomalies detected
	statsMutex   sync.RWMutex

	// Random number generation
	rng      *rand.Rand // Random number generator for reproducible results
	rngMutex sync.Mutex // Protects RNG access
}

// OnlineIsolationTree represents a single tree that can be updated incrementally.
type onlineIsolationTree struct {
	root        *onlineTreeNode // Root node of the tree
	maxDepth    int             // Maximum allowed depth
	sampleCount int             // Number of samples seen by this tree
	updateCount int             // Number of incremental updates performed

	// Tree update statistics for monitoring tree health
	lastUpdateTime time.Time // When this tree was last updated
}

// OnlineTreeNode represents a node in an online isolation tree.
type onlineTreeNode struct {
	// Split condition (for internal nodes)
	featureIndex int     // Index of feature to split on
	splitValue   float64 // Value to split at

	// Node statistics for incremental updates
	sampleCount int // Number of samples that have passed through this node
	depth       int // Depth of this node in the tree

	// Child nodes
	left  *onlineTreeNode // Left child (feature < splitValue)
	right *onlineTreeNode // Right child (feature >= splitValue)

	// Leaf node properties
	isLeaf         bool    // Whether this is a leaf node
	isolationScore float64 // Cached isolation score for leaf nodes
}

// OnlineForestStatistics holds performance and monitoring data.
type onlineForestStatistics struct {
	TotalSamples      uint64  // Total samples processed
	AnomalyCount      uint64  // Total anomalies detected
	AnomalyRate       float64 // Proportion of anomalies
	CurrentThreshold  float64 // Current adaptive threshold
	WindowUtilization float64 // How full the sliding window is
	ActiveTrees       int     // Number of active trees
}

// newOnlineIsolationForest creates a new online isolation forest with the specified parameters.
func newOnlineIsolationForest(numTrees, windowSize, maxDepth int) *onlineIsolationForest {
	if maxDepth <= 0 {
		maxDepth = int(math.Ceil(math.Log2(float64(windowSize))))
	}
	seed := uint64(time.Now().UnixNano())
	forest := &onlineIsolationForest{
		numTrees:     numTrees,
		maxDepth:     maxDepth,
		windowSize:   windowSize,
		trees:        make([]*onlineIsolationTree, numTrees),
		dataWindow:   make([][]float64, windowSize),
		scoreHistory: make([]float64, 0, windowSize),
		threshold:    0.5, // Initial threshold, will adapt based on data
		rng:          rand.New(rand.NewPCG(seed, seed^0x9e3779e97f4c7c15)),
	}

	// Initialize trees with minimal structure
	for i := range numTrees {
		forest.trees[i] = &onlineIsolationTree{
			maxDepth:       maxDepth,
			lastUpdateTime: time.Now(),
		}
	}
	return forest
}

// ProcessSample processes a single data point, updating the forest incrementally
// and returning an anomaly score immediately.
func (oif *onlineIsolationForest) ProcessSample(sample []float64) (float64, bool) {
	if len(sample) == 0 {
		return 0.0, false
	}

	// Calculate anomaly score using current trees
	anomalyScore := oif.calculateAnomalyScore(sample)

	// Determine if this is an anomaly based on adaptive threshold
	oif.thresholdMutex.RLock()
	currentThreshold := oif.threshold
	oif.thresholdMutex.RUnlock()
	isAnomaly := anomalyScore > currentThreshold

	// Update statistics
	oif.statsMutex.Lock()
	oif.totalSamples++
	if isAnomaly {
		oif.anomalyCount++
	}
	oif.statsMutex.Unlock()

	// Update the forest with this new sample (asynchronous to avoid blocking)
	go oif.updateForest(sample, anomalyScore)

	return anomalyScore, isAnomaly
}

// calculateAnomalyScore computes the anomaly score by averaging path lengths across all trees.
func (oif *onlineIsolationForest) calculateAnomalyScore(sample []float64) float64 {
	oif.treesMutex.RLock()
	defer oif.treesMutex.RUnlock()

	if len(oif.trees) == 0 {
		return 0.5 // Neutral score if no trees available
	}

	totalPathLength := 0.0
	validTrees := 0

	for _, tree := range oif.trees {
		if tree.root != nil {
			pathLength := tree.calculatePathLength(sample)
			totalPathLength += pathLength
			validTrees++
		}
	}

	if validTrees == 0 {
		return 0.5 // Neutral score if no valid trees
	}

	avgPathLength := totalPathLength / float64(validTrees)

	// Normalize path length to anomaly score using the expected path length formula
	expectedPathLength := oif.getExpectedPathLength()
	anomalyScore := math.Pow(2, -avgPathLength/expectedPathLength)

	// Ensure score is in valid range [0, 1]
	if anomalyScore < 0 {
		anomalyScore = 0
	} else if anomalyScore > 1 {
		anomalyScore = 1
	}
	return anomalyScore
}

// updateForest incrementally updates the forest with a new sample.
func (oif *onlineIsolationForest) updateForest(sample []float64, anomalyScore float64) {
	// Add sample to sliding window
	oif.updateSlidingWindow(sample)

	// Update adaptive threshold
	oif.updateAdaptiveThreshold(anomalyScore)

	// Incrementally update a subset of trees to distribute computational load
	oif.updateTreesIncremental(sample)
}

// updateSlidingWindow maintains a circular buffer of recent samples for tree updates.
func (oif *onlineIsolationForest) updateSlidingWindow(sample []float64) {
	oif.windowMutex.Lock()
	defer oif.windowMutex.Unlock()

	// Create a copy of the sample to avoid reference issues
	sampleCopy := make([]float64, len(sample))
	copy(sampleCopy, sample)

	// Add to circular buffer
	oif.dataWindow[oif.windowIndex] = sampleCopy
	oif.windowIndex = (oif.windowIndex + 1) % oif.windowSize
	if oif.windowIndex == 0 {
		oif.windowFull = true
	}
}

// updateAdaptiveThreshold adjusts the anomaly threshold based on recent score distribution.
func (oif *onlineIsolationForest) updateAdaptiveThreshold(score float64) {
	oif.thresholdMutex.Lock()
	defer oif.thresholdMutex.Unlock()

	// Add score to history
	oif.scoreHistory = append(oif.scoreHistory, score)

	// Maintain bounded history size
	if len(oif.scoreHistory) > oif.windowSize {
		oif.scoreHistory = oif.scoreHistory[1:]
	}

	// Update threshold based on score distribution (e.g., 90th percentile)
	if len(oif.scoreHistory) >= 50 { // Need sufficient samples for reliable threshold
		sortedScores := make([]float64, len(oif.scoreHistory))
		copy(sortedScores, oif.scoreHistory)

		// Simple insertion sort for small arrays
		for i := 1; i < len(sortedScores); i++ {
			key := sortedScores[i]
			j := i - 1
			for j >= 0 && sortedScores[j] > key {
				sortedScores[j+1] = sortedScores[j]
				j--
			}
			sortedScores[j+1] = key
		}

		// Use 90th percentile as threshold
		thresholdIndex := int(0.9 * float64(len(sortedScores)))
		if thresholdIndex >= len(sortedScores) {
			thresholdIndex = len(sortedScores) - 1
		}
		newThreshold := sortedScores[thresholdIndex]

		// Smooth threshold updates to avoid rapid changes
		oif.threshold = 0.9*oif.threshold + 0.1*newThreshold
	}
}

// updateTreesIncremental updates a random subset of trees with the new sample.
func (oif *onlineIsolationForest) updateTreesIncremental(sample []float64) {
	oif.treesMutex.Lock()
	defer oif.treesMutex.Unlock()

	// Update a random subset of trees (e.g., 10% per update)
	oif.rngMutex.Lock()
	numTreesToUpdate := maxInt(1, oif.numTrees/10)
	treesToUpdate := oif.rng.Perm(oif.numTrees)[:numTreesToUpdate]
	oif.rngMutex.Unlock()

	for _, treeIndex := range treesToUpdate {
		tree := oif.trees[treeIndex]
		oif.updateTree(tree, sample)
	}
}

// updateTree incrementally updates a single tree with a new sample.
func (oif *onlineIsolationForest) updateTree(tree *onlineIsolationTree, sample []float64) {
	if tree.root == nil {
		// Initialize tree with first sample
		tree.root = &onlineTreeNode{
			depth:          0,
			sampleCount:    1,
			isLeaf:         true,
			isolationScore: 0.5, // Neutral score for single sample
		}
		tree.sampleCount = 1
		tree.lastUpdateTime = time.Now()
		return
	}

	// Traverse tree and update nodes along the path
	oif.updateNodePath(tree.root, sample, 0, tree.maxDepth)
	tree.sampleCount++
	tree.updateCount++
	tree.lastUpdateTime = time.Now()
}

// updateNodePath updates all nodes along the path taken by a sample through the tree.
func (oif *onlineIsolationForest) updateNodePath(node *onlineTreeNode, sample []float64, depth, maxDepth int) {
	node.sampleCount++

	// If this is a leaf or we've reached max depth, stop here
	if node.isLeaf || depth >= maxDepth {
		return
	}

	// If this node needs to be split (has seen enough samples and is currently a leaf)
	if node.left == nil && node.right == nil && node.sampleCount > 10 {
		oif.splitNode(node, sample, depth, maxDepth)
		return
	}

	// Navigate to appropriate child if splits exist
	if node.left != nil && node.right != nil {
		if len(sample) > node.featureIndex && sample[node.featureIndex] < node.splitValue {
			oif.updateNodePath(node.left, sample, depth+1, maxDepth)
		} else {
			oif.updateNodePath(node.right, sample, depth+1, maxDepth)
		}
	}
}

// splitNode creates child nodes for a leaf that has accumulated enough samples.
func (oif *onlineIsolationForest) splitNode(node *onlineTreeNode, sample []float64, depth, maxDepth int) {
	if depth >= maxDepth || len(sample) == 0 {
		return
	}

	// Choose a random feature to split on
	oif.rngMutex.Lock()
	featureIndex := oif.rng.IntN(len(sample))

	// Get current window data to determine split value
	oif.windowMutex.RLock()
	windowData := oif.getWindowData()
	oif.windowMutex.RUnlock()

	var featureValues []float64
	for _, windowSample := range windowData {
		if len(windowSample) > featureIndex {
			featureValues = append(featureValues, windowSample[featureIndex])
		}
	}
	if len(featureValues) < 2 {
		oif.rngMutex.Unlock()
		return // Not enough data to determine split
	}

	// Find min and max values for this feature
	minVal, maxVal := featureValues[0], featureValues[0]
	for _, val := range featureValues {
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}
	if minVal >= maxVal {
		oif.rngMutex.Unlock()
		return // Cannot split on constant feature
	}

	// Choose random split point
	splitValue := minVal + oif.rng.Float64()*(maxVal-minVal)
	oif.rngMutex.Unlock()

	// Create child nodes
	node.featureIndex = featureIndex
	node.splitValue = splitValue
	node.isLeaf = false
	node.left = &onlineTreeNode{
		depth:          depth + 1,
		sampleCount:    1,
		isLeaf:         true,
		isolationScore: 0.5,
	}
	node.right = &onlineTreeNode{
		depth:          depth + 1,
		sampleCount:    1,
		isLeaf:         true,
		isolationScore: 0.5,
	}
}

// calculatePathLength computes the path length for a sample in a single tree.
func (tree *onlineIsolationTree) calculatePathLength(sample []float64) float64 {
	if tree.root == nil {
		return 0.0
	}
	return tree.traverseNode(tree.root, sample)
}

// traverseNode recursively traverses the tree to find the path length for a sample.
func (tree *onlineIsolationTree) traverseNode(node *onlineTreeNode, sample []float64) float64 {
	if node.isLeaf || node.left == nil || node.right == nil {
		// For leaf nodes, add expected remaining path length based on sample count
		return float64(node.depth) + tree.estimateRemainingPath(node.sampleCount)
	}

	// Navigate to appropriate child
	if len(sample) > node.featureIndex && sample[node.featureIndex] < node.splitValue {
		return tree.traverseNode(node.left, sample)
	}
	return tree.traverseNode(node.right, sample)
}

// estimateRemainingPath estimates the remaining path length for a leaf node.
func (*onlineIsolationTree) estimateRemainingPath(sampleCount int) float64 {
	if sampleCount <= 1 {
		return 0.0
	}
	// Use harmonic number approximation for expected remaining path
	return 2.0*(math.Log(float64(sampleCount-1))+0.5772156649) -
		(2.0 * float64(sampleCount-1) / float64(sampleCount))
}

// getWindowData returns a copy of current window data.
func (oif *onlineIsolationForest) getWindowData() [][]float64 {
	var result [][]float64
	if !oif.windowFull {
		// Window not full yet, return data from start to current index
		for i := 0; i < oif.windowIndex; i++ {
			if oif.dataWindow[i] != nil {
				result = append(result, oif.dataWindow[i])
			}
		}
	} else {
		// Window is full, return all data
		for i := 0; i < oif.windowSize; i++ {
			if oif.dataWindow[i] != nil {
				result = append(result, oif.dataWindow[i])
			}
		}
	}
	return result
}

// getExpectedPathLength returns the expected path length for normalization.
func (oif *onlineIsolationForest) getExpectedPathLength() float64 {
	// Use adaptive expected path length based on current window size
	oif.windowMutex.RLock()
	windowData := oif.getWindowData()
	sampleSize := len(windowData)
	oif.windowMutex.RUnlock()

	if sampleSize <= 1 {
		return 1.0
	}

	// Use harmonic number approximation
	return 2.0*(math.Log(float64(sampleSize-1))+0.5772156649) -
		(2.0 * float64(sampleSize-1) / float64(sampleSize))
}

// GetStatistics returns performance and health statistics for monitoring.
func (oif *onlineIsolationForest) GetStatistics() onlineForestStatistics {
	oif.statsMutex.RLock()
	defer oif.statsMutex.RUnlock()

	oif.thresholdMutex.RLock()
	currentThreshold := oif.threshold
	oif.thresholdMutex.RUnlock()

	oif.windowMutex.RLock()
	windowUtilization := float64(len(oif.getWindowData())) / float64(oif.windowSize)
	oif.windowMutex.RUnlock()

	anomalyRate := float64(0)
	if oif.totalSamples > 0 {
		anomalyRate = float64(oif.anomalyCount) / float64(oif.totalSamples)
	}

	return onlineForestStatistics{
		TotalSamples:      oif.totalSamples,
		AnomalyCount:      oif.anomalyCount,
		AnomalyRate:       anomalyRate,
		CurrentThreshold:  currentThreshold,
		WindowUtilization: windowUtilization,
		ActiveTrees:       len(oif.trees),
	}
}

// Utility functions
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
