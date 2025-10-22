// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/streamingaggregationprocessor"

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/streamingaggregationprocessor/internal/aggregation"
)

// streamingAggregationProcessor implements the streaming aggregation processor
// Double-buffer design: exactly 2 windows that alternate every windowSize duration
type streamingAggregationProcessor struct {
	logger *zap.Logger
	config *Config

	// Double-buffer window architecture
	windowA      *TimeWindow // First window (time boundaries only)
	windowB      *TimeWindow // Second window (time boundaries only)
	activeWindow *TimeWindow // Points to current writable window
	nextSwapTime time.Time   // When to swap windows
	windowSize   time.Duration

	// Aggregators stored at processor level (persistent across window swaps)
	aggregators   map[string]*Aggregator // Key: series key
	aggregatorsMu sync.RWMutex           // Protect aggregator map

	// Lifecycle management
	startOnce sync.Once
	stopOnce  sync.Once
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Statistics
	metricsReceived  atomic.Int64
	metricsProcessed atomic.Int64
	metricsDropped   atomic.Int64
	metricsFiltered  atomic.Int64 // Metrics filtered out by regex

	// Memory management
	memoryUsage    atomic.Int64
	maxMemoryBytes int64

	// Export management
	nextConsumer consumer.Metrics // Store the next consumer in the pipeline

	// Metric filtering
	metricRegexes []*regexp.Regexp // Compiled regexes for metric name matching

	// Mutex for window operations
	mu sync.RWMutex
}

// newStreamingAggregationProcessor creates a new streaming aggregation processor
func newStreamingAggregationProcessor(logger *zap.Logger, config *Config) (*streamingAggregationProcessor, error) {
	// Apply defaults to config
	config.applyDefaults()

	// Compile metric regex patterns
	var metricRegexes []*regexp.Regexp
	if len(config.Metrics) > 0 {
		metricRegexes = make([]*regexp.Regexp, len(config.Metrics))
		for i, metricConfig := range config.Metrics {
			regex, err := regexp.Compile(metricConfig.Match)
			if err != nil {
				return nil, fmt.Errorf("failed to compile metrics[%d].match pattern: %w", i, err)
			}
			metricRegexes[i] = regex
		}
		logger.Info("Metric name filtering enabled",
			zap.Int("pattern_count", len(config.Metrics)),
			zap.Strings("patterns", func() []string {
				patterns := make([]string, len(config.Metrics))
				for i, m := range config.Metrics {
					patterns[i] = m.Match
				}
				return patterns
			}()),
		)
	} else {
		logger.Info("No metric patterns configured - processing all metrics (backward compatibility)")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize double-buffer windows with proper time alignment
	now := time.Now()
	alignedStart := now.Truncate(config.WindowSize)

	p := &streamingAggregationProcessor{
		logger:         logger,
		config:         config,
		ctx:            ctx,
		cancel:         cancel,
		windowSize:     config.WindowSize,
		maxMemoryBytes: int64(config.MaxMemoryMB * 1024 * 1024),
		aggregators:    make(map[string]*Aggregator),
		metricRegexes:  metricRegexes,

		// Initialize double-buffer windows
		windowA:      NewTimeWindow(alignedStart, alignedStart.Add(config.WindowSize)),
		windowB:      NewTimeWindow(alignedStart.Add(config.WindowSize), alignedStart.Add(2*config.WindowSize)),
		nextSwapTime: alignedStart.Add(config.WindowSize),
	}

	// Start with window A as the active window
	p.activeWindow = p.windowA

	return p, nil
}

// getOrCreateAggregator gets or creates an aggregator for a series at processor level
func (p *streamingAggregationProcessor) getOrCreateAggregator(seriesKey string, metricType pmetric.MetricType) *Aggregator {
	p.aggregatorsMu.RLock()
	if agg, exists := p.aggregators[seriesKey]; exists {
		p.aggregatorsMu.RUnlock()
		return agg
	}
	p.aggregatorsMu.RUnlock()

	// Create new aggregator under write lock
	p.aggregatorsMu.Lock()
	defer p.aggregatorsMu.Unlock()

	// Double-check in case another goroutine created it
	if agg, exists := p.aggregators[seriesKey]; exists {
		return agg
	}

	// Create new aggregator
	agg := NewAggregator(metricType)
	p.aggregators[seriesKey] = agg

	// Update memory usage estimate
	p.memoryUsage.Add(agg.EstimateMemoryUsage())

	return agg
}

// resetAggregatorsForNewWindow resets window-specific state in all aggregators
func (p *streamingAggregationProcessor) resetAggregatorsForNewWindow() {
	p.aggregatorsMu.RLock()
	defer p.aggregatorsMu.RUnlock()

	for _, agg := range p.aggregators {
		agg.ResetForNewWindow()
	}
}

// hasActiveWindowData checks if there is data to export from current aggregators
func (p *streamingAggregationProcessor) hasActiveWindowData() bool {
	p.aggregatorsMu.RLock()
	defer p.aggregatorsMu.RUnlock()

	return len(p.aggregators) > 0
}

// Start starts the processor
func (p *streamingAggregationProcessor) Start(ctx context.Context, host component.Host) error {
	p.startOnce.Do(func() {
		p.logger.Info("Starting streaming aggregation processor (double-buffer mode)",
			zap.Duration("window_size", p.config.WindowSize),
		)

		// Start window swap checker (checks every second for swap time)
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.logger.Debug("Window swap checker started")
			p.runWindowSwapChecker()
		}()

		// Start memory monitor
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.monitorMemory()
		}()

		// Start statistics reporter
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.reportStatistics()
		}()
	})

	return nil
}

// Shutdown shuts down the processor
func (p *streamingAggregationProcessor) Shutdown(ctx context.Context) error {
	var shutdownErr error

	p.stopOnce.Do(func() {
		p.logger.Info("Shutting down streaming aggregation processor")

		// Signal all goroutines to stop
		p.cancel()

		// Wait for all goroutines to finish with timeout
		done := make(chan struct{})
		go func() {
			p.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			p.logger.Info("All goroutines stopped successfully")
		case <-ctx.Done():
			shutdownErr = fmt.Errorf("shutdown timeout exceeded")
			p.logger.Error("Shutdown timeout exceeded", zap.Error(shutdownErr))
		}

		// Export any remaining aggregated metrics
		p.forceExportActiveWindow()
	})

	return shutdownErr
}

// runWindowSwapChecker checks every second for window swap timing
func (p *streamingAggregationProcessor) runWindowSwapChecker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.checkWindowSwap()
		case <-p.ctx.Done():
			p.logger.Debug("Window swap checker stopping due to context cancellation")
			return
		}
	}
}

// checkWindowSwap checks if it's time to swap windows
func (p *streamingAggregationProcessor) checkWindowSwap() {
	now := time.Now()
	if now.After(p.nextSwapTime) || now.Equal(p.nextSwapTime) {
		p.swapWindows()
	}
}

// swapWindows swaps the active window and exports the previous one
func (p *streamingAggregationProcessor) swapWindows() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info("Swapping windows",
		zap.Time("current_window_start", p.activeWindow.start),
		zap.Time("current_window_end", p.activeWindow.end),
		zap.Time("swap_time", time.Now()),
	)

	// Export current active window if it has data
	if p.hasActiveWindowData() {
		p.logger.Debug("Exporting active window before swap")
		p.exportActiveWindow()
	}

	// Reset window-specific state in all aggregators for new window
	p.resetAggregatorsForNewWindow()

	// Swap to the other window
	if p.activeWindow == p.windowA {
		p.activeWindow = p.windowB
	} else {
		p.activeWindow = p.windowA
	}

	// Set new time boundaries for the now-active window
	now := time.Now()
	alignedStart := now.Truncate(p.windowSize)
	// If we're very close to the boundary, move to next boundary
	if alignedStart.Add(p.windowSize / 2).Before(now) {
		alignedStart = alignedStart.Add(p.windowSize)
	}

	p.activeWindow.start = alignedStart
	p.activeWindow.end = alignedStart.Add(p.windowSize)
	p.nextSwapTime = p.activeWindow.end

	p.logger.Debug("Window swap completed",
		zap.Time("new_window_start", p.activeWindow.start),
		zap.Time("new_window_end", p.activeWindow.end),
		zap.Time("next_swap_time", p.nextSwapTime),
	)
}

// ProcessMetrics processes incoming metrics
func (p *streamingAggregationProcessor) ProcessMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	p.metricsReceived.Add(int64(md.DataPointCount()))

	// Process metrics directly (no routing needed - upstream handles sharding)
	p.processMetrics(md)

	// Always return empty metrics - aggregated metrics are exported at window boundaries
	// through the nextConsumer
	return pmetric.NewMetrics(), nil
}

// processMetrics processes a batch of metrics
func (p *streamingAggregationProcessor) processMetrics(md pmetric.Metrics) {
	rms := md.ResourceMetrics()

	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resource := rm.Resource()

		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			scope := sm.Scope()

			metrics := sm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)

				// Apply metric name filtering - default behavior is to process all metrics
				// when no patterns are configured (backward compatibility)
				matched := true // Default to processing metrics
				if len(p.metricRegexes) > 0 {
					// If patterns are configured, check if metric matches any pattern
					matched = false
					for _, regex := range p.metricRegexes {
						if regex.MatchString(metric.Name()) {
							matched = true
							break
						}
					}
				}
				// If patterns are configured AND metric doesn't match any pattern, filter it out
				if !matched {
					p.metricsFiltered.Add(1)
					p.logger.Debug("Metric filtered out - no matching pattern",
						zap.String("metric", metric.Name()),
						zap.Int("configured_patterns", len(p.metricRegexes)),
					)
					continue
				}

				// Process each metric with automatic type-based aggregation
				if err := p.aggregateMetric(metric, resource, scope); err != nil {
					p.metricsDropped.Add(1)
					p.logger.Debug("Failed to aggregate metric",
						zap.String("metric", metric.Name()),
						zap.Error(err),
					)
				} else {
					p.metricsProcessed.Add(1)
				}
			}
		}
	}
}

// aggregateMetric aggregates a single metric based on its type with enhanced label handling
func (p *streamingAggregationProcessor) aggregateMetric(
	metric pmetric.Metric,
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
) error {
	// Find metric configuration for this metric
	metricConfig := p.findMetricConfig(metric.Name())

	// For all metric types, use the enhanced aggregation with label filtering
	// but integrate with the existing aggregator system
	return p.aggregateMetricWithEnhancedLabels(metric, metricConfig)
}

// aggregateMetricWithEnhancedLabels provides label filtering and integrates with existing aggregation
func (p *streamingAggregationProcessor) aggregateMetricWithEnhancedLabels(
	metric pmetric.Metric,
	metricConfig *MetricConfig,
) error {
	// Always use aggregateMetricWithGrouping for proper handling of custom aggregation strategies
	// For drop_all, it will create a single group with no labels
	if metricConfig != nil {
		return p.aggregateMetricWithGrouping(metric, metricConfig)
	}

	// Default behavior: use single series key for all data points (drop all labels)
	seriesKey := metric.Name() + "|"
	aggregator := p.getOrCreateAggregator(seriesKey, metric.Type())

	// Aggregate based on metric type using internal aggregation package
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		config := aggregation.CounterAggregationConfig{
			StaleDataThreshold: p.config.StaleDataThreshold,
			Logger:             p.logger,
		}
		return aggregation.AggregateSum(metric.Sum(), aggregator, config, p.forceExportActiveWindow)
	case pmetric.MetricTypeGauge:
		// Handle custom gauge aggregation strategies
		if metricConfig != nil && metricConfig.AggregateType != Last {
			p.logger.Debug("Using custom gauge aggregation",
				zap.String("metric", metric.Name()),
				zap.String("aggregate_type", string(metricConfig.AggregateType)),
				zap.Int("data_points", metric.Gauge().DataPoints().Len()),
			)
			return p.aggregateGaugeWithCustomStrategy(metric.Gauge(), aggregator, metricConfig)
		}
		// Default gauge aggregation (last value)
		p.logger.Debug("Using default gauge aggregation (last)",
			zap.String("metric", metric.Name()),
			zap.Bool("has_config", metricConfig != nil),
		)
		config := aggregation.GaugeAggregationConfig{
			Logger: p.logger,
		}
		return aggregation.AggregateGauge(metric.Gauge(), aggregator, config)
	case pmetric.MetricTypeHistogram:
		config := aggregation.HistogramAggregationConfig{
			StaleDataThreshold: p.config.StaleDataThreshold,
			Logger:             p.logger,
		}
		return aggregation.AggregateHistogram(metric.Histogram(), aggregator, config)
	case pmetric.MetricTypeExponentialHistogram:
		return aggregation.AggregateExponentialHistogram(metric, metric.ExponentialHistogram(), aggregator)
	case pmetric.MetricTypeSummary:
		return aggregation.AggregateSummary(metric.Summary(), aggregator)
	default:
		return fmt.Errorf("unsupported metric type: %v", metric.Type())
	}
}

// exportActiveWindow exports aggregated metrics from processor-level aggregators
func (p *streamingAggregationProcessor) exportActiveWindow() {
	metrics := p.exportAggregatedMetrics(p.activeWindow.start, p.activeWindow.end)

	if metrics.DataPointCount() > 0 && p.nextConsumer != nil {
		ctx := context.Background()
		if err := p.nextConsumer.ConsumeMetrics(ctx, metrics); err != nil {
			p.logger.Error("Failed to export aggregated metrics",
				zap.Error(err),
				zap.Time("window_start", p.activeWindow.start),
				zap.Time("window_end", p.activeWindow.end),
				zap.Int("data_points", metrics.DataPointCount()),
			)
		} else {
			p.logger.Info("Successfully exported aggregated window",
				zap.Time("window_start", p.activeWindow.start),
				zap.Time("window_end", p.activeWindow.end),
				zap.Int("data_points", metrics.DataPointCount()),
			)
		}
	}
}

// exportAggregatedMetrics creates metrics from processor-level aggregators
func (p *streamingAggregationProcessor) exportAggregatedMetrics(windowStart, windowEnd time.Time) pmetric.Metrics {
	md := pmetric.NewMetrics()

	p.aggregatorsMu.RLock()
	defer p.aggregatorsMu.RUnlock()

	if len(p.aggregators) == 0 {
		return md
	}

	// Create single resource and scope for all metrics
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("streamingaggregation")

	// Export each aggregator
	for seriesKey, agg := range p.aggregators {
		// Parse series key to extract metric name and labels
		metricName, labels := parseSeriesKey(seriesKey)

		// Create metric with enhanced metadata handling using aggregateutil
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(metricName)

		// Export based on metric type using our proven logic first
		agg.ExportTo(metric, windowStart, windowEnd, labels)

		// Then apply aggregateutil metadata enhancements (preserves the type setup)
		p.applyMetricMetadataEnhancements(metric, agg.metricType)
	}

	return md
}

// applyMetricMetadataEnhancements applies metadata enhancements using aggregateutil without changing type
func (p *streamingAggregationProcessor) applyMetricMetadataEnhancements(metric pmetric.Metric, metricType pmetric.MetricType) {
	// Only apply metadata enhancements - description and unit
	// The metric type is already set correctly by ExportTo
	metric.SetDescription("Aggregated metric from streaming aggregation processor")

	// Set appropriate unit based on metric type
	switch metricType {
	case pmetric.MetricTypeGauge:
		// Keep original unit or set a sensible default
		if metric.Unit() == "" {
			metric.SetUnit("1") // Dimensionless
		}
	case pmetric.MetricTypeSum:
		// For counters, often rate-based
		if metric.Unit() == "" {
			metric.SetUnit("1") // Default to dimensionless
		}
	case pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram:
		// Histograms often measure time, size, etc.
		if metric.Unit() == "" {
			// Only set unit if the metric name doesn't already indicate units
			metricName := metric.Name()
			if !containsTimeUnit(metricName) && !containsSizeUnit(metricName) {
				metric.SetUnit("1") // Dimensionless by default to avoid conflicts
			}
		}
	}
}

// containsTimeUnit checks if metric name already contains time unit indicators
func containsTimeUnit(name string) bool {
	timeUnits := []string{"_ms", "_milliseconds", "_s", "_seconds", "_us", "_microseconds", "_ns", "_nanoseconds"}
	for _, unit := range timeUnits {
		if strings.Contains(name, unit) {
			return true
		}
	}
	return false
}

// containsSizeUnit checks if metric name already contains size unit indicators
func containsSizeUnit(name string) bool {
	sizeUnits := []string{"_bytes", "_kb", "_mb", "_gb", "_bits"}
	for _, unit := range sizeUnits {
		if strings.Contains(name, unit) {
			return true
		}
	}
	return false
}

// forceExportActiveWindow forces export of current window (used during shutdown)
func (p *streamingAggregationProcessor) forceExportActiveWindow() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.hasActiveWindowData() {
		p.logger.Info("Force exporting active window during shutdown",
			zap.Time("window_start", p.activeWindow.start),
			zap.Time("window_end", p.activeWindow.end),
		)
		p.exportActiveWindow()
	}
}

// GetAggregatedMetrics returns the current aggregated metrics (for testing)
func (p *streamingAggregationProcessor) GetAggregatedMetrics() pmetric.Metrics {
	return p.exportAggregatedMetrics(p.activeWindow.start, p.activeWindow.end)
}

// monitorMemory monitors memory usage and triggers eviction if needed
func (p *streamingAggregationProcessor) monitorMemory() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			usage := p.memoryUsage.Load()

			if usage > p.maxMemoryBytes {
				p.logger.Warn("Memory limit exceeded, triggering eviction",
					zap.Int64("usage_bytes", usage),
					zap.Int64("limit_bytes", p.maxMemoryBytes),
				)
				p.performEviction()
			} else if float64(usage) > float64(p.maxMemoryBytes)*0.8 {
				p.logger.Debug("Memory usage high",
					zap.Int64("usage_bytes", usage),
					zap.Int64("limit_bytes", p.maxMemoryBytes),
					zap.Float64("usage_percent", float64(usage)/float64(p.maxMemoryBytes)*100),
				)
			}

		case <-p.ctx.Done():
			return
		}
	}
}

// performEviction performs memory eviction when under pressure
func (p *streamingAggregationProcessor) performEviction() {
	p.aggregatorsMu.Lock()
	defer p.aggregatorsMu.Unlock()

	if len(p.aggregators) == 0 {
		return
	}

	// Evict 20% of least recently used aggregators
	numToEvict := len(p.aggregators) / 5
	if numToEvict == 0 && len(p.aggregators) > 0 {
		numToEvict = 1 // Evict at least one
	}

	// Simple eviction: remove the first N aggregators
	// In a production system, you'd want proper LRU tracking
	var bytesFreed int64
	var keysToDelete []string

	count := 0
	for key, agg := range p.aggregators {
		if count >= numToEvict {
			break
		}
		bytesFreed += agg.EstimateMemoryUsage()
		keysToDelete = append(keysToDelete, key)
		count++
	}

	// Delete the selected aggregators
	for _, key := range keysToDelete {
		delete(p.aggregators, key)
	}

	p.memoryUsage.Add(-bytesFreed)

	p.logger.Debug("Evicted aggregators",
		zap.Int("evicted_count", len(keysToDelete)),
		zap.Int64("bytes_freed", bytesFreed),
	)
}

// reportStatistics reports processor statistics
func (p *streamingAggregationProcessor) reportStatistics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			received := p.metricsReceived.Load()
			processed := p.metricsProcessed.Load()
			dropped := p.metricsDropped.Load()
			filtered := p.metricsFiltered.Load()

			p.logger.Info("Processor statistics",
				zap.Int64("metrics_received", received),
				zap.Int64("metrics_processed", processed),
				zap.Int64("metrics_dropped", dropped),
				zap.Int64("metrics_filtered", filtered),
				zap.Float64("drop_rate", float64(dropped)/float64(received)*100),
				zap.Float64("filter_rate", float64(filtered)/float64(received)*100),
				zap.Int64("memory_bytes", p.memoryUsage.Load()),
			)

		case <-p.ctx.Done():
			return
		}
	}
}

// Label filtering and series key generation utilities

// findMetricConfig finds the MetricConfig for a given metric name
func (p *streamingAggregationProcessor) findMetricConfig(metricName string) *MetricConfig {
	for i, metricConfig := range p.config.Metrics {
		if i < len(p.metricRegexes) && p.metricRegexes[i].MatchString(metricName) {
			p.logger.Debug("Found metric config match",
				zap.String("metric", metricName),
				zap.String("pattern", metricConfig.Match),
				zap.String("aggregate_type", string(metricConfig.AggregateType)),
				zap.String("label_type", string(metricConfig.Labels.Type)),
			)
			return &metricConfig
		}
	}
	p.logger.Debug("No metric config found",
		zap.String("metric", metricName),
		zap.Int("total_configs", len(p.config.Metrics)),
		zap.Int("total_regexes", len(p.metricRegexes)),
	)
	return nil // No matching config found
}

// getLabelFilteredAttributes applies label filtering based on VictoriaMetrics pattern
// Adapted from VictoriaMetrics getInputOutputLabels function
func getLabelFilteredAttributes(attrs pcommon.Map, labelConfig LabelConfig) pcommon.Map {
	filteredAttrs := pcommon.NewMap()

	switch labelConfig.Type {
	case Keep: // Equivalent to VictoriaMetrics "by" - keep only specified labels
		attrs.Range(func(k string, v pcommon.Value) bool {
			if slices.Contains(labelConfig.Names, k) {
				v.CopyTo(filteredAttrs.PutEmpty(k))
			}
			return true
		})
	case Remove: // Equivalent to VictoriaMetrics "without" - remove specified labels
		attrs.Range(func(k string, v pcommon.Value) bool {
			if !slices.Contains(labelConfig.Names, k) {
				v.CopyTo(filteredAttrs.PutEmpty(k))
			}
			return true
		})
	case DropAll: // Default behavior - return empty map
		// filteredAttrs remains empty
	}

	return filteredAttrs
}

// generateSeriesKey creates a series key with filtered labels
func (p *streamingAggregationProcessor) generateSeriesKey(metricName string, attrs pcommon.Map, labelConfig LabelConfig) string {
	if labelConfig.Type == DropAll {
		return metricName + "|" // Current behavior
	}

	filteredAttrs := getLabelFilteredAttributes(attrs, labelConfig)
	if filteredAttrs.Len() == 0 {
		return metricName + "|" // No labels to keep
	}

	// Create deterministic label string
	var labelPairs []string
	filteredAttrs.Range(func(k string, v pcommon.Value) bool {
		labelPairs = append(labelPairs, k+"="+v.AsString())
		return true
	})
	sort.Strings(labelPairs) // Ensure deterministic order

	return metricName + "|" + strings.Join(labelPairs, ",")
}

// getFilterAttrKeys converts LabelConfig to filter keys for aggregateutil.FilterAttrs
func getFilterAttrKeys(labelConfig LabelConfig) []string {
	switch labelConfig.Type {
	case Keep:
		return labelConfig.Names // Keep only these
	case Remove:
		return []string{} // Keep all except those in Names (handled differently)
	case DropAll:
		return []string{} // Remove all attributes
	default:
		return []string{} // Default to remove all
	}
}

// aggregateGaugeWithStrategy aggregates gauge metrics using aggregateutil
func (p *streamingAggregationProcessor) aggregateGaugeWithStrategy(
	metric pmetric.Metric,
	metricConfig *MetricConfig,
) error {
	if metric.Type() != pmetric.MetricTypeGauge {
		return fmt.Errorf("aggregateGaugeWithStrategy can only be used with gauge metrics")
	}

	// Apply label filtering using aggregateutil.FilterAttrs
	if metricConfig.Labels.Type == Remove {
		// For "remove" type, we need custom filtering since aggregateutil.FilterAttrs
		// only supports keeping specified labels
		p.filterRemoveLabels(metric, metricConfig.Labels.Names)
	} else {
		// For "keep" and "drop_all" types, use aggregateutil.FilterAttrs
		filterKeys := getFilterAttrKeys(metricConfig.Labels)
		aggregateutil.FilterAttrs(metric, filterKeys)
	}

	// Group data points by the filtered attributes
	aggGroups := &aggregateutil.AggGroups{}
	aggregateutil.GroupDataPoints(metric, aggGroups)

	// Create output metric
	outputMetric := pmetric.NewMetric()
	aggregateutil.CopyMetricDetails(metric, outputMetric)

	// Apply aggregation strategy
	if metricConfig.AggregateType == Last {
		// Special handling for "last" - we need to use a custom approach
		// since aggregateutil doesn't support "last" aggregation
		p.applyLastAggregation(metric, outputMetric)
	} else {
		// Use existing aggregation types from aggregateutil
		aggregateutil.MergeDataPoints(outputMetric, metricConfig.getAggregateUtilType(), *aggGroups)
	}

	// Store the aggregated metric (integrate with existing aggregator system)
	return p.storeAggregatedGaugeMetric(outputMetric, metricConfig)
}

// filterRemoveLabels removes specified labels from all data points
func (p *streamingAggregationProcessor) filterRemoveLabels(metric pmetric.Metric, labelsToRemove []string) {
	aggregateutil.RangeDataPointAttributes(metric, func(attrs pcommon.Map) bool {
		for _, labelName := range labelsToRemove {
			attrs.Remove(labelName)
		}
		return true
	})
}

// applyLabelFiltering applies label filtering to all data points in a metric
func (p *streamingAggregationProcessor) applyLabelFiltering(metric pmetric.Metric, labelConfig LabelConfig) {
	switch labelConfig.Type {
	case Keep:
		// Keep only specified labels
		aggregateutil.FilterAttrs(metric, labelConfig.Names)
	case Remove:
		// Remove specified labels
		aggregateutil.RangeDataPointAttributes(metric, func(attrs pcommon.Map) bool {
			for _, labelName := range labelConfig.Names {
				attrs.Remove(labelName)
			}
			return true
		})
	case DropAll:
		// Remove all labels
		aggregateutil.FilterAttrs(metric, []string{})
	}
}

// generateSeriesKeyFromMetric generates a series key from a metric after label filtering
func (p *streamingAggregationProcessor) generateSeriesKeyFromMetric(metric pmetric.Metric, metricConfig *MetricConfig) string {
	if metricConfig == nil || metricConfig.Labels.Type == DropAll {
		return metric.Name() + "|" // Default behavior
	}

	// Since we already applied label filtering, we can use any data point to get the filtered attributes
	var attrs pcommon.Map
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		if metric.Gauge().DataPoints().Len() > 0 {
			attrs = metric.Gauge().DataPoints().At(0).Attributes()
		}
	case pmetric.MetricTypeSum:
		if metric.Sum().DataPoints().Len() > 0 {
			attrs = metric.Sum().DataPoints().At(0).Attributes()
		}
	case pmetric.MetricTypeHistogram:
		if metric.Histogram().DataPoints().Len() > 0 {
			attrs = metric.Histogram().DataPoints().At(0).Attributes()
		}
	case pmetric.MetricTypeExponentialHistogram:
		if metric.ExponentialHistogram().DataPoints().Len() > 0 {
			attrs = metric.ExponentialHistogram().DataPoints().At(0).Attributes()
		}
	case pmetric.MetricTypeSummary:
		if metric.Summary().DataPoints().Len() > 0 {
			attrs = metric.Summary().DataPoints().At(0).Attributes()
		}
	}

	if attrs.Len() == 0 {
		return metric.Name() + "|"
	}

	// Create deterministic label string from filtered attributes
	var labelPairs []string
	attrs.Range(func(k string, v pcommon.Value) bool {
		labelPairs = append(labelPairs, k+"="+v.AsString())
		return true
	})
	sort.Strings(labelPairs)

	return metric.Name() + "|" + strings.Join(labelPairs, ",")
}

// applyLastAggregation implements "last" aggregation strategy for gauge metrics
func (p *streamingAggregationProcessor) applyLastAggregation(inputMetric, outputMetric pmetric.Metric) {
	if inputMetric.Type() != pmetric.MetricTypeGauge {
		return
	}

	inputDps := inputMetric.Gauge().DataPoints()
	outputDps := outputMetric.Gauge().DataPoints()

	if inputDps.Len() == 0 {
		return
	}

	// Find the data point with the latest timestamp
	latestIdx := 0
	latestTimestamp := inputDps.At(0).Timestamp()

	for i := 1; i < inputDps.Len(); i++ {
		if inputDps.At(i).Timestamp() > latestTimestamp {
			latestIdx = i
			latestTimestamp = inputDps.At(i).Timestamp()
		}
	}

	// Copy the latest data point to output
	dp := outputDps.AppendEmpty()
	inputDps.At(latestIdx).CopyTo(dp)
}

// storeAggregatedGaugeMetric integrates with existing aggregator system
func (p *streamingAggregationProcessor) storeAggregatedGaugeMetric(metric pmetric.Metric, metricConfig *MetricConfig) error {
	// For now, we'll integrate this with the existing system by processing the aggregated metric
	// through the normal flow but with the enhanced series key

	if metric.Gauge().DataPoints().Len() == 0 {
		return nil // No data points to store
	}

	// Get the first data point to extract attributes for series key generation
	dp := metric.Gauge().DataPoints().At(0)
	seriesKey := p.generateSeriesKey(metric.Name(), dp.Attributes(), metricConfig.Labels)

	// Get or create aggregator with enhanced series key
	aggregator := p.getOrCreateAggregator(seriesKey, metric.Type())

	// Store the aggregated value using our enhanced aggregator
	// For gauge metrics with custom aggregation, we store the final aggregated value
	aggregator.UpdateLast(extractDoubleValue(dp), dp.Timestamp())

	return nil
}

// aggregateGaugeWithCustomStrategy implements custom aggregation strategies for gauge metrics
func (p *streamingAggregationProcessor) aggregateGaugeWithCustomStrategy(
	gauge pmetric.Gauge,
	aggregator *Aggregator,
	metricConfig *MetricConfig,
) error {
	dps := gauge.DataPoints()
	if dps.Len() == 0 {
		return nil
	}

	// Collect all values for custom aggregation
	var values []float64
	var latestTimestamp pcommon.Timestamp

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		// Get the value based on the data point type
		var value float64
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		default:
			continue // Skip if neither int nor double
		}

		values = append(values, value)

		// Keep track of the latest timestamp
		if dp.Timestamp() > latestTimestamp {
			latestTimestamp = dp.Timestamp()
		}
	}

	if len(values) == 0 {
		return nil
	}

	// Apply the aggregation strategy
	var aggregatedValue float64
	switch metricConfig.AggregateType {
	case Sum:
		for _, v := range values {
			aggregatedValue += v
		}
	case Average:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		aggregatedValue = sum / float64(len(values))
	case Max:
		aggregatedValue = values[0]
		for _, v := range values[1:] {
			if v > aggregatedValue {
				aggregatedValue = v
			}
		}
	case Min:
		aggregatedValue = values[0]
		for _, v := range values[1:] {
			if v < aggregatedValue {
				aggregatedValue = v
			}
		}
	case Last:
		// Find the value with the latest timestamp (already handled above)
		aggregatedValue = values[len(values)-1] // Use last value as fallback
	default:
		// Default to last value
		aggregatedValue = values[len(values)-1]
	}

	// Update the aggregator with the computed value
	aggregator.UpdateLast(aggregatedValue, latestTimestamp)

	return nil
}

// aggregateMetricWithGrouping groups data points by filtered labels before aggregating
func (p *streamingAggregationProcessor) aggregateMetricWithGrouping(
	metric pmetric.Metric,
	metricConfig *MetricConfig,
) error {

	// Group data points by their filtered labels
	groups := make(map[string][]dataPointInfo)

	// Process based on metric type
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		p.groupGaugeDataPoints(metric.Gauge(), metricConfig, groups, metric.Name())
	case pmetric.MetricTypeSum:
		p.groupSumDataPoints(metric.Sum(), metricConfig, groups, metric.Name())
	case pmetric.MetricTypeHistogram:
		p.groupHistogramDataPoints(metric.Histogram(), metricConfig, groups, metric.Name())
	case pmetric.MetricTypeExponentialHistogram:
		p.groupExponentialHistogramDataPoints(metric.ExponentialHistogram(), metricConfig, groups, metric.Name())
	default:
		// For other types, fall back to default behavior
		return p.aggregateMetricDefault(metric, metricConfig)
	}

	// Process each group separately
	for seriesKey, dataPoints := range groups {
		err := p.processDataPointGroup(metric, seriesKey, dataPoints, metricConfig)
		if err != nil {
			p.logger.Error("Failed to process data point group",
				zap.String("series_key", seriesKey),
				zap.Error(err),
			)
			return err
		}
	}

	return nil
}

// dataPointInfo holds information about a data point
type dataPointInfo struct {
	value      float64
	timestamp  pcommon.Timestamp
	attributes pcommon.Map
}

// groupGaugeDataPoints groups gauge data points by filtered labels
func (p *streamingAggregationProcessor) groupGaugeDataPoints(
	gauge pmetric.Gauge,
	metricConfig *MetricConfig,
	groups map[string][]dataPointInfo,
	metricName string,
) {
	dps := gauge.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		// Apply label filtering to get the filtered attributes
		filteredAttrs := getLabelFilteredAttributes(dp.Attributes(), metricConfig.Labels)

		// Generate series key from filtered attributes
		seriesKey := p.generateSeriesKeyFromAttributes(metricName, filteredAttrs)

		// Extract value
		var value float64
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		default:
			continue
		}

		// Add to group
		groups[seriesKey] = append(groups[seriesKey], dataPointInfo{
			value:      value,
			timestamp:  dp.Timestamp(),
			attributes: filteredAttrs,
		})
	}
}

// groupSumDataPoints groups sum data points by filtered labels
func (p *streamingAggregationProcessor) groupSumDataPoints(
	sum pmetric.Sum,
	metricConfig *MetricConfig,
	groups map[string][]dataPointInfo,
	metricName string,
) {
	dps := sum.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		// Apply label filtering to get the filtered attributes
		filteredAttrs := getLabelFilteredAttributes(dp.Attributes(), metricConfig.Labels)

		// Generate series key from filtered attributes
		seriesKey := p.generateSeriesKeyFromAttributes(metricName, filteredAttrs)

		// Extract value
		var value float64
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		default:
			continue
		}

		// Add to group
		groups[seriesKey] = append(groups[seriesKey], dataPointInfo{
			value:      value,
			timestamp:  dp.Timestamp(),
			attributes: filteredAttrs,
		})
	}
}

// groupHistogramDataPoints groups histogram data points by filtered labels
func (p *streamingAggregationProcessor) groupHistogramDataPoints(
	histogram pmetric.Histogram,
	metricConfig *MetricConfig,
	groups map[string][]dataPointInfo,
	metricName string,
) {
	dps := histogram.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		// Apply label filtering to get the filtered attributes
		filteredAttrs := getLabelFilteredAttributes(dp.Attributes(), metricConfig.Labels)

		// Generate series key from filtered attributes
		seriesKey := p.generateSeriesKeyFromAttributes(metricName, filteredAttrs)

		// For histograms, use the sum as the placeholder value for grouping
		// The actual histogram processing will happen later in processHistogramMetricWithStrategy
		value := dp.Sum()

		// Add to group
		groups[seriesKey] = append(groups[seriesKey], dataPointInfo{
			value:      value,
			timestamp:  dp.Timestamp(),
			attributes: filteredAttrs,
		})
	}
}

// groupExponentialHistogramDataPoints groups exponential histogram data points by filtered labels
func (p *streamingAggregationProcessor) groupExponentialHistogramDataPoints(
	exponentialHistogram pmetric.ExponentialHistogram,
	metricConfig *MetricConfig,
	groups map[string][]dataPointInfo,
	metricName string,
) {
	dps := exponentialHistogram.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		// Apply label filtering to get the filtered attributes
		filteredAttrs := getLabelFilteredAttributes(dp.Attributes(), metricConfig.Labels)

		// Generate series key from filtered attributes
		seriesKey := p.generateSeriesKeyFromAttributes(metricName, filteredAttrs)

		// For exponential histograms, use the sum as the placeholder value for grouping
		// The actual exponential histogram processing will happen later
		value := dp.Sum()

		// Add to group
		groups[seriesKey] = append(groups[seriesKey], dataPointInfo{
			value:      value,
			timestamp:  dp.Timestamp(),
			attributes: filteredAttrs,
		})
	}
}

// generateSeriesKeyFromAttributes generates a series key from filtered attributes
func (p *streamingAggregationProcessor) generateSeriesKeyFromAttributes(metricName string, attrs pcommon.Map) string {
	if attrs.Len() == 0 {
		return metricName + "|"
	}

	// Create deterministic label string from attributes
	var labelPairs []string
	attrs.Range(func(k string, v pcommon.Value) bool {
		labelPairs = append(labelPairs, k+"="+v.AsString())
		return true
	})
	sort.Strings(labelPairs)

	return metricName + "|" + strings.Join(labelPairs, ",")
}

// countTotalDataPoints counts total data points across all groups
func (p *streamingAggregationProcessor) countTotalDataPoints(groups map[string][]dataPointInfo) int {
	total := 0
	for _, dataPoints := range groups {
		total += len(dataPoints)
	}
	return total
}

// processDataPointGroup processes a single group of data points
func (p *streamingAggregationProcessor) processDataPointGroup(
	originalMetric pmetric.Metric,
	seriesKey string,
	dataPoints []dataPointInfo,
	metricConfig *MetricConfig,
) error {
	if len(dataPoints) == 0 {
		return nil
	}

	// Get or create aggregator for this series key
	aggregator := p.getOrCreateAggregator(seriesKey, originalMetric.Type())

	// Handle gauge metrics with custom aggregation (including "last" for proper timestamp handling)
	if originalMetric.Type() == pmetric.MetricTypeGauge {
		return p.processGaugeGroupWithStrategy(dataPoints, aggregator, metricConfig)
	}

	// Handle histogram metrics with custom aggregation (sum and quantile strategies)
	if originalMetric.Type() == pmetric.MetricTypeHistogram && metricConfig != nil {
		return p.processHistogramMetricWithStrategy(originalMetric, aggregator, metricConfig, seriesKey)
	}

	// Handle exponential histogram metrics with custom aggregation (sum and quantile strategies)
	if originalMetric.Type() == pmetric.MetricTypeExponentialHistogram && metricConfig != nil {
		// Use exponential histogram aggregator for all cases (same as classic histograms)
		// For quantiles, we'll aggregate first, then calculate quantile, then store as gauge
		return p.processExponentialHistogramMetricWithStrategy(originalMetric, aggregator, metricConfig, seriesKey)
	}

	// Handle sum metrics with custom aggregation (separate logic for counters vs up-down counters)
	if originalMetric.Type() == pmetric.MetricTypeSum && metricConfig != nil {
		if originalMetric.Sum().IsMonotonic() {
			// Monotonic counter (http_requests_total) - cumulative, only increases
			// Exclude "Last" for counters as it doesn't make sense for cumulative metrics
			if metricConfig.AggregateType != Last {
				return p.processCounterGroupWithStrategy(dataPoints, aggregator, metricConfig)
			}
		} else {
			// UpDownCounter (active_connections) - current state, can increase/decrease
			// "Last" is valid for UpDownCounters (current state snapshot)
			return p.processUpDownCounterGroupWithStrategy(dataPoints, aggregator, metricConfig)
		}
	}

	// For other metrics or default behavior: use simple last value
	lastDataPoint := dataPoints[len(dataPoints)-1]
	aggregator.UpdateLast(lastDataPoint.value, lastDataPoint.timestamp)

	return nil
}

// processGaugeGroupWithStrategy applies custom aggregation strategy to a group
func (p *streamingAggregationProcessor) processGaugeGroupWithStrategy(
	dataPoints []dataPointInfo,
	aggregator *Aggregator,
	metricConfig *MetricConfig,
) error {
	if len(dataPoints) == 0 {
		return nil
	}

	// Extract values and find latest timestamp
	var values []float64
	var latestTimestamp pcommon.Timestamp

	for _, dp := range dataPoints {
		values = append(values, dp.value)
		if dp.timestamp > latestTimestamp {
			latestTimestamp = dp.timestamp
		}
	}

	// Apply aggregation strategy
	var aggregatedValue float64
	switch metricConfig.AggregateType {
	case Sum:
		for _, v := range values {
			aggregatedValue += v
		}
	case Average:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		aggregatedValue = sum / float64(len(values))
	case Max:
		aggregatedValue = values[0]
		for _, v := range values[1:] {
			if v > aggregatedValue {
				aggregatedValue = v
			}
		}
	case Min:
		aggregatedValue = values[0]
		for _, v := range values[1:] {
			if v < aggregatedValue {
				aggregatedValue = v
			}
		}
	case Last:
		// Find the value with the latest timestamp
		latestIdx := 0
		latestTimestamp = dataPoints[0].timestamp
		for i := 1; i < len(dataPoints); i++ {
			if dataPoints[i].timestamp > latestTimestamp {
				latestIdx = i
				latestTimestamp = dataPoints[i].timestamp
			}
		}
		aggregatedValue = dataPoints[latestIdx].value
	default:
		aggregatedValue = values[len(values)-1]
	}

	// Update aggregator
	aggregator.UpdateLast(aggregatedValue, latestTimestamp)

	return nil
}

// processUpDownCounterGroupWithStrategy applies aggregation strategies specifically for UpDownCounters
func (p *streamingAggregationProcessor) processUpDownCounterGroupWithStrategy(
	dataPoints []dataPointInfo,
	aggregator *Aggregator,
	metricConfig *MetricConfig,
) error {
	if len(dataPoints) == 0 {
		return nil
	}

	// Extract current values and find latest timestamp
	var values []float64
	var latestTimestamp pcommon.Timestamp

	for _, dp := range dataPoints {
		values = append(values, dp.value)
		if dp.timestamp > latestTimestamp {
			latestTimestamp = dp.timestamp
		}
	}

	// Apply UpDownCounter-specific aggregation strategies
	var aggregatedValue float64
	switch metricConfig.AggregateType {
	case Sum:
		// Sum: Add all current connection values together
		// Example: Total connections across all instances
		for _, v := range values {
			aggregatedValue += v
		}
	case Average:
		// Average: Calculate mean current value across instances
		// Example: Average connections per instance
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		aggregatedValue = sum / float64(len(values))
	case Max:
		// Max: Find the highest current value
		// Example: Peak connections from any instance
		aggregatedValue = values[0]
		for _, v := range values[1:] {
			if v > aggregatedValue {
				aggregatedValue = v
			}
		}
	case Min:
		// Min: Find the lowest current value
		// Example: Minimum connections (could indicate issues)
		aggregatedValue = values[0]
		for _, v := range values[1:] {
			if v < aggregatedValue {
				aggregatedValue = v
			}
		}
	case Last:
		// Last: Most recent current value (by timestamp)
		// Example: Current connection state snapshot
		latestIdx := 0
		latestTimestamp = dataPoints[0].timestamp
		for i := 1; i < len(dataPoints); i++ {
			if dataPoints[i].timestamp > latestTimestamp {
				latestIdx = i
				latestTimestamp = dataPoints[i].timestamp
			}
		}
		aggregatedValue = dataPoints[latestIdx].value
	default:
		// Fallback to sum for unknown strategies
		for _, v := range values {
			aggregatedValue += v
		}
	}

	// Set the aggregation strategy on the aggregator
	aggregator.SetAggregateType(string(metricConfig.AggregateType))

	// For UpDownCounters, always use current value semantics (not cumulative)
	// Store the aggregated current value directly
	aggregator.UpdateUpDownCounter(aggregatedValue, latestTimestamp)

	return nil
}

// processCounterGroupWithStrategy applies aggregation strategies specifically for monotonic Counters
func (p *streamingAggregationProcessor) processCounterGroupWithStrategy(
	dataPoints []dataPointInfo,
	aggregator *Aggregator,
	metricConfig *MetricConfig,
) error {
	if len(dataPoints) == 0 {
		return nil
	}

	// Extract cumulative values and find latest timestamp
	var values []float64
	var latestTimestamp pcommon.Timestamp

	for _, dp := range dataPoints {
		values = append(values, dp.value)
		if dp.timestamp > latestTimestamp {
			latestTimestamp = dp.timestamp
		}
	}

	// Apply Counter-specific aggregation strategies
	var aggregatedValue float64
	switch metricConfig.AggregateType {
	case Sum:
		// Sum: Add all cumulative counter values together
		// Example: Total requests across all instances
		for _, v := range values {
			aggregatedValue += v
		}
	case Average:
		// Average: Calculate mean cumulative value across instances
		// Example: Average requests per instance since startup
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		aggregatedValue = sum / float64(len(values))
	case Rate:
		// Rate: Sum all values and let aggregator calculate rate at export time
		// Example: Total requests per second across all instances
		for _, v := range values {
			aggregatedValue += v
		}
	default:
		// Fallback to sum for unknown strategies (Max/Min don't make sense for counters)
		for _, v := range values {
			aggregatedValue += v
		}
	}

	// Set the aggregation strategy on the aggregator
	aggregator.SetAggregateType(string(metricConfig.AggregateType))

	// Handle Counter-specific storage based on strategy
	if metricConfig.AggregateType == Rate {
		// For rate, use counter delta logic but will export as gauge
		aggregator.ComputeDeltaFromCumulative(aggregatedValue)
	} else if metricConfig.AggregateType == Average {
		// For average strategy, set the averaged value as the final cumulative total
		aggregator.SetCounterTotal(aggregatedValue, latestTimestamp)
	} else {
		// For sum strategy, treat as counter delta and let aggregator handle cumulation
		aggregator.ComputeDeltaFromCumulative(aggregatedValue)
	}

	return nil
}

// processHistogramGroupWithStrategy applies aggregation strategies specifically for classic Histograms
func (p *streamingAggregationProcessor) processHistogramGroupWithStrategy(
	dataPoints []dataPointInfo,
	aggregator *Aggregator,
	metricConfig *MetricConfig,
	originalHistogram pmetric.Histogram,
) error {
	if len(dataPoints) == 0 {
		return nil
	}

	// Set the aggregation strategy on the aggregator
	aggregator.SetAggregateType(string(metricConfig.AggregateType))

	// For histograms, we need to aggregate across the grouped histogram data points
	// The dataPoints represent the grouped data, but we need to process histogram buckets
	return p.processHistogramDataPointsGrouped(originalHistogram, aggregator, metricConfig)
}

// processHistogramDataPoints processes histogram data points for aggregation
func (p *streamingAggregationProcessor) processHistogramDataPoints(
	histogram pmetric.Histogram,
	aggregator *Aggregator,
	metricConfig *MetricConfig,
) error {
	// Set the aggregation strategy on the aggregator
	aggregator.SetAggregateType(string(metricConfig.AggregateType))

	// Apply histogram-specific aggregation strategies
	switch metricConfig.AggregateType {
	case Sum:
		// Sum: Aggregate histogram buckets, counts, and sums across instances
		return p.aggregateHistogramSum(histogram, aggregator)
	case P50, P90, P95, P99:
		// Quantile: Calculate percentile from histogram buckets
		return p.aggregateHistogramQuantile(histogram, aggregator, metricConfig.AggregateType)
	default:
		return fmt.Errorf("unsupported aggregation type %q for histogram metrics", metricConfig.AggregateType)
	}
}

// aggregateHistogramSum aggregates histogram by summing buckets, counts, and sums
func (p *streamingAggregationProcessor) aggregateHistogramSum(
	histogram pmetric.Histogram,
	aggregator *Aggregator,
) error {
	// Process each data point in the histogram
	dataPoints := histogram.DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		// Update histogram aggregation with this data point
		err := aggregator.UpdateHistogramSum(dp)
		if err != nil {
			return fmt.Errorf("failed to update histogram sum: %w", err)
		}
	}
	return nil
}

// aggregateHistogramQuantile calculates a specific percentile from histogram data
func (p *streamingAggregationProcessor) aggregateHistogramQuantile(
	histogram pmetric.Histogram,
	aggregator *Aggregator,
	quantileType AggregationType,
) error {
	// First aggregate all histogram data points
	err := p.aggregateHistogramSum(histogram, aggregator)
	if err != nil {
		return err
	}

	// Calculate the requested quantile from aggregated histogram
	quantile := getQuantileValue(quantileType)
	value, timestamp := aggregator.CalculateHistogramQuantile(quantile)

	// Store the quantile result as a gauge value
	aggregator.UpdateLast(value, timestamp)

	return nil
}

// getQuantileValue returns the quantile value for the aggregation type
func getQuantileValue(aggregateType AggregationType) float64 {
	switch aggregateType {
	case P50:
		return 0.50
	case P90:
		return 0.90
	case P95:
		return 0.95
	case P99:
		return 0.99
	default:
		return 0.95 // Default to P95
	}
}

// processHistogramMetricWithStrategy processes histogram metrics with custom aggregation strategies
func (p *streamingAggregationProcessor) processHistogramMetricWithStrategy(
	originalMetric pmetric.Metric,
	aggregator *Aggregator,
	metricConfig *MetricConfig,
	seriesKey string,
) error {
	if originalMetric.Type() != pmetric.MetricTypeHistogram {
		return fmt.Errorf("expected histogram metric, got %s", originalMetric.Type())
	}

	// Create a filtered histogram containing only data points for this series key
	histogram := originalMetric.Histogram()
	filteredHistogram := p.createFilteredHistogram(histogram, metricConfig, seriesKey)

	return p.processHistogramDataPoints(filteredHistogram, aggregator, metricConfig)
}

// createFilteredHistogram creates a histogram containing only data points matching the series key
func (p *streamingAggregationProcessor) createFilteredHistogram(
	originalHistogram pmetric.Histogram,
	metricConfig *MetricConfig,
	targetSeriesKey string,
) pmetric.Histogram {
	// Create a new histogram with only matching data points
	filteredHistogram := pmetric.NewHistogram()

	originalDPs := originalHistogram.DataPoints()
	for i := 0; i < originalDPs.Len(); i++ {
		dp := originalDPs.At(i)

		// Apply label filtering to get the filtered attributes
		filteredAttrs := getLabelFilteredAttributes(dp.Attributes(), metricConfig.Labels)

		// Generate series key from filtered attributes
		seriesKey := p.generateSeriesKeyFromAttributes("http_response_time_ms", filteredAttrs)

		// Only include data points that match this series key
		if seriesKey == targetSeriesKey {
			// Copy this data point to the filtered histogram
			newDP := filteredHistogram.DataPoints().AppendEmpty()
			dp.CopyTo(newDP)
		}
	}

	return filteredHistogram
}

// processHistogramDataPointsGrouped processes grouped histogram data points for aggregation
func (p *streamingAggregationProcessor) processHistogramDataPointsGrouped(
	originalHistogram pmetric.Histogram,
	aggregator *Aggregator,
	metricConfig *MetricConfig,
) error {
	// For grouped processing, we directly process the entire histogram
	return p.processHistogramDataPoints(originalHistogram, aggregator, metricConfig)
}

// processExponentialHistogramMetricWithStrategy processes exponential histogram metrics with custom aggregation strategies
func (p *streamingAggregationProcessor) processExponentialHistogramMetricWithStrategy(
	originalMetric pmetric.Metric,
	aggregator *Aggregator,
	metricConfig *MetricConfig,
	seriesKey string,
) error {
	if originalMetric.Type() != pmetric.MetricTypeExponentialHistogram {
		return fmt.Errorf("expected exponential histogram metric, got %s", originalMetric.Type())
	}

	// Create a filtered exponential histogram containing only data points for this series key
	exponentialHistogram := originalMetric.ExponentialHistogram()
	filteredExpHistogram := p.createFilteredExponentialHistogram(exponentialHistogram, metricConfig, seriesKey, originalMetric.Name())

	return p.processExponentialHistogramDataPoints(originalMetric, filteredExpHistogram, aggregator, metricConfig)
}

// createFilteredExponentialHistogram creates an exponential histogram containing only data points matching the series key
func (p *streamingAggregationProcessor) createFilteredExponentialHistogram(
	originalExpHistogram pmetric.ExponentialHistogram,
	metricConfig *MetricConfig,
	targetSeriesKey string,
	metricName string,
) pmetric.ExponentialHistogram {
	// Create a new exponential histogram with only matching data points
	filteredExpHistogram := pmetric.NewExponentialHistogram()

	originalDPs := originalExpHistogram.DataPoints()
	for i := 0; i < originalDPs.Len(); i++ {
		dp := originalDPs.At(i)

		// Apply label filtering to get the filtered attributes
		filteredAttrs := getLabelFilteredAttributes(dp.Attributes(), metricConfig.Labels)

		// Generate series key from filtered attributes
		seriesKey := p.generateSeriesKeyFromAttributes(metricName, filteredAttrs)

		// Only include data points that match this series key
		if seriesKey == targetSeriesKey {
			// Copy this data point to the filtered exponential histogram
			newDP := filteredExpHistogram.DataPoints().AppendEmpty()
			dp.CopyTo(newDP)

			// Replace the attributes with filtered ones (this is the key fix!)
			newDP.Attributes().Clear()
			filteredAttrs.CopyTo(newDP.Attributes())
		}
	}

	return filteredExpHistogram
}

// processExponentialHistogramDataPoints processes exponential histogram data points for aggregation
func (p *streamingAggregationProcessor) processExponentialHistogramDataPoints(
	originalMetric pmetric.Metric,
	exponentialHistogram pmetric.ExponentialHistogram,
	aggregator *Aggregator,
	metricConfig *MetricConfig,
) error {
	// Set the aggregation strategy on the aggregator
	aggregator.SetAggregateType(string(metricConfig.AggregateType))

	// Apply exponential histogram-specific aggregation strategies
	switch metricConfig.AggregateType {
	case Sum:
		// Sum: Aggregate exponential histogram data points (maintains current behavior)
		return p.aggregateExponentialHistogramSum(originalMetric, exponentialHistogram, aggregator)
	default:
		return fmt.Errorf("unsupported aggregation type %q for exponential histogram metrics, only 'sum' is supported", metricConfig.AggregateType)
	}
}

// aggregateExponentialHistogramSum aggregates exponential histogram using the proven accumulator approach
func (p *streamingAggregationProcessor) aggregateExponentialHistogramSum(
	originalMetric pmetric.Metric,
	exponentialHistogram pmetric.ExponentialHistogram,
	aggregator *Aggregator,
) error {
	// Process each data point in the exponential histogram
	dataPoints := exponentialHistogram.DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		// Use the proven MergeExponentialHistogram method from aggregator
		// Pass the original metric so accumulator can capture name/description/unit properly
		err := aggregator.MergeExponentialHistogram(originalMetric, dp)
		if err != nil {
			return fmt.Errorf("failed to merge exponential histogram: %w", err)
		}
	}
	return nil
}

// aggregateMetricDefault provides the original aggregation logic for backward compatibility
func (p *streamingAggregationProcessor) aggregateMetricDefault(
	metric pmetric.Metric,
	metricConfig *MetricConfig,
) error {
	// Generate series key (default behavior - all metrics get same key)
	seriesKey := metric.Name() + "|"

	// Get or create aggregator at processor level (persistent across window swaps)
	aggregator := p.getOrCreateAggregator(seriesKey, metric.Type())

	// Aggregate based on metric type using internal aggregation package
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		config := aggregation.CounterAggregationConfig{
			StaleDataThreshold: p.config.StaleDataThreshold,
			Logger:             p.logger,
		}
		return aggregation.AggregateSum(metric.Sum(), aggregator, config, p.forceExportActiveWindow)
	case pmetric.MetricTypeGauge:
		config := aggregation.GaugeAggregationConfig{
			Logger: p.logger,
		}
		return aggregation.AggregateGauge(metric.Gauge(), aggregator, config)
	case pmetric.MetricTypeHistogram:
		config := aggregation.HistogramAggregationConfig{
			StaleDataThreshold: p.config.StaleDataThreshold,
			Logger:             p.logger,
		}
		return aggregation.AggregateHistogram(metric.Histogram(), aggregator, config)
	case pmetric.MetricTypeExponentialHistogram:
		return aggregation.AggregateExponentialHistogram(metric, metric.ExponentialHistogram(), aggregator)
	case pmetric.MetricTypeSummary:
		return aggregation.AggregateSummary(metric.Summary(), aggregator)
	default:
		return fmt.Errorf("unsupported metric type: %v", metric.Type())
	}
}

// extractDoubleValue extracts double value from NumberDataPoint
func extractDoubleValue(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	default:
		return 0
	}
}
