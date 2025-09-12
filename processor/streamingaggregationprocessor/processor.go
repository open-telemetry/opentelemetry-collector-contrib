// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/streamingaggregationprocessor"

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// streamingAggregationProcessor implements the streaming aggregation processor
// Double-buffer design: exactly 2 windows that alternate every windowSize duration
type streamingAggregationProcessor struct {
	logger *zap.Logger
	config *Config
	
	// Double-buffer window architecture
	windowA      *TimeWindow       // First window (time boundaries only)
	windowB      *TimeWindow       // Second window (time boundaries only)  
	activeWindow *TimeWindow       // Points to current writable window
	nextSwapTime time.Time         // When to swap windows
	windowSize   time.Duration
	
	// Aggregators stored at processor level (persistent across window swaps)
	aggregators   map[string]*Aggregator  // Key: series key
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
	
	// Memory management
	memoryUsage   atomic.Int64
	maxMemoryBytes int64
	
	// Export management
	nextConsumer   consumer.Metrics  // Store the next consumer in the pipeline
	
	// Mutex for window operations
	mu sync.RWMutex
}

// newStreamingAggregationProcessor creates a new streaming aggregation processor
func newStreamingAggregationProcessor(logger *zap.Logger, config *Config) (*streamingAggregationProcessor, error) {
	// Apply defaults to config
	config.applyDefaults()
	
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
	if alignedStart.Add(p.windowSize/2).Before(now) {
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

// aggregateMetric aggregates a single metric based on its type
func (p *streamingAggregationProcessor) aggregateMetric(
	metric pmetric.Metric,
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
) error {
	// For streaming aggregation, we aggregate all data points together regardless of attributes
	// This provides true cardinality reduction by dropping all labels
	// Use just the metric name as the key for all metric types
	seriesKey := metric.Name() + "|"
	
	// Get or create aggregator at processor level (persistent across window swaps)
	aggregator := p.getOrCreateAggregator(seriesKey, metric.Type())
	
	// Aggregate based on metric type
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		return p.aggregateSum(metric.Sum(), aggregator)
	case pmetric.MetricTypeGauge:
		return p.aggregateGauge(metric.Gauge(), aggregator)
	case pmetric.MetricTypeHistogram:
		return p.aggregateHistogram(metric.Histogram(), aggregator)
	case pmetric.MetricTypeExponentialHistogram:
		return p.aggregateExponentialHistogram(metric.ExponentialHistogram(), aggregator)
	case pmetric.MetricTypeSummary:
		return p.aggregateSummary(metric.Summary(), aggregator)
	default:
		return fmt.Errorf("unsupported metric type: %v", metric.Type())
	}
}

// aggregateGauge aggregates gauge metrics - keep last value
func (p *streamingAggregationProcessor) aggregateGauge(gauge pmetric.Gauge, agg *Aggregator) error {
	dps := gauge.DataPoints()
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
		
		agg.UpdateLast(value, dp.Timestamp())
	}
	return nil
}

// aggregateSum aggregates sum/counter metrics - sum values
func (p *streamingAggregationProcessor) aggregateSum(sum pmetric.Sum, agg *Aggregator) error {
	dps := sum.DataPoints()
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
		
		// Handle based on monotonic property and temporality
		if !sum.IsMonotonic() {
			// UpDownCounter - track the net change within the window
			// For UpDownCounters, we want to track the difference between the first
			// and last value seen in the window, not accumulate all changes
			if sum.AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
				// For cumulative UpDownCounters, track first and last values
				agg.UpdateUpDownCounter(value, dp.Timestamp())
			} else {
				// For delta UpDownCounters, sum the deltas
				agg.UpdateSum(value)
			}
			
			p.logger.Debug("Aggregating UpDownCounter",
				zap.Float64("value", value),
				zap.String("temporality", sum.AggregationTemporality().String()),
				zap.Bool("is_monotonic", sum.IsMonotonic()),
			)
		} else {
			// Regular counter (monotonic) - accumulate deltas
			if sum.AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
				// For cumulative, compute the delta from the previous value with gap detection
				// ComputeDeltaFromCumulativeWithGapDetection already updates counterWindowDeltaSum internally
				deltaValue, wasReset := agg.ComputeDeltaFromCumulativeWithGapDetection(value, dp.Timestamp(), p.config.StaleDataThreshold)
				// Don't call UpdateSum here - ComputeDeltaFromCumulativeWithGapDetection handles the accumulation
				
				// If cumulative state was reset due to gap, trigger immediate export
				if wasReset {
					go p.forceExportActiveWindow() // Export active window immediately in background
				}
				
				p.logger.Debug("Aggregating cumulative counter",
					zap.Float64("cumulative_value", value),
					zap.Float64("delta_value", deltaValue),
					zap.String("temporality", sum.AggregationTemporality().String()),
					zap.Bool("is_monotonic", sum.IsMonotonic()),
				)
			} else {
				// For delta temporality, use the value directly
				// For delta counters, we need to accumulate in the counter-specific fields
				// We treat delta values as increments to the total sum
				agg.mu.Lock()
				if !agg.hasCounterCumulative {
					// First delta value initializes the counter
					agg.counterTotalSum = value
					agg.hasCounterCumulative = true
				} else {
					// Add delta to total sum
					agg.counterTotalSum += value
				}
				agg.counterWindowDeltaSum += value
				agg.mu.Unlock()
				
				p.logger.Debug("Aggregating delta counter",
					zap.Float64("value", value),
					zap.Float64("total_sum", agg.counterTotalSum),
					zap.String("temporality", sum.AggregationTemporality().String()),
					zap.Bool("is_monotonic", sum.IsMonotonic()),
				)
			}
		}
	}
	return nil
}

// aggregateHistogram aggregates histogram metrics - merge buckets
func (p *streamingAggregationProcessor) aggregateHistogram(hist pmetric.Histogram, agg *Aggregator) error {
	dps := hist.DataPoints()
	temporality := hist.AggregationTemporality()
	
	// Debug logging to understand what's being aggregated
	p.logger.Debug("Aggregating histogram",
		zap.Int("data_points", dps.Len()),
		zap.String("temporality", temporality.String()),
	)
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		// Log each data point being aggregated
		attrs := make(map[string]string)
		dp.Attributes().Range(func(k string, v pcommon.Value) bool {
			attrs[k] = v.AsString()
			return true
		})
		
		p.logger.Debug("Merging histogram data point",
			zap.Uint64("count", dp.Count()),
			zap.Float64("sum", dp.Sum()),
			zap.Any("attributes", attrs),
			zap.String("temporality", temporality.String()),
		)
		
		agg.MergeHistogramWithTemporalityAndGapDetection(dp, temporality, p.config.StaleDataThreshold)
	}
	return nil
}

// aggregateExponentialHistogram aggregates exponential histogram metrics
func (p *streamingAggregationProcessor) aggregateExponentialHistogram(hist pmetric.ExponentialHistogram, agg *Aggregator) error {
	dps := hist.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		agg.MergeExponentialHistogram(dp)
	}
	return nil
}

// aggregateSummary aggregates summary metrics
func (p *streamingAggregationProcessor) aggregateSummary(summary pmetric.Summary, agg *Aggregator) error {
	dps := summary.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		agg.UpdateSum(dp.Sum())
		agg.UpdateCount(dp.Count())
	}
	return nil
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
		
		// Create metric based on aggregator state
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(metricName)
		
		// Export based on metric type
		agg.ExportTo(metric, windowStart, windowEnd, labels)
	}
	
	return md
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
			
			p.logger.Info("Processor statistics",
				zap.Int64("metrics_received", received),
				zap.Int64("metrics_processed", processed),
				zap.Int64("metrics_dropped", dropped),
				zap.Float64("drop_rate", float64(dropped)/float64(received)*100),
				zap.Int64("memory_bytes", p.memoryUsage.Load()),
			)
			
		case <-p.ctx.Done():
			return
		}
	}
}

// Helper functions

func getMetricTimestamp(metric pmetric.Metric) time.Time {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		if metric.Gauge().DataPoints().Len() > 0 {
			return metric.Gauge().DataPoints().At(0).Timestamp().AsTime()
		}
	case pmetric.MetricTypeSum:
		if metric.Sum().DataPoints().Len() > 0 {
			return metric.Sum().DataPoints().At(0).Timestamp().AsTime()
		}
	case pmetric.MetricTypeHistogram:
		if metric.Histogram().DataPoints().Len() > 0 {
			return metric.Histogram().DataPoints().At(0).Timestamp().AsTime()
		}
	case pmetric.MetricTypeExponentialHistogram:
		if metric.ExponentialHistogram().DataPoints().Len() > 0 {
			return metric.ExponentialHistogram().DataPoints().At(0).Timestamp().AsTime()
		}
	case pmetric.MetricTypeSummary:
		if metric.Summary().DataPoints().Len() > 0 {
			return metric.Summary().DataPoints().At(0).Timestamp().AsTime()
		}
	}
	return time.Now()
}

func buildSeriesKey(metric pmetric.Metric, resource pcommon.Resource) string {
	// Build a unique key for the series based on metric name and all labels
	// Keep metric name separate from labels for easier parsing
	key := metric.Name() + "|"
	
	// Labels to skip to avoid conflicts with Prometheus exporter const_labels
	skipLabels := map[string]bool{
		"environment": true,
		"job": true,
		"instance": true,
	}
	
	// Add resource attributes, filtering out problematic ones
	resourceAttrs := resource.Attributes()
	first := true
	resourceAttrs.Range(func(k string, v pcommon.Value) bool {
		// Skip labels that would conflict with Prometheus
		if skipLabels[k] {
			return true
		}
		
		if !first {
			key += ","
		}
		key += fmt.Sprintf("%s=%s", k, v.AsString())
		first = false
		return true
	})
	
	return key
}
