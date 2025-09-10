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
// Single-instance design: upstream sharding handles distribution
type streamingAggregationProcessor struct {
	logger *zap.Logger
	config *Config
	
	// Time windows for aggregation
	windows       []*Window
	currentWindow int
	windowSize    time.Duration
	numWindows    int
	
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
	exportTicker   *time.Ticker
	rotateTicker   *time.Ticker
	lastExportTime time.Time
	nextConsumer   consumer.Metrics  // Store the next consumer in the pipeline
	
	// Mutex for window operations
	mu sync.RWMutex
}

// newStreamingAggregationProcessor creates a new streaming aggregation processor
func newStreamingAggregationProcessor(logger *zap.Logger, config *Config) (*streamingAggregationProcessor, error) {
	// Apply defaults to config
	config.applyDefaults()
	
	ctx, cancel := context.WithCancel(context.Background())
	
	p := &streamingAggregationProcessor{
		logger:         logger,
		config:         config,
		ctx:            ctx,
		cancel:         cancel,
		windowSize:     config.WindowSize,
		numWindows:     config.NumWindows,
		windows:        make([]*Window, config.NumWindows),
		maxMemoryBytes: int64(config.MaxMemoryMB * 1024 * 1024),
	}
	
	// Initialize windows starting from current time
	// This prevents "out of order" errors when restarting the processor
	now := time.Now()
	// Align to window boundary for cleaner timestamps
	alignedStart := now.Truncate(config.WindowSize)
	for i := 0; i < p.numWindows; i++ {
		// All windows start empty from the current aligned time
		// They will be populated as new data arrives
		windowStart := alignedStart.Add(time.Duration(i) * config.WindowSize)
		p.windows[i] = NewWindow(windowStart, windowStart.Add(config.WindowSize))
	}
	// Start with the first window as current
	p.currentWindow = 0
	
	return p, nil
}

// Start starts the processor
func (p *streamingAggregationProcessor) Start(ctx context.Context, host component.Host) error {
	p.startOnce.Do(func() {
		p.logger.Info("Starting streaming aggregation processor (single-instance mode)",
			zap.Duration("window_size", p.config.WindowSize),
			zap.Int("num_windows", p.config.NumWindows),
		)
		
		// Start window rotation
		p.rotateTicker = time.NewTicker(p.windowSize)
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.runWindowRotation()
		}()
		
		// Start export ticker
		p.exportTicker = time.NewTicker(p.config.ExportInterval)
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.runExporter()
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
		
		// Stop tickers
		if p.rotateTicker != nil {
			p.rotateTicker.Stop()
		}
		if p.exportTicker != nil {
			p.exportTicker.Stop()
		}
		
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
		p.forceExport()
	})
	
	return shutdownErr
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
	// Get timestamp from metric
	timestamp := getMetricTimestamp(metric)
	
	// Find appropriate window
	p.mu.RLock()
	window := p.getWindowForTimestamp(timestamp)
	p.mu.RUnlock()
	
	if window == nil {
		return fmt.Errorf("metric timestamp outside of window range")
	}
	
	// For streaming aggregation, we aggregate all data points together regardless of attributes
	// This provides true cardinality reduction by dropping all labels
	// Use just the metric name as the key for all metric types
	seriesKey := metric.Name() + "|"
	
	// Get or create aggregator
	aggregator := window.GetOrCreateAggregator(seriesKey, metric.Type())
	
	// Update memory tracking
	p.memoryUsage.Add(aggregator.EstimateMemoryUsage())
	
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
				// For cumulative, compute the delta from the previous value
				// ComputeDeltaFromCumulative already updates counterWindowDeltaSum internally
				deltaValue := agg.ComputeDeltaFromCumulative(value)
				// Don't call UpdateSum here - ComputeDeltaFromCumulative handles the accumulation
				
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
		
		agg.MergeHistogramWithTemporality(dp, temporality)
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

// getWindowForTimestamp returns the appropriate window for a timestamp
func (p *streamingAggregationProcessor) getWindowForTimestamp(timestamp time.Time) *Window {
	// Find the appropriate window
	for _, window := range p.windows {
		if window.Contains(timestamp) {
			return window
		}
	}
	
	// Check if within late arrival tolerance (5 seconds default)
	currentWindow := p.windows[p.currentWindow]
	lateArrivalTolerance := 5 * time.Second
	if timestamp.After(currentWindow.start.Add(-lateArrivalTolerance)) {
		return currentWindow
	}
	
	return nil
}

// runWindowRotation handles window rotation
func (p *streamingAggregationProcessor) runWindowRotation() {
	for {
		select {
		case <-p.rotateTicker.C:
			p.rotateWindows()
		case <-p.ctx.Done():
			return
		}
	}
}

// rotateWindows rotates the time windows and exports completed windows
func (p *streamingAggregationProcessor) rotateWindows() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.logger.Debug("Window rotation triggered",
		zap.Time("rotation_time", time.Now()),
		zap.Int("num_windows", p.numWindows),
	)
	
	// If we only have one window, export it and create a new one
	if p.numWindows == 1 {
		window := p.windows[0]
		if window.HasData() {
			p.logger.Debug("Exporting single window before rotation",
				zap.Time("window_start", window.start),
				zap.Time("window_end", window.end),
			)
			p.exportWindow(window)
		}
		// Clear the window to reset all aggregator state
		window.Clear()
		newStart := time.Now()
		p.windows[0] = NewWindow(newStart, newStart.Add(p.windowSize))
		return
	}
	
	// For multiple windows, export the oldest completed window
	// The oldest window (index 0) is the one that just completed
	oldestWindow := p.windows[0]
	if oldestWindow.HasData() {
		p.logger.Debug("Exporting oldest window during rotation",
			zap.Time("window_start", oldestWindow.start),
			zap.Time("window_end", oldestWindow.end),
		)
		p.exportWindow(oldestWindow)
		// Clear the window to free memory and reset aggregator state
		oldestWindow.Clear()
	}
	
	// Shift windows - move all windows one position to the left
	for i := 0; i < p.numWindows-1; i++ {
		p.windows[i] = p.windows[i+1]
	}
	
	// Create new current window at the end
	lastWindow := p.windows[p.numWindows-2]
	newStart := lastWindow.end
	p.windows[p.numWindows-1] = NewWindow(newStart, newStart.Add(p.windowSize))
	
	p.logger.Debug("Windows rotated successfully",
		zap.Time("new_window_start", newStart),
		zap.Time("new_window_end", newStart.Add(p.windowSize)),
	)
}

// runExporter handles periodic export of completed windows
func (p *streamingAggregationProcessor) runExporter() {
	for {
		select {
		case <-p.exportTicker.C:
			p.exportCompletedWindows()
		case <-p.ctx.Done():
			return
		}
	}
}

// exportCompletedWindows exports windows that are complete
func (p *streamingAggregationProcessor) exportCompletedWindows() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	now := time.Now()
	
	for i, window := range p.windows {
		// Skip current window and future windows
		if i >= p.currentWindow {
			continue
		}
		
		// Export if window is complete (end time has passed)
		if window.end.Before(now) && window.HasData() {
			p.exportWindow(window)
			// Don't clear here - let rotation handle cleanup
			// This allows GetAggregatedMetrics to still see the data
		}
	}
}

// exportWindow exports a single window's aggregated metrics
func (p *streamingAggregationProcessor) exportWindow(window *Window) {
	metrics := window.Export()
	if metrics.DataPointCount() > 0 {
		// Send aggregated metrics to the next consumer in the pipeline
		if p.nextConsumer != nil {
			ctx := context.Background()
			if err := p.nextConsumer.ConsumeMetrics(ctx, metrics); err != nil {
				p.logger.Error("Failed to export aggregated metrics",
					zap.Error(err),
					zap.Time("window_start", window.start),
					zap.Time("window_end", window.end),
					zap.Int("data_points", metrics.DataPointCount()),
				)
			} else {
				p.logger.Info("Successfully exported aggregated window",
					zap.Time("window_start", window.start),
					zap.Time("window_end", window.end),
					zap.Int("data_points", metrics.DataPointCount()),
				)
			}
		} else {
			p.logger.Warn("No next consumer configured, metrics not exported",
				zap.Time("window_start", window.start),
				zap.Time("window_end", window.end),
				zap.Int("data_points", metrics.DataPointCount()),
			)
		}
		
		// Update memory usage
		p.memoryUsage.Add(-window.GetMemoryUsage())
	}
}

// forceExport forces export of all windows (used during shutdown)
func (p *streamingAggregationProcessor) forceExport() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	for _, window := range p.windows {
		if window.HasData() {
			p.exportWindow(window)
		}
	}
}

// GetAggregatedMetrics returns the current aggregated metrics (for testing)
// This is not part of the normal flow - metrics are normally exported on schedule
func (p *streamingAggregationProcessor) GetAggregatedMetrics() pmetric.Metrics {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	combined := pmetric.NewMetrics()
	
	// Only export metrics from the current window to avoid duplication
	// The current window is the one actively receiving and aggregating data
	currentWindow := p.windows[p.currentWindow]
	if currentWindow.HasData() {
		windowMetrics := currentWindow.Export()
		if windowMetrics.DataPointCount() > 0 {
			// Merge window metrics into combined
			windowMetrics.ResourceMetrics().MoveAndAppendTo(combined.ResourceMetrics())
		}
	}
	
	return combined
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
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Evict oldest windows first
	for i := 0; i < p.numWindows-1; i++ {
		if p.windows[i].HasData() {
			bytesFreed := p.windows[i].GetMemoryUsage()
			p.windows[i].Clear()
			p.memoryUsage.Add(-bytesFreed)
			
			p.logger.Debug("Evicted window",
				zap.Int("window_index", i),
				zap.Int64("bytes_freed", bytesFreed),
			)
			
			// Check if we've freed enough memory
			if p.memoryUsage.Load() < p.maxMemoryBytes {
				return
			}
		}
	}
	
	// If still over limit, evict low-priority series from current window
	if p.memoryUsage.Load() > p.maxMemoryBytes {
		currentWindow := p.windows[p.currentWindow]
		bytesFreed := currentWindow.EvictLowPrioritySeries(0.2) // Evict 20% of series
		p.memoryUsage.Add(-bytesFreed)
		
		p.logger.Debug("Evicted low-priority series",
			zap.Int64("bytes_freed", bytesFreed),
		)
	}
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
