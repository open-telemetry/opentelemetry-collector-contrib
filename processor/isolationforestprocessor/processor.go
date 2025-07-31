// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package isolationforestprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor"

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor/internal/metadata"
)

// isolationForestProcessor implements the Isolation Forest anomaly detection algorithm
type isolationForestProcessor struct {
	config       *Config
	logger       *zap.Logger
	nextConsumer consumer.Metrics
	telemetry    *metadata.TelemetryBuilder

	// Model state
	trees        []*isolationTree
	dataWindow   *slidingWindow
	lastTraining time.Time
	mu           sync.RWMutex

	// Metrics
	processedCount int64
	anomalyCount   int64
}

// isolationTree represents a single isolation tree
type isolationTree struct {
	root     *treeNode
	maxDepth int
}

// treeNode represents a node in the isolation tree
type treeNode struct {
	splitFeature int
	splitValue   float64
	left         *treeNode
	right        *treeNode
	isLeaf       bool
	size         int
}

// dataPoint represents a single data point for analysis
type dataPoint struct {
	timestamp time.Time
	features  []float64
	metadata  map[string]interface{}
}

// slidingWindow maintains a sliding window of data points
type slidingWindow struct {
	data    []dataPoint
	maxSize int
	mu      sync.RWMutex
}

// newIsolationForestProcessor creates a new isolation forest processor
func newIsolationForestProcessor(
	set processor.Settings,
	config *Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	telemetry, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	return &isolationForestProcessor{
		config:       config,
		logger:       set.Logger,
		nextConsumer: nextConsumer,
		telemetry:    telemetry,
		dataWindow:   newSlidingWindow(config.WindowSize),
		lastTraining: time.Now(),
	}, nil
}

// newSlidingWindow creates a new sliding window
func newSlidingWindow(maxSize int) *slidingWindow {
	return &slidingWindow{
		data:    make([]dataPoint, 0, maxSize),
		maxSize: maxSize,
	}
}

// add adds a data point to the sliding window
func (sw *slidingWindow) add(dp dataPoint) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.data = append(sw.data, dp)
	if len(sw.data) > sw.maxSize {
		sw.data = sw.data[1:]
	}
}

// getData returns a copy of the current data
func (sw *slidingWindow) getData() []dataPoint {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	result := make([]dataPoint, len(sw.data))
	copy(result, sw.data)
	return result
}

// Capabilities returns the consumer capabilities
func (ifp *isolationForestProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start starts the processor
func (ifp *isolationForestProcessor) Start(ctx context.Context, host component.Host) error {
	ifp.logger.Info("Starting Isolation Forest processor",
		zap.Int("num_trees", ifp.config.NumTrees),
		zap.Int("subsample_size", ifp.config.SubsampleSize),
		zap.Float64("anomaly_threshold", ifp.config.AnomalyThreshold),
	)
	return nil
}

// Shutdown stops the processor
func (ifp *isolationForestProcessor) Shutdown(ctx context.Context) error {
	ifp.logger.Info("Shutting down Isolation Forest processor",
		zap.Int64("processed_count", ifp.processedCount),
		zap.Int64("anomaly_count", ifp.anomalyCount),
	)
	return nil
}

// ConsumeMetrics processes the metrics data
func (ifp *isolationForestProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	now := time.Now()

	// Check if we need to retrain the model
	if now.Sub(ifp.lastTraining) > ifp.config.TrainingInterval {
		go ifp.trainModel()
	}

	// Process each metric
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		ifp.processResourceMetrics(rm)
	}

	// Update telemetry
	ifp.telemetry.IsolationforestProcessedMetrics.Add(ctx, 1)

	return ifp.nextConsumer.ConsumeMetrics(ctx, md)
}

// processResourceMetrics processes resource metrics
func (ifp *isolationForestProcessor) processResourceMetrics(rm pmetric.ResourceMetrics) {
	for i := 0; i < rm.ScopeMetrics().Len(); i++ {
		sm := rm.ScopeMetrics().At(i)
		ifp.processScopeMetrics(sm)
	}
}

// processScopeMetrics processes scope metrics
func (ifp *isolationForestProcessor) processScopeMetrics(sm pmetric.ScopeMetrics) {
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		ifp.processMetric(metric)
	}
}

// processMetric processes a single metric
func (ifp *isolationForestProcessor) processMetric(metric pmetric.Metric) {
	metricName := metric.Name()

	// Check if this metric should be analyzed
	if len(ifp.config.MetricsToAnalyze) > 0 {
		shouldAnalyze := false
		for _, name := range ifp.config.MetricsToAnalyze {
			if name == metricName {
				shouldAnalyze = true
				break
			}
		}
		if !shouldAnalyze {
			return
		}
	}

	// Extract data points based on metric type
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		ifp.processGauge(metric.Gauge())
	case pmetric.MetricTypeSum:
		ifp.processSum(metric.Sum())
	case pmetric.MetricTypeHistogram:
		ifp.processHistogram(metric.Histogram())
	case pmetric.MetricTypeSummary:
		ifp.processSummary(metric.Summary())
	}
}

// processGauge processes gauge metrics
func (ifp *isolationForestProcessor) processGauge(gauge pmetric.Gauge) {
	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dp := gauge.DataPoints().At(i)
		features := ifp.extractFeatures(dp.DoubleValue(), dp.Attributes())

		dataPoint := dataPoint{
			timestamp: time.Unix(0, int64(dp.Timestamp())),
			features:  features,
			metadata: map[string]interface{}{
				"value": dp.DoubleValue(),
			},
		}

		ifp.dataWindow.add(dataPoint)
		ifp.analyzeDataPoint(dataPoint, dp)
		ifp.processedCount++
	}
}

// processSum processes sum metrics
func (ifp *isolationForestProcessor) processSum(sum pmetric.Sum) {
	for i := 0; i < sum.DataPoints().Len(); i++ {
		dp := sum.DataPoints().At(i)
		features := ifp.extractFeatures(dp.DoubleValue(), dp.Attributes())

		dataPoint := dataPoint{
			timestamp: time.Unix(0, int64(dp.Timestamp())),
			features:  features,
			metadata: map[string]interface{}{
				"value": dp.DoubleValue(),
			},
		}

		ifp.dataWindow.add(dataPoint)
		ifp.analyzeDataPoint(dataPoint, dp)
		ifp.processedCount++
	}
}

// processHistogram processes histogram metrics
func (ifp *isolationForestProcessor) processHistogram(histogram pmetric.Histogram) {
	for i := 0; i < histogram.DataPoints().Len(); i++ {
		dp := histogram.DataPoints().At(i)
		// Use count and sum as features
		features := []float64{float64(dp.Count()), dp.Sum()}

		dataPoint := dataPoint{
			timestamp: time.Unix(0, int64(dp.Timestamp())),
			features:  features,
			metadata: map[string]interface{}{
				"count": dp.Count(),
				"sum":   dp.Sum(),
			},
		}

		ifp.dataWindow.add(dataPoint)
		ifp.processedCount++
	}
}

// processSummary processes summary metrics
func (ifp *isolationForestProcessor) processSummary(summary pmetric.Summary) {
	for i := 0; i < summary.DataPoints().Len(); i++ {
		dp := summary.DataPoints().At(i)
		// Use count and sum as features
		features := []float64{float64(dp.Count()), dp.Sum()}

		dataPoint := dataPoint{
			timestamp: time.Unix(0, int64(dp.Timestamp())),
			features:  features,
			metadata: map[string]interface{}{
				"count": dp.Count(),
				"sum":   dp.Sum(),
			},
		}

		ifp.dataWindow.add(dataPoint)
		ifp.processedCount++
	}
}

// extractFeatures extracts features from a metric value and attributes
func (ifp *isolationForestProcessor) extractFeatures(value float64, attributes pcommon.Map) []float64 {
	if len(ifp.config.Features) == 0 {
		return []float64{value}
	}

	features := make([]float64, 0, len(ifp.config.Features))
	for _, featureName := range ifp.config.Features {
		if attr, ok := attributes.Get(featureName); ok {
			switch attr.Type() {
			case pcommon.ValueTypeDouble:
				features = append(features, attr.Double())
			case pcommon.ValueTypeInt:
				features = append(features, float64(attr.Int()))
			}
		}
	}

	// Always include the metric value
	features = append(features, value)
	return features
}

// analyzeDataPoint analyzes a data point for anomalies
func (ifp *isolationForestProcessor) analyzeDataPoint(dp dataPoint, metricDP interface{}) {
	ifp.mu.RLock()
	trees := ifp.trees
	ifp.mu.RUnlock()

	if len(trees) == 0 {
		return // Model not trained yet
	}

	// Calculate anomaly score
	score := ifp.calculateAnomalyScore(dp.features)

	// Add anomaly score as attribute if configured
	if ifp.config.AddAnomalyScore {
		ifp.addAnomalyScoreAttribute(metricDP, score)
	}

	// Check if it's anomalous
	if score > ifp.config.AnomalyThreshold {
		ifp.anomalyCount++
		ifp.telemetry.IsolationforestAnomaliesDetected.Add(context.Background(), 1)
		ifp.logger.Debug("Anomaly detected",
			zap.Float64("score", score),
			zap.Any("features", dp.features),
			zap.Time("timestamp", dp.timestamp),
		)
	}
}

// addAnomalyScoreAttribute adds the anomaly score as an attribute
func (ifp *isolationForestProcessor) addAnomalyScoreAttribute(metricDP interface{}, score float64) {
	switch dp := metricDP.(type) {
	case pmetric.NumberDataPoint:
		dp.Attributes().PutDouble("anomaly_score", score)
	case pmetric.HistogramDataPoint:
		dp.Attributes().PutDouble("anomaly_score", score)
	case pmetric.SummaryDataPoint:
		dp.Attributes().PutDouble("anomaly_score", score)
	}
}

// trainModel trains the isolation forest model
func (ifp *isolationForestProcessor) trainModel() {
	startTime := time.Now()
	ifp.logger.Info("Training Isolation Forest model")

	data := ifp.dataWindow.getData()
	if len(data) < ifp.config.SubsampleSize {
		ifp.logger.Debug("Insufficient data for training", zap.Int("data_points", len(data)))
		return
	}

	// Build isolation trees
	trees := make([]*isolationTree, ifp.config.NumTrees)
	for i := 0; i < ifp.config.NumTrees; i++ {
		trees[i] = ifp.buildTree(data)
	}

	ifp.mu.Lock()
	ifp.trees = trees
	ifp.lastTraining = time.Now()
	ifp.mu.Unlock()

	trainingDuration := time.Since(startTime)
	ifp.telemetry.IsolationforestModelTrainingDuration.Record(context.Background(), trainingDuration.Milliseconds())

	ifp.logger.Info("Isolation Forest model trained successfully",
		zap.Int("num_trees", len(trees)),
		zap.Int("data_points", len(data)),
		zap.Duration("training_duration", trainingDuration),
	)
}

// buildTree builds a single isolation tree
func (ifp *isolationForestProcessor) buildTree(data []dataPoint) *isolationTree {
	// Sample data for this tree
	subsample := ifp.subsample(data, ifp.config.SubsampleSize)

	// Build the tree
	maxDepth := int(math.Ceil(math.Log2(float64(len(subsample)))))
	root := ifp.buildTreeNode(subsample, 0, maxDepth)

	return &isolationTree{
		root:     root,
		maxDepth: maxDepth,
	}
}

// subsample randomly samples data points
func (ifp *isolationForestProcessor) subsample(data []dataPoint, size int) []dataPoint {
	if len(data) <= size {
		return data
	}

	// Create indices and shuffle them
	indices := make([]int, len(data))
	for i := range indices {
		indices[i] = i
	}

	// Fisher-Yates shuffle
	for i := len(indices) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		indices[i], indices[j] = indices[j], indices[i]
	}

	result := make([]dataPoint, size)
	for i := 0; i < size; i++ {
		result[i] = data[indices[i]]
	}
	return result
}

// buildTreeNode recursively builds tree nodes
func (ifp *isolationForestProcessor) buildTreeNode(data []dataPoint, depth, maxDepth int) *treeNode {
	node := &treeNode{size: len(data)}

	// Stop conditions
	if len(data) <= 1 || depth >= maxDepth {
		node.isLeaf = true
		return node
	}

	// Find feature dimensions
	if len(data) == 0 || len(data[0].features) == 0 {
		node.isLeaf = true
		return node
	}

	numFeatures := len(data[0].features)

	// Randomly select a feature
	splitFeature := rand.Intn(numFeatures)

	// Find min and max values for the selected feature
	minVal, maxVal := math.Inf(1), math.Inf(-1)
	for _, dp := range data {
		if splitFeature < len(dp.features) {
			val := dp.features[splitFeature]
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
		}
	}

	if minVal >= maxVal {
		node.isLeaf = true
		return node
	}

	// Random split value
	splitValue := minVal + rand.Float64()*(maxVal-minVal)

	node.splitFeature = splitFeature
	node.splitValue = splitValue

	// Split data
	var leftData, rightData []dataPoint
	for _, dp := range data {
		if splitFeature < len(dp.features) {
			if dp.features[splitFeature] < splitValue {
				leftData = append(leftData, dp)
			} else {
				rightData = append(rightData, dp)
			}
		}
	}

	// Build child nodes
	if len(leftData) > 0 {
		node.left = ifp.buildTreeNode(leftData, depth+1, maxDepth)
	}
	if len(rightData) > 0 {
		node.right = ifp.buildTreeNode(rightData, depth+1, maxDepth)
	}

	return node
}

// calculateAnomalyScore calculates the anomaly score for a data point
func (ifp *isolationForestProcessor) calculateAnomalyScore(features []float64) float64 {
	if len(ifp.trees) == 0 {
		return 0.0
	}

	totalPathLength := 0.0
	for _, tree := range ifp.trees {
		pathLength := ifp.getPathLength(tree.root, features, 0)
		totalPathLength += pathLength
	}

	avgPathLength := totalPathLength / float64(len(ifp.trees))

	// Normalize the path length to get anomaly score
	// Shorter paths indicate anomalies
	c := ifp.averagePathLengthBST(ifp.config.SubsampleSize)
	if c <= 0 {
		return 0.0
	}

	score := math.Pow(2.0, -avgPathLength/c)
	return score
}

// getPathLength calculates the path length from root to leaf for given features
func (ifp *isolationForestProcessor) getPathLength(node *treeNode, features []float64, depth int) float64 {
	if node == nil || node.isLeaf {
		// Add the average path length of unsuccessful search in BST
		return float64(depth) + ifp.averagePathLengthBST(node.size)
	}

	if node.splitFeature < len(features) {
		if features[node.splitFeature] < node.splitValue {
			return ifp.getPathLength(node.left, features, depth+1)
		} else {
			return ifp.getPathLength(node.right, features, depth+1)
		}
	}

	return float64(depth)
}

// averagePathLengthBST calculates the average path length of unsuccessful search in BST
func (ifp *isolationForestProcessor) averagePathLengthBST(n int) float64 {
	if n <= 1 {
		return 0.0
	}
	return 2.0*((math.Log(float64(n-1)) + 0.5772156649) - (2.0*float64(n-1))/float64(n))
}
