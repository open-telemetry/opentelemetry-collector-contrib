// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// processor.go - Main processor implementation with signal-specific processing methods
package isolationforestprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor"

import (
	"context"
	"fmt"
	"hash/fnv"
	"maps"
	"math"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// isolationForestProcessor is the core processor that contains the isolation forest
// algorithm implementation and coordinates processing across different signal types.
type isolationForestProcessor struct {
	config *Config
	logger *zap.Logger

	// Machine learning components
	defaultForest *onlineIsolationForest            // Default model for single-model mode
	modelForests  map[string]*onlineIsolationForest // Named models for multi-model mode
	forestsMutex  sync.RWMutex                      // Protects forest access

	// Feature extraction components
	traceExtractor   *traceFeatureExtractor
	metricsExtractor *metricsFeatureExtractor
	logsExtractor    *logsFeatureExtractor

	// Performance tracking
	processedCount uint64
	anomalyCount   uint64
	statsMutex     sync.Mutex

	// Model lifecycle management
	lastModelUpdate time.Time
	updateTicker    *time.Ticker
	stopChan        chan struct{}
	shutdownWG      sync.WaitGroup
}

// newIsolationForestProcessor creates a new processor instance with the specified configuration.
// This function initializes all the core components including the isolation forest models,
// feature extractors, and performance monitoring systems.
func newIsolationForestProcessor(config *Config, logger *zap.Logger) (*isolationForestProcessor, error) {
	processor := &isolationForestProcessor{
		config:          config,
		logger:          logger,
		modelForests:    make(map[string]*onlineIsolationForest),
		stopChan:        make(chan struct{}),
		lastModelUpdate: time.Now(),
	}

	// Initialize feature extractors for different signal types
	processor.traceExtractor = newTraceFeatureExtractor(config.Features.Traces, logger)
	processor.metricsExtractor = newMetricsFeatureExtractor(config.Features.Metrics, logger)
	processor.logsExtractor = newLogsFeatureExtractor(config.Features.Logs, logger)

	// Initialize isolation forest models based on configuration mode
	if config.IsMultiModelMode() {
		// Create named models for multi-model configuration
		for _, modelConfig := range config.Models {
			// Use adaptive forest creation if adaptive window is enabled
			var forest *onlineIsolationForest
			if config.IsAdaptiveWindowEnabled() {
				forest = newOnlineIsolationForestWithAdaptive(
					modelConfig.ForestSize,
					config.Performance.BatchSize, // Use global batch size as initial window size
					0,                            // Let forest determine max depth automatically
					config.AdaptiveWindow,        // Pass adaptive configuration
				)
			} else {
				forest = newOnlineIsolationForest(
					modelConfig.ForestSize,
					config.Performance.BatchSize,
					0,
				)
			}
			processor.modelForests[modelConfig.Name] = forest

			logger.Info("Initialized model",
				zap.String("model_name", modelConfig.Name),
				zap.Int("forest_size", modelConfig.ForestSize),
				zap.Int("subsample_size", modelConfig.SubsampleSize),
				zap.Bool("adaptive_window_enabled", config.IsAdaptiveWindowEnabled()),
			)
		}
	} else {
		// Create single default model
		// Use adaptive forest creation if adaptive window is enabled
		if config.IsAdaptiveWindowEnabled() {
			processor.defaultForest = newOnlineIsolationForestWithAdaptive(
				config.ForestSize,
				config.Performance.BatchSize,
				0, // Auto-determine max depth
				config.AdaptiveWindow,
			)
		} else {
			processor.defaultForest = newOnlineIsolationForest(
				config.ForestSize,
				config.Performance.BatchSize,
				0,
			)
		}

		logger.Info("Initialized default model",
			zap.Int("forest_size", config.ForestSize),
			zap.Int("subsample_size", config.SubsampleSize),
			zap.Bool("adaptive_window_enabled", config.IsAdaptiveWindowEnabled()),
		)
	}

	// Start model update ticker for periodic retraining
	updateFreq, err := config.GetUpdateFrequencyDuration()
	if err != nil {
		return nil, fmt.Errorf("failed to parse update frequency: %w", err)
	}

	processor.updateTicker = time.NewTicker(updateFreq)

	return processor, nil
}

// Start initializes the processor
func (p *isolationForestProcessor) Start(_ context.Context, _ component.Host) error {
	p.logger.Info("Starting isolation forest processor")
	// Any additional initialization logic can go here

	// Start the background model update loop
	p.shutdownWG.Go(func() {
		p.modelUpdateLoop()
	})

	return nil
}

// Shutdown gracefully stops the processor and cleans up resources.
func (p *isolationForestProcessor) Shutdown(_ context.Context) error {
	p.logger.Info("Shutting down isolation forest processor")

	// Stop the update ticker
	if p.updateTicker != nil {
		p.updateTicker.Stop()
	}

	// Signal background goroutines to stop
	close(p.stopChan)

	// Wait for all background goroutines to complete
	p.shutdownWG.Wait()

	p.logger.Info("Isolation forest processor shutdown complete")
	return nil
}

// modelUpdateLoop runs periodic model updates in the background to adapt to changing patterns.
func (p *isolationForestProcessor) modelUpdateLoop() {
	for {
		select {
		case <-p.updateTicker.C:
			p.performModelUpdate()
		case <-p.stopChan:
			return
		}
	}
}

// performModelUpdate triggers model retraining based on recent data patterns.
func (p *isolationForestProcessor) performModelUpdate() {
	p.logger.Debug("Performing scheduled model update")

	// Get current statistics from all models
	p.forestsMutex.RLock()
	if p.defaultForest != nil {
		stats := p.defaultForest.GetStatistics()
		// Enhanced logging with adaptive window statistics
		logFields := []zap.Field{
			zap.Uint64("total_samples", stats.TotalSamples),
			zap.Float64("anomaly_rate", stats.AnomalyRate),
			zap.Float64("current_threshold", stats.CurrentThreshold),
		}
		if stats.AdaptiveEnabled {
			logFields = append(logFields,
				zap.Int("current_window_size", stats.CurrentWindowSize),
				zap.Float64("velocity_samples_per_sec", stats.VelocitySamples),
				zap.Float64("memory_usage_mb", stats.MemoryUsageMB),
			)
		}
		p.logger.Debug("Default model statistics", logFields...)
	}

	for name, forest := range p.modelForests {
		stats := forest.GetStatistics()
		// Enhanced logging with adaptive window statistics
		logFields := []zap.Field{
			zap.String("model_name", name),
			zap.Uint64("total_samples", stats.TotalSamples),
			zap.Float64("anomaly_rate", stats.AnomalyRate),
		}
		if stats.AdaptiveEnabled {
			logFields = append(logFields,
				zap.Int("current_window_size", stats.CurrentWindowSize),
				zap.Float64("velocity_samples_per_sec", stats.VelocitySamples),
				zap.Float64("memory_usage_mb", stats.MemoryUsageMB),
			)
		}
		p.logger.Debug("Model statistics", logFields...)
	}
	p.forestsMutex.RUnlock()

	p.lastModelUpdate = time.Now()
}

// processFeatures is the core method that takes extracted features and runs them through
// the isolation forest algorithm to compute anomaly scores and classifications.
func (p *isolationForestProcessor) processFeatures(features map[string][]float64, attributes map[string]any) (float64, bool, string) {
	if len(features) == 0 {
		return 0.0, false, ""
	}

	// Determine which model to use based on configuration
	var forest *onlineIsolationForest
	var modelName string

	p.forestsMutex.RLock()
	defer p.forestsMutex.RUnlock()

	if p.config.IsMultiModelMode() {
		// Find matching model based on attributes
		if modelConfig := p.config.GetModelForAttributes(attributes); modelConfig != nil {
			if f, exists := p.modelForests[modelConfig.Name]; exists {
				forest = f
				modelName = modelConfig.Name
			}
		}

		// Fall back to first available model if no match found
		if forest == nil && len(p.modelForests) > 0 {
			for name, f := range p.modelForests {
				forest = f
				modelName = name
				break
			}
		}
	} else {
		forest = p.defaultForest
		modelName = "default"
	}

	if forest == nil {
		p.logger.Warn("No isolation forest available for processing")
		return 0.0, false, ""
	}

	// Combine all features into a single feature vector
	var combinedFeatures []float64
	for _, featureVector := range features {
		combinedFeatures = append(combinedFeatures, featureVector...)
	}

	if len(combinedFeatures) == 0 {
		return 0.0, false, modelName
	}

	// Process through isolation forest
	anomalyScore, isAnomaly := forest.ProcessSample(combinedFeatures)

	// Update statistics
	p.statsMutex.Lock()
	p.processedCount++
	if isAnomaly {
		p.anomalyCount++
	}
	p.statsMutex.Unlock()

	return anomalyScore, isAnomaly, modelName
}

// processTraces processes trace telemetry
func (p *isolationForestProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	// Honor cancellation/deadline; satisfies unparam + uses ctx.
	if err := ctx.Err(); err != nil {
		return td, err
	}

	// Process each resource scope and its spans
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		resourceAttrs := attributeMapToGeneric(rs.Resource().Attributes())

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)

			// Create a new span slice for filtered spans
			newSpans := ptrace.NewSpanSlice()

			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)

				// Extract features from the span
				features := p.traceExtractor.ExtractFeatures(span, resourceAttrs)

				// Combine span and resource attributes for model selection
				spanAttrs := attributeMapToGeneric(span.Attributes())
				allAttrs := mergeAttributes(resourceAttrs, spanAttrs)

				// Process through isolation forest
				score, isAnomaly, modelName := p.processFeatures(features, allAttrs)

				// Apply processing mode
				if p.config.Mode == "filter" && !isAnomaly {
					// Skip this span - don't add to newSpans
					continue
				}

				// Copy span to new slice
				newSpan := newSpans.AppendEmpty()
				span.CopyTo(newSpan)

				// Add anomaly attributes in enrich or both modes
				if p.config.Mode == "enrich" || p.config.Mode == "both" {
					newSpan.Attributes().PutDouble(p.config.ScoreAttribute, score)
					newSpan.Attributes().PutBool(p.config.ClassificationAttribute, isAnomaly)
					if modelName != "" && modelName != "default" {
						newSpan.Attributes().PutStr("anomaly.model_name", modelName)
					}
				}
			}

			// Replace the original spans with filtered/enriched spans
			ss.Spans().RemoveIf(func(_ ptrace.Span) bool { return true })
			for k := 0; k < newSpans.Len(); k++ {
				newSpan := ss.Spans().AppendEmpty()
				newSpans.At(k).CopyTo(newSpan)
			}
		}
	}

	return td, nil
}

// processMetrics processes metric telemetry
func (p *isolationForestProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	// Honor cancellation/deadline; satisfies unparam + uses ctx.
	if err := ctx.Err(); err != nil {
		return md, err
	}
	// Process each resource metric and its data points
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resourceAttrs := attributeMapToGeneric(rm.Resource().Attributes())

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)

			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)

				// Extract features based on metric type
				features := p.metricsExtractor.ExtractFeatures(metric, resourceAttrs)

				// Process through isolation forest
				score, isAnomaly, modelName := p.processFeatures(features, resourceAttrs)

				// Add anomaly attributes to metric data points
				if p.config.Mode == "enrich" || p.config.Mode == "both" {
					p.addAnomalyAttributesToMetric(metric, score, isAnomaly, modelName)
				}
			}
		}
	}

	return md, nil
}

// processLogs processes log telemetry
func (p *isolationForestProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	// Honor cancellation/deadline; satisfies unparam + uses ctx.
	if err := ctx.Err(); err != nil {
		return ld, err
	}

	// Process each resource log and its records
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		resourceAttrs := attributeMapToGeneric(rl.Resource().Attributes())

		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)

			// Create a new log record slice for filtered logs
			newLogs := plog.NewLogRecordSlice()

			for k := 0; k < sl.LogRecords().Len(); k++ {
				record := sl.LogRecords().At(k)

				// Extract features from the log record
				features := p.logsExtractor.ExtractFeatures(record, resourceAttrs)

				// Combine log and resource attributes for model selection
				logAttrs := attributeMapToGeneric(record.Attributes())
				allAttrs := mergeAttributes(resourceAttrs, logAttrs)

				// Process through isolation forest
				score, isAnomaly, modelName := p.processFeatures(features, allAttrs)

				// Apply processing mode
				if p.config.Mode == "filter" && !isAnomaly {
					// Skip this log record - don't add to newLogs
					continue
				}

				// Copy log record to new slice
				newRecord := newLogs.AppendEmpty()
				record.CopyTo(newRecord)

				// Add anomaly attributes in enrich or both modes
				if p.config.Mode == "enrich" || p.config.Mode == "both" {
					newRecord.Attributes().PutDouble(p.config.ScoreAttribute, score)
					newRecord.Attributes().PutBool(p.config.ClassificationAttribute, isAnomaly)
					if modelName != "" && modelName != "default" {
						newRecord.Attributes().PutStr("anomaly.model_name", modelName)
					}
				}
			}

			// Replace the original log records with filtered/enriched logs
			sl.LogRecords().RemoveIf(func(_ plog.LogRecord) bool { return true })
			for k := 0; k < newLogs.Len(); k++ {
				newRecord := sl.LogRecords().AppendEmpty()
				newLogs.At(k).CopyTo(newRecord)
			}
		}
	}

	return ld, nil
}

// addAnomalyAttributesToMetric adds anomaly detection results to metric data points
func (p *isolationForestProcessor) addAnomalyAttributesToMetric(metric pmetric.Metric, score float64, isAnomaly bool, modelName string) {
	// Add attributes to different metric types based on their structure
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		gauge := metric.Gauge()
		for i := 0; i < gauge.DataPoints().Len(); i++ {
			dp := gauge.DataPoints().At(i)
			dp.Attributes().PutDouble(p.config.ScoreAttribute, score)
			dp.Attributes().PutBool(p.config.ClassificationAttribute, isAnomaly)
			if modelName != "" && modelName != "default" {
				dp.Attributes().PutStr("anomaly.model_name", modelName)
			}
		}
	case pmetric.MetricTypeSum:
		sum := metric.Sum()
		for i := 0; i < sum.DataPoints().Len(); i++ {
			dp := sum.DataPoints().At(i)
			dp.Attributes().PutDouble(p.config.ScoreAttribute, score)
			dp.Attributes().PutBool(p.config.ClassificationAttribute, isAnomaly)
			if modelName != "" && modelName != "default" {
				dp.Attributes().PutStr("anomaly.model_name", modelName)
			}
		}
	case pmetric.MetricTypeHistogram:
		histogram := metric.Histogram()
		for i := 0; i < histogram.DataPoints().Len(); i++ {
			dp := histogram.DataPoints().At(i)
			dp.Attributes().PutDouble(p.config.ScoreAttribute, score)
			dp.Attributes().PutBool(p.config.ClassificationAttribute, isAnomaly)
			if modelName != "" && modelName != "default" {
				dp.Attributes().PutStr("anomaly.model_name", modelName)
			}
		}
	case pmetric.MetricTypeSummary:
		summary := metric.Summary()
		for i := 0; i < summary.DataPoints().Len(); i++ {
			dp := summary.DataPoints().At(i)
			dp.Attributes().PutDouble(p.config.ScoreAttribute, score)
			dp.Attributes().PutBool(p.config.ClassificationAttribute, isAnomaly)
			if modelName != "" && modelName != "default" {
				dp.Attributes().PutStr("anomaly.model_name", modelName)
			}
		}
	case pmetric.MetricTypeExponentialHistogram:
		exponentialHistogram := metric.ExponentialHistogram()
		for i := 0; i < exponentialHistogram.DataPoints().Len(); i++ {
			dp := exponentialHistogram.DataPoints().At(i)
			dp.Attributes().PutDouble(p.config.ScoreAttribute, score)
			dp.Attributes().PutBool(p.config.ClassificationAttribute, isAnomaly)
			if modelName != "" && modelName != "default" {
				dp.Attributes().PutStr("anomaly.model_name", modelName)
			}
		}
	}
}

// Feature extraction components for different signal types

// TraceFeatureExtractor extracts numerical features from trace spans
type traceFeatureExtractor struct {
	features []string
	logger   *zap.Logger
}

func newTraceFeatureExtractor(features []string, logger *zap.Logger) *traceFeatureExtractor {
	return &traceFeatureExtractor{
		features: features,
		logger:   logger,
	}
}

func (tfe *traceFeatureExtractor) ExtractFeatures(span ptrace.Span, resourceAttrs map[string]any) map[string][]float64 {
	features := make(map[string][]float64)

	for _, featureName := range tfe.features {
		switch featureName {
		case "duration":
			// Extract span duration in milliseconds
			duration := float64(span.EndTimestamp()-span.StartTimestamp()) / 1e6 // Convert nanoseconds to milliseconds
			features["duration"] = []float64{duration}

		case "error":
			// Binary feature indicating error status
			errorValue := 0.0
			if span.Status().Code() == ptrace.StatusCodeError {
				errorValue = 1.0
			}
			features["error"] = []float64{errorValue}

		case "http.status_code":
			// HTTP status code if available
			if statusCode, exists := span.Attributes().Get("http.status_code"); exists {
				if code, err := strconv.ParseFloat(statusCode.AsString(), 64); err == nil {
					features["http.status_code"] = []float64{code}
				}
			}

		case "service.name":
			// Categorical encoding of service name
			if serviceName, exists := resourceAttrs["service.name"]; exists {
				encoded := categoricalEncode(fmt.Sprintf("%v", serviceName))
				features["service.name"] = []float64{encoded}
			}

		case "operation.name":
			// Categorical encoding of operation name
			encoded := categoricalEncode(span.Name())
			features["operation.name"] = []float64{encoded}
		}
	}

	return features
}

// MetricsFeatureExtractor extracts numerical features from metrics
type metricsFeatureExtractor struct {
	features []string
	logger   *zap.Logger

	// Track previous values for rate calculation
	previousValues map[string]float64
	previousTimes  map[string]time.Time
	mutex          sync.Mutex
}

func newMetricsFeatureExtractor(features []string, logger *zap.Logger) *metricsFeatureExtractor {
	return &metricsFeatureExtractor{
		features:       features,
		logger:         logger,
		previousValues: make(map[string]float64),
		previousTimes:  make(map[string]time.Time),
	}
}

func (mfe *metricsFeatureExtractor) ExtractFeatures(metric pmetric.Metric, _ map[string]any) map[string][]float64 {
	features := make(map[string][]float64)

	// Extract primary metric value based on type
	var currentValue float64
	var timestamp time.Time

	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		if metric.Gauge().DataPoints().Len() > 0 {
			dp := metric.Gauge().DataPoints().At(0)
			currentValue = dp.DoubleValue()
			timestamp = time.Unix(0, int64(dp.Timestamp()))
		}
	case pmetric.MetricTypeSum:
		if metric.Sum().DataPoints().Len() > 0 {
			dp := metric.Sum().DataPoints().At(0)
			currentValue = dp.DoubleValue()
			timestamp = time.Unix(0, int64(dp.Timestamp()))
		}
	}

	metricKey := metric.Name()

	for _, featureName := range mfe.features {
		switch featureName {
		case "value":
			features["value"] = []float64{currentValue}

		case "rate_of_change":
			// Calculate rate of change from previous value
			mfe.mutex.Lock()
			if prevValue, exists := mfe.previousValues[metricKey]; exists {
				if prevTime, timeExists := mfe.previousTimes[metricKey]; timeExists {
					timeDiff := timestamp.Sub(prevTime).Seconds()
					if timeDiff > 0 {
						rate := (currentValue - prevValue) / timeDiff
						features["rate_of_change"] = []float64{rate}
					}
				}
			}
			mfe.previousValues[metricKey] = currentValue
			mfe.previousTimes[metricKey] = timestamp
			mfe.mutex.Unlock()
		}
	}

	return features
}

// LogsFeatureExtractor extracts numerical features from log records
type logsFeatureExtractor struct {
	features      []string
	logger        *zap.Logger
	lastTimestamp map[string]time.Time
	mutex         sync.Mutex
}

func newLogsFeatureExtractor(features []string, logger *zap.Logger) *logsFeatureExtractor {
	return &logsFeatureExtractor{
		features:      features,
		logger:        logger,
		lastTimestamp: make(map[string]time.Time),
	}
}

func (lfe *logsFeatureExtractor) ExtractFeatures(record plog.LogRecord, resourceAttrs map[string]any) map[string][]float64 {
	features := make(map[string][]float64)

	for _, featureName := range lfe.features {
		switch featureName {
		case "severity_number":
			// Numeric log severity level
			features["severity_number"] = []float64{float64(record.SeverityNumber())}

		case "timestamp_gap":
			// Time since last log entry from same source
			currentTime := time.Unix(0, int64(record.Timestamp()))

			// Use service name or other identifier as key
			var sourceKey string
			if serviceName, exists := resourceAttrs["service.name"]; exists {
				sourceKey = fmt.Sprintf("%v", serviceName)
			} else {
				sourceKey = "default"
			}

			lfe.mutex.Lock()
			if lastTime, exists := lfe.lastTimestamp[sourceKey]; exists {
				gap := currentTime.Sub(lastTime).Seconds()
				features["timestamp_gap"] = []float64{gap}
			}
			lfe.lastTimestamp[sourceKey] = currentTime
			lfe.mutex.Unlock()

		case "message_length":
			// Length of log message
			messageLength := float64(len(record.Body().AsString()))
			features["message_length"] = []float64{messageLength}
		}
	}

	return features
}

// Utility functions for attribute handling and feature processing

// attributeMapToGeneric converts OpenTelemetry attribute maps to generic map[string]any
func attributeMapToGeneric(attrs pcommon.Map) map[string]any {
	result := make(map[string]any)
	attrs.Range(func(k string, v pcommon.Value) bool {
		switch v.Type() {
		case pcommon.ValueTypeStr:
			result[k] = v.Str()
		case pcommon.ValueTypeInt:
			result[k] = v.Int()
		case pcommon.ValueTypeDouble:
			result[k] = v.Double()
		case pcommon.ValueTypeBool:
			result[k] = v.Bool()
		default:
			result[k] = v.AsString()
		}
		return true
	})
	return result
}

// mergeAttributes combines multiple attribute maps with later maps taking precedence
func mergeAttributes(in ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, m := range in {
		maps.Copy(result, m)
	}
	return result
}

// categoricalEncode converts string values to numerical representation using hash function
func categoricalEncode(value string) float64 {
	h := fnv.New64a()
	h.Write([]byte(value))

	// Convert hash to float64 in range [0, 1]
	hashValue := h.Sum64()
	return float64(hashValue) / float64(math.MaxUint64)
}
