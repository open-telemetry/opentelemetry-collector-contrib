// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/streamingaggregationprocessor"

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// TimeWindow represents a time window boundary (no aggregators stored here)
type TimeWindow struct {
	start time.Time
	end   time.Time
}

// Window represents a time window for aggregation (deprecated - use TimeWindow for new double-buffer design)
type Window struct {
	start time.Time
	end   time.Time
	
	// Aggregators by series key
	aggregators map[string]*Aggregator
	
	// Memory tracking
	memoryUsage int64
	
	// For LRU eviction
	lastAccess map[string]time.Time
	
	// Mutex for thread-safe access
	mu sync.RWMutex
}

// NewTimeWindow creates a new time window boundary
func NewTimeWindow(start, end time.Time) *TimeWindow {
	return &TimeWindow{
		start: start,
		end:   end,
	}
}

// Contains checks if a timestamp falls within this time window
func (tw *TimeWindow) Contains(timestamp time.Time) bool {
	return !timestamp.Before(tw.start) && timestamp.Before(tw.end)
}

// NewWindow creates a new time window (deprecated - use NewTimeWindow for new double-buffer design)
func NewWindow(start, end time.Time) *Window {
	return &Window{
		start:       start,
		end:         end,
		aggregators: make(map[string]*Aggregator),
		lastAccess:  make(map[string]time.Time),
	}
}

// Contains checks if a timestamp falls within this window
func (w *Window) Contains(timestamp time.Time) bool {
	return !timestamp.Before(w.start) && timestamp.Before(w.end)
}

// GetOrCreateAggregator gets or creates an aggregator for a series
func (w *Window) GetOrCreateAggregator(seriesKey string, metricType pmetric.MetricType) *Aggregator {
	w.mu.RLock()
	if agg, exists := w.aggregators[seriesKey]; exists {
		w.mu.RUnlock()
		w.mu.Lock()
		w.lastAccess[seriesKey] = time.Now()
		w.mu.Unlock()
		return agg
	}
	w.mu.RUnlock()
	
	// Create new aggregator under write lock
	w.mu.Lock()
	defer w.mu.Unlock()
	
	// Double-check in case another goroutine created it
	if agg, exists := w.aggregators[seriesKey]; exists {
		w.lastAccess[seriesKey] = time.Now()
		return agg
	}
	
	// Create new aggregator
	agg := NewAggregator(metricType)
	w.aggregators[seriesKey] = agg
	w.lastAccess[seriesKey] = time.Now()
	
	// Update memory usage estimate
	w.memoryUsage += agg.EstimateMemoryUsage()
	
	return agg
}

// HasData returns true if the window contains any data
func (w *Window) HasData() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.aggregators) > 0
}

// GetSeriesCount returns the number of series in the window
func (w *Window) GetSeriesCount() int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return int64(len(w.aggregators))
}

// GetMemoryUsage returns the estimated memory usage
func (w *Window) GetMemoryUsage() int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.memoryUsage
}

// Clear clears all data from the window
func (w *Window) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	// Instead of completely clearing aggregators, reset them for the new window
	// This preserves cumulative state needed for proper delta computation
	for _, agg := range w.aggregators {
		agg.ResetForNewWindow()
	}
	
	// Clear the last access times as they are window-specific
	w.lastAccess = make(map[string]time.Time)
	
	// Recalculate memory usage after reset
	w.memoryUsage = 0
	for _, agg := range w.aggregators {
		w.memoryUsage += agg.EstimateMemoryUsage()
	}
}

// Export exports the aggregated metrics from the window
func (w *Window) Export() pmetric.Metrics {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	md := pmetric.NewMetrics()
	
	if len(w.aggregators) == 0 {
		return md
	}
	
	// Group aggregators by resource and scope
	// For simplicity, we'll create a single resource and scope
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("streamingaggregation")
	
	// Export each aggregator
	for seriesKey, agg := range w.aggregators {
		// Parse series key to extract metric name and labels
		metricName, labels := parseSeriesKey(seriesKey)
		
		// Create metric based on aggregator state
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(metricName)
		
		// Export based on metric type (use cumulative for backward compatibility)
		agg.ExportTo(metric, w.start, w.end, labels)
	}
	
	return md
}

// EvictLowPrioritySeries evicts a percentage of least recently used series
func (w *Window) EvictLowPrioritySeries(percentage float64) int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if len(w.aggregators) == 0 {
		return 0
	}
	
	// Sort series by last access time
	type seriesAccess struct {
		key        string
		lastAccess time.Time
	}
	
	series := make([]seriesAccess, 0, len(w.aggregators))
	for key, lastAccess := range w.lastAccess {
		series = append(series, seriesAccess{key: key, lastAccess: lastAccess})
	}
	
	sort.Slice(series, func(i, j int) bool {
		return series[i].lastAccess.Before(series[j].lastAccess)
	})
	
	// Evict the specified percentage
	toEvict := int(float64(len(series)) * percentage)
	if toEvict == 0 && len(series) > 0 {
		toEvict = 1 // Evict at least one
	}
	
	var bytesFreed int64
	for i := 0; i < toEvict && i < len(series); i++ {
		key := series[i].key
		if agg, exists := w.aggregators[key]; exists {
			bytesFreed += agg.EstimateMemoryUsage()
			delete(w.aggregators, key)
			delete(w.lastAccess, key)
		}
	}
	
	w.memoryUsage -= bytesFreed
	return bytesFreed
}

// Aggregator handles aggregation for a single metric series
type Aggregator struct {
	metricType pmetric.MetricType
	
	// ===== GAUGE METRICS =====
	// For gauge metrics
	lastValue     float64
	lastTimestamp pcommon.Timestamp
	
	// ===== BASIC AGGREGATIONS =====
	// For basic aggregations
	count int64
	sum   float64
	min   float64
	max   float64
	
	// For mean calculation
	sumForMean   float64
	countForMean int64
	
	// ===== RATE CALCULATION =====
	// For rate calculation
	firstValue     float64
	firstTimestamp pcommon.Timestamp
	lastRateValue  float64
	lastRateTime   pcommon.Timestamp
	
	// ===== CUMULATIVE TO DELTA CONVERSION =====
	// For cumulative to delta conversion
	lastCumulativeValue float64
	hasCumulativeValue  bool
	
	// ===== UPDOWNCOUNTER TRACKING =====
	// For UpDownCounter tracking (non-monotonic sums)
	upDownFirstValue    float64
	upDownLastValue     float64
	hasUpDownFirstValue bool
	
	// ===== COUNTER-SPECIFIC FIELDS =====
	// Counter-specific fields for proper delta aggregation
	counterWindowDeltaSum float64      // Sum of deltas in current window only
	counterLastCumulative float64      // Last cumulative value seen (persistent across windows)
	counterTotalSum       float64      // Total accumulated sum across all windows
	hasCounterCumulative  bool         // Whether we've seen a cumulative value
	isCounterReset        bool         // Flag to track counter resets
	counterLastTimestamp  pcommon.Timestamp // Last timestamp for gap detection
	
	// ===== HISTOGRAM WINDOW-SPECIFIC STATE (reset each window) =====
	histogramBuckets map[float64]uint64  // Current window bucket counts
	histogramSum     float64              // Current window sum
	histogramCount   uint64               // Current window count
	
	// ===== HISTOGRAM CUMULATIVE TRACKING (persistent across windows) =====
	// Total accumulated values - these are what get exported
	histogramTotalBuckets map[float64]uint64  // Total accumulated bucket counts
	histogramTotalSum     float64              // Total accumulated sum
	histogramTotalCount   uint64               // Total accumulated count
	
	// Last seen cumulative values for delta computation
	// Track per source (label combination) for proper aggregation
	lastHistogramValues map[string]*histogramCumulativeState // Key is label hash
	
	// ===== EXPONENTIAL HISTOGRAM =====
	// For exponential histogram
	expHistogram *ExponentialHistogramState
	
	// ===== PERCENTILE CALCULATION =====
	// For percentile calculation (using reservoir sampling)
	reservoir []float64
	
	// ===== THREAD SAFETY =====
	mu sync.Mutex // Protects all fields for thread-safe access
}

// ExponentialHistogramState holds state for exponential histogram aggregation
type ExponentialHistogramState struct {
	scale        int32
	zeroCount    uint64
	sum          float64
	count        uint64
	positiveBuckets map[int32]uint64
	negativeBuckets map[int32]uint64
}

// histogramCumulativeState tracks cumulative state for a specific source
type histogramCumulativeState struct {
	lastSum       float64
	lastCount     uint64
	lastBuckets   map[float64]uint64
	lastTimestamp pcommon.Timestamp
}

// NewAggregator creates a new aggregator
func NewAggregator(metricType pmetric.MetricType) *Aggregator {
	return &Aggregator{
		metricType:          metricType,
		min:                 1e308,  // Max float64
		max:                 -1e308, // Min float64
		histogramBuckets:    make(map[float64]uint64),
		reservoir:           make([]float64, 0, 1000), // Reservoir sampling with 1000 samples
		lastHistogramValues: make(map[string]*histogramCumulativeState),
	}
}

// UpdateLast updates the last value (for gauges)
func (a *Aggregator) UpdateLast(value float64, timestamp pcommon.Timestamp) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastValue = value
	a.lastTimestamp = timestamp
}

// UpdateSum updates the sum
func (a *Aggregator) UpdateSum(value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sum += value
	a.count++
}

// ComputeDeltaFromCumulative computes the delta from a cumulative value
func (a *Aggregator) ComputeDeltaFromCumulative(cumulativeValue float64) float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// For counter-specific tracking, use the new fields
	if !a.hasCounterCumulative {
		a.counterLastCumulative = cumulativeValue
		a.hasCounterCumulative = true
		a.counterLastTimestamp = pcommon.NewTimestampFromTime(time.Now())
		// Return the cumulative value as initial delta (not 0)
		// This preserves the initial counter value
		a.counterWindowDeltaSum = cumulativeValue
		a.counterTotalSum = cumulativeValue // Initialize total sum
		return cumulativeValue
	}
	
	// Compute the delta from the last cumulative value
	delta := cumulativeValue - a.counterLastCumulative
	
	// Handle counter resets (when cumulative value decreases)
	if delta < 0 {
		// Counter was reset, use the new value as the delta
		delta = cumulativeValue
		a.isCounterReset = true
	}
	
	// Update the last cumulative value, window delta sum, and total sum
	a.counterLastCumulative = cumulativeValue
	a.counterLastTimestamp = pcommon.NewTimestampFromTime(time.Now())
	a.counterWindowDeltaSum += delta
	a.counterTotalSum += delta // Accumulate to total
	
	return delta
}

// ComputeDeltaFromCumulativeWithGapDetection computes delta with gap detection
// Returns (deltaValue, wasReset) - wasReset indicates if cumulative state was reset due to gap
func (a *Aggregator) ComputeDeltaFromCumulativeWithGapDetection(cumulativeValue float64, timestamp pcommon.Timestamp, staleThreshold time.Duration) (float64, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	currentTime := timestamp.AsTime()
	fmt.Printf("GAP DETECTION: Checking cumulative value %.2f at %v (threshold: %v)\n", cumulativeValue, currentTime, staleThreshold)
	
	// Check if we have existing cumulative state and if it's stale
	if a.hasCounterCumulative {
		lastTime := a.counterLastTimestamp.AsTime()
		timeSinceLastData := currentTime.Sub(lastTime)
		
		// If data is stale (gap detected), reset cumulative state
		if timeSinceLastData > staleThreshold {
			// Reset cumulative state - treat new value as initial value
			fmt.Printf("GAP DETECTED: Counter reset after %v gap (threshold: %v), old value: %.2f, new value: %.2f\n", timeSinceLastData, staleThreshold, a.counterLastCumulative, cumulativeValue)
			a.counterLastCumulative = cumulativeValue
			a.counterLastTimestamp = timestamp
			a.counterWindowDeltaSum = cumulativeValue
			a.counterTotalSum = cumulativeValue
			a.isCounterReset = false
			return cumulativeValue, true // true indicates reset occurred
		}
	}
	
	// No gap detected, use normal delta computation
	if !a.hasCounterCumulative {
		a.counterLastCumulative = cumulativeValue
		a.hasCounterCumulative = true
		a.counterLastTimestamp = timestamp
		a.counterWindowDeltaSum = cumulativeValue
		a.counterTotalSum = cumulativeValue
		return cumulativeValue, false // false indicates no reset
	}
	
	// Compute the delta from the last cumulative value
	delta := cumulativeValue - a.counterLastCumulative
	
	// Handle counter resets (when cumulative value decreases)
	if delta < 0 {
		delta = cumulativeValue
		a.isCounterReset = true
	}
	
	// Update the last cumulative value, window delta sum, and total sum
	a.counterLastCumulative = cumulativeValue
	a.counterLastTimestamp = timestamp
	a.counterWindowDeltaSum += delta
	a.counterTotalSum += delta
	
	return delta, false // false indicates no reset
}

// UpdateMean updates values for mean calculation
func (a *Aggregator) UpdateMean(value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sumForMean += value
	a.countForMean++
	a.updateMinMaxLocked(value)
}

// UpdateMin updates the minimum value
func (a *Aggregator) UpdateMin(value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if value < a.min {
		a.min = value
	}
}

// UpdateMax updates the maximum value
func (a *Aggregator) UpdateMax(value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if value > a.max {
		a.max = value
	}
}

// IncrementCount increments the count
func (a *Aggregator) IncrementCount() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.count++
}

// UpdateCount updates the count
func (a *Aggregator) UpdateCount(count uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.count += int64(count)
}

// UpdateRate updates values for rate calculation
func (a *Aggregator) UpdateRate(value float64, timestamp pcommon.Timestamp) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.firstTimestamp == 0 {
		a.firstValue = value
		a.firstTimestamp = timestamp
	}
	a.lastRateValue = value
	a.lastRateTime = timestamp
}

// UpdateIncrease updates the increase value
func (a *Aggregator) UpdateIncrease(value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.firstValue == 0 {
		a.firstValue = value
	}
	a.lastRateValue = value
}

// UpdateUpDownCounter updates values for UpDownCounter (non-monotonic sum)
// For UpDownCounters, we track the net change within the window
func (a *Aggregator) UpdateUpDownCounter(value float64, timestamp pcommon.Timestamp) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if !a.hasUpDownFirstValue {
		a.upDownFirstValue = value
		a.hasUpDownFirstValue = true
	}
	a.upDownLastValue = value
	a.lastTimestamp = timestamp
}

// MergeHistogram merges a histogram data point (for backward compatibility)
func (a *Aggregator) MergeHistogram(dp pmetric.HistogramDataPoint) {
	// Default to cumulative temporality for backward compatibility
	a.MergeHistogramWithTemporality(dp, pmetric.AggregationTemporalityCumulative)
}

// MergeHistogramWithTemporality merges a histogram data point with specified temporality
func (a *Aggregator) MergeHistogramWithTemporality(dp pmetric.HistogramDataPoint, temporality pmetric.AggregationTemporality) {
	a.MergeHistogramWithTemporalityAndGapDetection(dp, temporality, 0)
}

// MergeHistogramWithTemporalityAndGapDetection merges a histogram data point with gap detection
func (a *Aggregator) MergeHistogramWithTemporalityAndGapDetection(dp pmetric.HistogramDataPoint, temporality pmetric.AggregationTemporality, staleThreshold time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	currentSum := dp.Sum()
	currentCount := dp.Count()
	
	// Initialize buckets if needed
	if a.histogramTotalBuckets == nil {
		a.histogramTotalBuckets = make(map[float64]uint64)
	}
	if a.histogramBuckets == nil {
		a.histogramBuckets = make(map[float64]uint64)
	}
	if a.lastHistogramValues == nil {
		a.lastHistogramValues = make(map[string]*histogramCumulativeState)
	}
	
	if temporality == pmetric.AggregationTemporalityDelta {
		// For delta temporality, directly add the values
		a.histogramSum += currentSum
		a.histogramCount += currentCount
		a.histogramTotalSum += currentSum
		a.histogramTotalCount += currentCount
		
		// Add bucket counts
		buckets := dp.BucketCounts()
		bounds := dp.ExplicitBounds()
		
		for i := 0; i < buckets.Len() && i <= bounds.Len(); i++ {
			var bound float64
			if i < bounds.Len() {
				bound = bounds.At(i)
			} else {
				bound = 1e308 // Infinity bucket
			}
			bucketCount := buckets.At(i)
			a.histogramBuckets[bound] += bucketCount
			a.histogramTotalBuckets[bound] += bucketCount
		}
	} else {
		// For cumulative temporality in streaming aggregation:
		// We need to track per-source state even though labels are dropped.
		// Use the attributes to create a source key for tracking.
		sourceKey := getSourceKey(dp.Attributes())
		
		// Get or create the cumulative state for this source
		sourceState, exists := a.lastHistogramValues[sourceKey]
		currentTime := time.Now()
		
		// Check for stale data if threshold is provided and state exists
		if exists && staleThreshold > 0 {
			lastTime := sourceState.lastTimestamp.AsTime()
			if !lastTime.IsZero() {
				timeSinceLastData := currentTime.Sub(lastTime)
				if timeSinceLastData > staleThreshold {
					// Reset cumulative state - treat as new source
					exists = false
				}
			}
		}
		
		if !exists {
			sourceState = &histogramCumulativeState{
				lastBuckets:   make(map[float64]uint64),
				lastTimestamp: pcommon.NewTimestampFromTime(currentTime),
			}
			a.lastHistogramValues[sourceKey] = sourceState
		}
		
		// Compute deltas from the last seen values for this source
		deltaSum := currentSum - sourceState.lastSum
		deltaCount := currentCount - sourceState.lastCount
		
		// If this is the first time seeing this source, use the full values
		if !exists {
			deltaSum = currentSum
			deltaCount = currentCount
		} else if deltaCount < 0 || deltaSum < 0 {
			// Handle counter reset - when cumulative values decrease
			// This means the counter was reset, so use the new values as-is
			deltaSum = currentSum
			deltaCount = currentCount
		}
		
		// Update window and total values with the deltas
		a.histogramSum += deltaSum
		a.histogramCount += deltaCount
		a.histogramTotalSum += deltaSum
		a.histogramTotalCount += deltaCount
		
		// Update bucket counts with deltas
		buckets := dp.BucketCounts()
		bounds := dp.ExplicitBounds()
		
		for i := 0; i < buckets.Len() && i <= bounds.Len(); i++ {
			var bound float64
			if i < bounds.Len() {
				bound = bounds.At(i)
			} else {
				bound = 1e308 // Infinity bucket
			}
			
			currentBucketCount := buckets.At(i)
			lastBucketCount := sourceState.lastBuckets[bound]
			deltaBucketCount := currentBucketCount - lastBucketCount
			
			// If first time, use full value
			if !exists {
				deltaBucketCount = currentBucketCount
			} else if deltaBucketCount < 0 {
				// Handle counter reset for this bucket
				deltaBucketCount = currentBucketCount
			}
			
			a.histogramBuckets[bound] += deltaBucketCount
			a.histogramTotalBuckets[bound] += deltaBucketCount
			
			// Update the last seen value for this bucket
			sourceState.lastBuckets[bound] = currentBucketCount
		}
		
		// Update the last seen values for this source
		sourceState.lastSum = currentSum
		sourceState.lastCount = currentCount
		sourceState.lastTimestamp = pcommon.NewTimestampFromTime(currentTime)
	}
	
	// Update min/max if available
	if dp.HasMin() {
		a.updateMinMaxLocked(dp.Min())
	}
	if dp.HasMax() {
		a.updateMinMaxLocked(dp.Max())
	}
}

// MergeExponentialHistogram merges an exponential histogram data point
func (a *Aggregator) MergeExponentialHistogram(dp pmetric.ExponentialHistogramDataPoint) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if a.expHistogram == nil {
		a.expHistogram = &ExponentialHistogramState{
			scale:           dp.Scale(),
			positiveBuckets: make(map[int32]uint64),
			negativeBuckets: make(map[int32]uint64),
		}
	}
	
	a.expHistogram.sum += dp.Sum()
	a.expHistogram.count += dp.Count()
	a.expHistogram.zeroCount += dp.ZeroCount()
	
	// Merge positive buckets
	positive := dp.Positive()
	positiveCounts := positive.BucketCounts()
	for i := 0; i < positiveCounts.Len(); i++ {
		bucketIndex := int32(i) + positive.Offset()
		a.expHistogram.positiveBuckets[bucketIndex] += positiveCounts.At(i)
	}
	
	// Merge negative buckets
	negative := dp.Negative()
	negativeCounts := negative.BucketCounts()
	for i := 0; i < negativeCounts.Len(); i++ {
		bucketIndex := int32(i) + negative.Offset()
		a.expHistogram.negativeBuckets[bucketIndex] += negativeCounts.At(i)
	}
}

// UpdatePercentile updates values for percentile calculation
func (a *Aggregator) UpdatePercentile(dp pmetric.HistogramDataPoint, percentile float64) {
	// For histogram data points, we can estimate percentiles from buckets
	// This is a simplified implementation
	// In production, you might want to use a more sophisticated algorithm
	
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Add samples to reservoir (simplified - just using sum/count)
	if dp.Count() > 0 {
		avgValue := dp.Sum() / float64(dp.Count())
		a.reservoir = append(a.reservoir, avgValue)
		
		// Keep reservoir size limited
		if len(a.reservoir) > 1000 {
			a.reservoir = a.reservoir[len(a.reservoir)-1000:]
		}
	}
}

// updateMinMax updates min and max values (thread-safe)
func (a *Aggregator) updateMinMax(value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.updateMinMaxLocked(value)
}

// updateMinMaxLocked updates min and max values (must be called with lock held)
func (a *Aggregator) updateMinMaxLocked(value float64) {
	if value < a.min {
		a.min = value
	}
	if value > a.max {
		a.max = value
	}
}

// ExportTo exports the aggregated data to a metric
func (a *Aggregator) ExportTo(metric pmetric.Metric, windowStart, windowEnd time.Time, labels map[string]string) {
	switch a.metricType {
	case pmetric.MetricTypeGauge:
		a.exportGauge(metric, labels)
	case pmetric.MetricTypeSum:
		a.exportSum(metric, windowStart, windowEnd, labels)
	case pmetric.MetricTypeHistogram:
		a.exportHistogram(metric, windowStart, windowEnd, labels)
	case pmetric.MetricTypeExponentialHistogram:
		a.exportExponentialHistogram(metric, windowStart, windowEnd, labels)
	}
}

// exportGauge exports as a gauge metric
func (a *Aggregator) exportGauge(metric pmetric.Metric, labels map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	
	// Use the appropriate value based on what was aggregated
	if a.lastTimestamp != 0 {
		dp.SetDoubleValue(a.lastValue)
		dp.SetTimestamp(a.lastTimestamp)
	} else if a.countForMean > 0 {
		dp.SetDoubleValue(a.sumForMean / float64(a.countForMean))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	} else if a.count > 0 {
		dp.SetDoubleValue(a.sum / float64(a.count))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	}
	
	// Set labels
	for k, v := range labels {
		dp.Attributes().PutStr(k, v)
	}
}

// exportSum exports as a sum metric
func (a *Aggregator) exportSum(metric pmetric.Metric, windowStart, windowEnd time.Time, labels map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Check if this is an UpDownCounter (non-monotonic sum)
	if a.hasUpDownFirstValue {
		// UpDownCounters: always export as gauges (they represent current state)
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetDoubleValue(a.upDownLastValue)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(windowEnd))
		
		// Set labels
		for k, v := range labels {
			dp.Attributes().PutStr(k, v)
		}
		return // Early return for UpDownCounter
	}
	
	// Cumulative export: Export as monotonic sum for Prometheus compatibility
	sum := metric.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sum.SetIsMonotonic(true)
	
	// Export the total accumulated sum for counters
	if a.hasCounterCumulative {
		dp := sum.DataPoints().AppendEmpty()
		// Export the total accumulated sum across all windows
		dp.SetDoubleValue(a.counterTotalSum)
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(windowStart))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(windowEnd))
		
		// Set labels
		for k, v := range labels {
			dp.Attributes().PutStr(k, v)
		}
	} else if a.sum != 0 || a.count > 0 {
		// Fallback for non-counter sum metrics or delta temporality inputs
		dp := sum.DataPoints().AppendEmpty()
		dp.SetDoubleValue(a.sum)
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(windowStart))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(windowEnd))
		
		// Set labels
		for k, v := range labels {
			dp.Attributes().PutStr(k, v)
		}
	}
}

// exportHistogram exports as a histogram metric
func (a *Aggregator) exportHistogram(metric pmetric.Metric, windowStart, windowEnd time.Time, labels map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Cumulative export: Export as histogram for Prometheus compatibility
	hist := metric.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	dp := hist.DataPoints().AppendEmpty()
	// Export the total cumulative values, not the window-specific values
	dp.SetSum(a.histogramTotalSum)
	dp.SetCount(a.histogramTotalCount)
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(windowStart))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(windowEnd))
	
	if a.min != 1e308 {
		dp.SetMin(a.min)
	}
	if a.max != -1e308 {
		dp.SetMax(a.max)
	}
	
	// Sort bucket bounds and set them using total bucket counts
	bounds := make([]float64, 0, len(a.histogramTotalBuckets))
	for bound := range a.histogramTotalBuckets {
		if bound != 1e308 { // Skip infinity bucket
			bounds = append(bounds, bound)
		}
	}
	sort.Float64s(bounds)
	
	dp.ExplicitBounds().FromRaw(bounds)
	
	// Set bucket counts from total buckets
	for _, bound := range bounds {
		dp.BucketCounts().Append(a.histogramTotalBuckets[bound])
	}
	// Always add the infinity bucket at the end
	if infCount, ok := a.histogramTotalBuckets[1e308]; ok {
		dp.BucketCounts().Append(infCount)
	} else {
		dp.BucketCounts().Append(0) // Add 0 if no infinity bucket exists
	}
	
	// Set labels
	for k, v := range labels {
		dp.Attributes().PutStr(k, v)
	}
}

// exportExponentialHistogram exports as an exponential histogram metric
func (a *Aggregator) exportExponentialHistogram(metric pmetric.Metric, windowStart, windowEnd time.Time, labels map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if a.expHistogram == nil {
		return
	}
	
	expHist := metric.SetEmptyExponentialHistogram()
	expHist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	
	dp := expHist.DataPoints().AppendEmpty()
	dp.SetScale(a.expHistogram.scale)
	dp.SetSum(a.expHistogram.sum)
	dp.SetCount(a.expHistogram.count)
	dp.SetZeroCount(a.expHistogram.zeroCount)
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(windowStart))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(windowEnd))
	
	// Set positive buckets
	if len(a.expHistogram.positiveBuckets) > 0 {
		positive := dp.Positive()
		minOffset := int32(1<<31 - 1)
		maxOffset := int32(-1 << 31)
		
		for offset := range a.expHistogram.positiveBuckets {
			if offset < minOffset {
				minOffset = offset
			}
			if offset > maxOffset {
				maxOffset = offset
			}
		}
		
		positive.SetOffset(minOffset)
		for i := minOffset; i <= maxOffset; i++ {
			count := a.expHistogram.positiveBuckets[i]
			positive.BucketCounts().Append(count)
		}
	}
	
	// Set negative buckets
	if len(a.expHistogram.negativeBuckets) > 0 {
		negative := dp.Negative()
		minOffset := int32(1<<31 - 1)
		maxOffset := int32(-1 << 31)
		
		for offset := range a.expHistogram.negativeBuckets {
			if offset < minOffset {
				minOffset = offset
			}
			if offset > maxOffset {
				maxOffset = offset
			}
		}
		
		negative.SetOffset(minOffset)
		for i := minOffset; i <= maxOffset; i++ {
			count := a.expHistogram.negativeBuckets[i]
			negative.BucketCounts().Append(count)
		}
	}
	
	// Set labels
	for k, v := range labels {
		dp.Attributes().PutStr(k, v)
	}
}

// ResetForNewWindow resets window-specific state while preserving cumulative tracking
func (a *Aggregator) ResetForNewWindow() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Reset window-specific state based on metric type
	if a.metricType == pmetric.MetricTypeSum {
		// For counters, reset window delta sum but keep cumulative state
		// Note: The total sum is already accumulated in ComputeDeltaFromCumulative
		a.counterWindowDeltaSum = 0
		a.isCounterReset = false
		// Preserve: counterLastCumulative, hasCounterCumulative, counterTotalSum
		// These need to persist across windows for proper delta computation
	}
	
	// Reset general window-specific fields
	a.sum = 0
	a.count = 0
	a.sumForMean = 0
	a.countForMean = 0
	
	// Reset min/max for new window
	a.min = 1e308  // Max float64
	a.max = -1e308 // Min float64
	
	// For histograms, reset the aggregated buckets for the new window
	// but keep the last seen cumulative values for delta computation
	if a.metricType == pmetric.MetricTypeHistogram {
		// Reset the aggregated bucket counts for the new window
		a.histogramBuckets = make(map[float64]uint64)
		a.histogramSum = 0
		a.histogramCount = 0
		// Preserve: lastHistogramValues (per-source cumulative state)
		// These are needed to compute deltas from cumulative values
	}
	
	// For gauges, keep the last value as it represents the current state
	// Don't reset: lastValue, lastTimestamp
	
	// For UpDownCounters, keep tracking the values
	// Don't reset: upDownFirstValue, upDownLastValue, hasUpDownFirstValue
}

// EstimateMemoryUsage estimates the memory usage of the aggregator
func (a *Aggregator) EstimateMemoryUsage() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Base size
	size := int64(200) // Base struct size
	
	// Histogram buckets
	size += int64(len(a.histogramBuckets) * 16) // map entry overhead
	
	// Exponential histogram
	if a.expHistogram != nil {
		size += int64(100) // Base struct
		size += int64(len(a.expHistogram.positiveBuckets) * 12)
		size += int64(len(a.expHistogram.negativeBuckets) * 12)
	}
	
	// Reservoir
	size += int64(len(a.reservoir) * 8)
	
	return size
}

// Helper function to parse series key
func parseSeriesKey(seriesKey string) (string, map[string]string) {
	// Format: metricName|label1=value1,label2=value2
	
	labels := make(map[string]string)
	
	// Find the pipe separator
	pipeIdx := -1
	for i, ch := range seriesKey {
		if ch == '|' {
			pipeIdx = i
			break
		}
	}
	
	var metricName string
	var labelsPart string
	
	if pipeIdx == -1 {
		// No pipe found, entire key is the metric name
		metricName = seriesKey
		return metricName, labels
	}
	
	// Split metric name and labels
	metricName = seriesKey[:pipeIdx]
	if pipeIdx < len(seriesKey)-1 {
		labelsPart = seriesKey[pipeIdx+1:]
	}
	
	// Parse labels if present
	if labelsPart != "" {
		// Split by comma
		labelPairs := splitLabels(labelsPart)
		for _, pair := range labelPairs {
			// Split by equals
			eqIdx := -1
			for i, ch := range pair {
				if ch == '=' {
					eqIdx = i
					break
				}
			}
			
			if eqIdx > 0 && eqIdx < len(pair)-1 {
				key := pair[:eqIdx]
				value := pair[eqIdx+1:]
				labels[key] = value
			}
		}
	}
	
	return metricName, labels
}

// splitLabels splits label string by comma
func splitLabels(s string) []string {
	var result []string
	var current string
	
	for _, ch := range s {
		if ch == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	
	if current != "" {
		result = append(result, current)
	}
	
	return result
}

// getSourceKey generates a unique key for a histogram source based on its attributes
func getSourceKey(attrs pcommon.Map) string {
	// Create a deterministic key from the attributes
	// This allows us to track cumulative state per unique source
	var key string
	attrs.Range(func(k string, v pcommon.Value) bool {
		if key != "" {
			key += ","
		}
		key += k + "=" + v.AsString()
		return true
	})
	return key
}
