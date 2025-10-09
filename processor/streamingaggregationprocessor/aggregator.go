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
	// For exponential histogram - using well-tested accumulator from native-histogram-otel
	expHistogramAccumulator *HistogramAccumulator

	// ===== THREAD SAFETY =====
	mu sync.Mutex // Protects all fields for thread-safe access
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
		metricType:            metricType,
		min:                   1e308,  // Max float64
		max:                   -1e308, // Min float64
		histogramBuckets:      make(map[float64]uint64),
		histogramTotalBuckets: make(map[float64]uint64),
		lastHistogramValues:   make(map[string]*histogramCumulativeState),
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

// ProcessDeltaCounter processes delta counter values
func (a *Aggregator) ProcessDeltaCounter(value float64) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.hasCounterCumulative {
		// First delta value initializes the counter
		a.counterTotalSum = value
		a.hasCounterCumulative = true
	} else {
		// Add delta to total sum
		a.counterTotalSum += value
	}
	a.counterWindowDeltaSum += value
	return nil
}

// UpdateMean updates values for mean calculation
func (a *Aggregator) UpdateMean(value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sumForMean += value
	a.countForMean++
	a.updateMinMaxLocked(value)
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

// updateMinMaxLocked updates min and max values (must be called with lock held)
func (a *Aggregator) updateMinMaxLocked(value float64) {
	if value < a.min {
		a.min = value
	}
	if value > a.max {
		a.max = value
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

	// For histograms, reset the window-specific values for the new window
	// but keep both the total accumulated values and per-source cumulative state
	if a.metricType == pmetric.MetricTypeHistogram {
		// Reset the window-specific bucket counts for the new window
		a.histogramBuckets = make(map[float64]uint64)
		a.histogramSum = 0
		a.histogramCount = 0
		// Preserve: histogramTotalBuckets, histogramTotalSum, histogramTotalCount (for cumulative export)
		// Preserve: lastHistogramValues (per-source cumulative state for delta computation)
	}

	// For exponential histograms, reset the accumulator for true windowed behavior
	// This matches native-histogram-otel behavior where each window is independent
	if a.metricType == pmetric.MetricTypeExponentialHistogram {
		a.expHistogramAccumulator = nil
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

	// Exponential histogram accumulator
	if a.expHistogramAccumulator != nil {
		size += int64(200) // Base accumulator struct
		if a.expHistogramAccumulator.PositiveBuckets != nil {
			size += int64(len(a.expHistogramAccumulator.PositiveBuckets.BucketCounts) * 8)
		}
		if a.expHistogramAccumulator.NegativeBuckets != nil {
			size += int64(len(a.expHistogramAccumulator.NegativeBuckets.BucketCounts) * 8)
		}
	}

	return size
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
		// Track per-endpoint cumulative state but aggregate the deltas
		sourceKey := getSourceKey(dp.Attributes())

		// Get or create the cumulative state for this specific source (endpoint)
		sourceState, exists := a.lastHistogramValues[sourceKey]
		currentTime := time.Now()

		// Check for stale data if threshold is provided and state exists
		gapDetected := false
		if exists && staleThreshold > 0 {
			lastTime := sourceState.lastTimestamp.AsTime()
			if !lastTime.IsZero() {
				timeSinceLastData := currentTime.Sub(lastTime)
				if timeSinceLastData > staleThreshold {
					// Gap detected for this source - check if we need to reset everything
					fmt.Printf("GAP DETECTED: Source %s has %v gap (threshold: %v)\\n", sourceKey, timeSinceLastData, staleThreshold)
					gapDetected = true
					exists = false
				}
			}
		}

		// If any gap was detected, reset all histogram totals once
		if gapDetected {
			fmt.Printf("GAP DETECTED: Resetting all histogram totals due to stale data\\n")
			a.histogramTotalSum = 0
			a.histogramTotalCount = 0
			a.histogramTotalBuckets = make(map[float64]uint64)
			// Clear all source states - start completely fresh
			a.lastHistogramValues = make(map[string]*histogramCumulativeState)
		}

		if !exists {
			sourceState = &histogramCumulativeState{
				lastBuckets:   make(map[float64]uint64),
				lastTimestamp: pcommon.NewTimestampFromTime(currentTime),
			}
			a.lastHistogramValues[sourceKey] = sourceState
		}

		// Compute deltas from the last seen values for this specific source
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

		// Update window values with the deltas (this aggregates across all endpoints)
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

// MergeExponentialHistogram merges an exponential histogram using the proven accumulator
func (a *Aggregator) MergeExponentialHistogram(metric pmetric.Metric, dp pmetric.ExponentialHistogramDataPoint) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Initialize accumulator on first data point
	if a.expHistogramAccumulator == nil {
		a.expHistogramAccumulator = NewHistogramAccumulator(metric, dp)
		return nil
	}

	// Use the proven merge logic with scale normalization
	return a.expHistogramAccumulator.Merge(dp)
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

	// For gauges, only export if we have a valid lastValue with timestamp
	// This prevents computing ratios that cause Prometheus to add _ratio suffix
	if a.lastTimestamp == 0 {
		// No valid gauge data yet - don't export anything
		// This prevents empty or ratio-based gauge exports
		return
	}

	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()

	// Use the last value (proper gauge semantics)
	dp.SetDoubleValue(a.lastValue)
	dp.SetTimestamp(a.lastTimestamp)

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
	// Export the total cumulative values (for Prometheus compatibility)
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

// exportExponentialHistogram exports using the accumulator's ToDataPoint method
func (a *Aggregator) exportExponentialHistogram(metric pmetric.Metric, windowStart, windowEnd time.Time, labels map[string]string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.expHistogramAccumulator == nil {
		return
	}

	expHist := metric.SetEmptyExponentialHistogram()
	expHist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	dp := expHist.DataPoints().AppendEmpty()

	// Use the proven ToDataPoint logic from the accumulator
	a.expHistogramAccumulator.ToDataPoint(dp)

	// Override timestamps for window boundaries
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(windowStart))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(windowEnd))

	// Set labels (streaming processor drops all incoming labels, adds back if configured)
	for k, v := range labels {
		dp.Attributes().PutStr(k, v)
	}
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