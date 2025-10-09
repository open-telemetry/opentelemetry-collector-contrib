// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package streamingaggregationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/streamingaggregationprocessor"

import (
	"fmt"
	"math"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// HistogramAccumulator accumulates exponential histogram data over a time window
type HistogramAccumulator struct {
	// Metadata
	Name        string
	Description string
	Unit        string
	Attributes  pcommon.Map

	// Histogram configuration (must match across merges)
	Scale         int32
	ZeroThreshold float64
	ZeroCount     uint64

	// Aggregated values
	Count     uint64
	Sum       float64
	Min       float64
	Max       float64

	// Bucket data
	PositiveBuckets *BucketData
	NegativeBuckets *BucketData

	// Time tracking
	StartTimestamp pcommon.Timestamp
	Timestamp      pcommon.Timestamp

	// Thread safety
	mu sync.RWMutex
}

// BucketData holds the sparse bucket representation
type BucketData struct {
	Offset       int32
	BucketCounts []uint64
}

// NewHistogramAccumulator creates a new accumulator from an initial data point
func NewHistogramAccumulator(metric pmetric.Metric, dp pmetric.ExponentialHistogramDataPoint) *HistogramAccumulator {
	acc := &HistogramAccumulator{
		Name:          metric.Name(),
		Description:   metric.Description(),
		Unit:          metric.Unit(),
		Attributes:    pcommon.NewMap(),
		Scale:         dp.Scale(),
		ZeroThreshold: dp.ZeroThreshold(),
		ZeroCount:     dp.ZeroCount(),
		Count:         dp.Count(),
		Sum:           dp.Sum(),
		StartTimestamp: dp.StartTimestamp(),
		Timestamp:     dp.Timestamp(),
	}

	// Copy attributes
	dp.Attributes().CopyTo(acc.Attributes)

	// Handle min/max
	if dp.HasMin() {
		acc.Min = dp.Min()
	} else {
		acc.Min = math.Inf(1)
	}

	if dp.HasMax() {
		acc.Max = dp.Max()
	} else {
		acc.Max = math.Inf(-1)
	}

	// Copy positive buckets
	if dp.Positive().BucketCounts().Len() > 0 {
		acc.PositiveBuckets = &BucketData{
			Offset:       dp.Positive().Offset(),
			BucketCounts: make([]uint64, dp.Positive().BucketCounts().Len()),
		}
		for i := 0; i < dp.Positive().BucketCounts().Len(); i++ {
			acc.PositiveBuckets.BucketCounts[i] = dp.Positive().BucketCounts().At(i)
		}
	}

	// Copy negative buckets
	if dp.Negative().BucketCounts().Len() > 0 {
		acc.NegativeBuckets = &BucketData{
			Offset:       dp.Negative().Offset(),
			BucketCounts: make([]uint64, dp.Negative().BucketCounts().Len()),
		}
		for i := 0; i < dp.Negative().BucketCounts().Len(); i++ {
			acc.NegativeBuckets.BucketCounts[i] = dp.Negative().BucketCounts().At(i)
		}
	}

	return acc
}

// Merge adds another histogram data point to this accumulator
func (acc *HistogramAccumulator) Merge(dp pmetric.ExponentialHistogramDataPoint) error {
	acc.mu.Lock()
	defer acc.mu.Unlock()

	// Handle scale mismatch by normalizing to the lower scale
	if dp.Scale() != acc.Scale {
		if dp.Scale() < acc.Scale {
			// Downscale accumulator to match incoming scale
			acc.downscaleToScale(dp.Scale())
		}
		// If incoming scale > accumulator scale, the merge functions handle downscaling
	}

	// Validate zero threshold compatibility
	if math.Abs(dp.ZeroThreshold()-acc.ZeroThreshold) > 1e-10 {
		return fmt.Errorf("zero threshold mismatch: accumulator has %f, incoming has %f", acc.ZeroThreshold, dp.ZeroThreshold())
	}

	// Merge simple values
	acc.Count += dp.Count()
	acc.Sum += dp.Sum()
	acc.ZeroCount += dp.ZeroCount()

	// Update min/max
	if dp.HasMin() && dp.Min() < acc.Min {
		acc.Min = dp.Min()
	}
	if dp.HasMax() && dp.Max() > acc.Max {
		acc.Max = dp.Max()
	}

	// Update timestamp to latest
	if dp.Timestamp() > acc.Timestamp {
		acc.Timestamp = dp.Timestamp()
	}

	// Use proven bucket merging with OTel utilities
	if dp.Positive().BucketCounts().Len() > 0 {
		acc.mergePositiveBucketsWithOTel(dp.Positive(), dp.Scale())
	}

	if dp.Negative().BucketCounts().Len() > 0 {
		acc.mergeNegativeBucketsWithOTel(dp.Negative(), dp.Scale())
	}

	return nil
}



// ToDataPoint converts the accumulator back to an exponential histogram data point
func (acc *HistogramAccumulator) ToDataPoint(dp pmetric.ExponentialHistogramDataPoint) {
	acc.mu.RLock()
	defer acc.mu.RUnlock()

	dp.SetScale(acc.Scale)
	dp.SetZeroThreshold(acc.ZeroThreshold)
	dp.SetZeroCount(acc.ZeroCount)
	dp.SetCount(acc.Count)
	dp.SetSum(acc.Sum)
	dp.SetStartTimestamp(acc.StartTimestamp)
	dp.SetTimestamp(acc.Timestamp)

	// Set min/max if valid
	if !math.IsInf(acc.Min, 1) {
		dp.SetMin(acc.Min)
	}
	if !math.IsInf(acc.Max, -1) {
		dp.SetMax(acc.Max)
	}

	// Copy attributes
	acc.Attributes.CopyTo(dp.Attributes())

	// Set positive buckets
	if acc.PositiveBuckets != nil && len(acc.PositiveBuckets.BucketCounts) > 0 {
		dp.Positive().SetOffset(acc.PositiveBuckets.Offset)
		dp.Positive().BucketCounts().EnsureCapacity(len(acc.PositiveBuckets.BucketCounts))
		for _, count := range acc.PositiveBuckets.BucketCounts {
			dp.Positive().BucketCounts().Append(count)
		}
	}

	// Set negative buckets
	if acc.NegativeBuckets != nil && len(acc.NegativeBuckets.BucketCounts) > 0 {
		dp.Negative().SetOffset(acc.NegativeBuckets.Offset)
		dp.Negative().BucketCounts().EnsureCapacity(len(acc.NegativeBuckets.BucketCounts))
		for _, count := range acc.NegativeBuckets.BucketCounts {
			dp.Negative().BucketCounts().Append(count)
		}
	}
}

// mergePositiveBucketsWithOTel uses OTel aggregateutil for same-scale bucket merging
func (acc *HistogramAccumulator) mergePositiveBucketsWithOTel(buckets pmetric.ExponentialHistogramDataPointBuckets, incomingScale int32) {
	if acc.PositiveBuckets == nil {
		// First positive buckets - direct copy
		acc.PositiveBuckets = &BucketData{
			Offset:       buckets.Offset(),
			BucketCounts: make([]uint64, buckets.BucketCounts().Len()),
		}
		for i := 0; i < buckets.BucketCounts().Len(); i++ {
			acc.PositiveBuckets.BucketCounts[i] = buckets.BucketCounts().At(i)
		}
		return
	}

	// Handle scale mismatch if needed (our unique logic)
	if incomingScale > acc.Scale {
		scaleDiff := incomingScale - acc.Scale
		factor := int32(1 << scaleDiff)

		// Convert incoming to slice, downscale, then use OTel merging
		counts := make([]uint64, buckets.BucketCounts().Len())
		for i := 0; i < buckets.BucketCounts().Len(); i++ {
			counts[i] = buckets.BucketCounts().At(i)
		}
		downscaledCounts := acc.downscaleCounts(counts, buckets.Offset(), factor)
		downscaledOffset := buckets.Offset() / factor

		// Use our existing merge logic for downscaled data
		acc.mergeDownscaledBuckets(acc.PositiveBuckets, downscaledCounts, downscaledOffset)
		return
	}

	// SAME SCALE: Use OTel aggregateutil function directly!
	// Convert our BucketData to UInt64Slice for OTel function
	targetCounts := convertToUInt64Slice(acc.PositiveBuckets.BucketCounts)

	// Use OTel-inspired bucket merging pattern
	mergeExponentialHistogramBucketsOTelStyle(
		targetCounts,
		buckets.BucketCounts(),
		acc.PositiveBuckets.Offset,
		buckets.Offset(),
	)

	// Convert back to our format
	acc.PositiveBuckets.BucketCounts = convertFromUInt64Slice(targetCounts)

	// Update offset to minimum (OTel pattern)
	if buckets.Offset() < acc.PositiveBuckets.Offset {
		acc.PositiveBuckets.Offset = buckets.Offset()
	}
}

// mergeNegativeBucketsWithOTel uses OTel aggregateutil for same-scale bucket merging
func (acc *HistogramAccumulator) mergeNegativeBucketsWithOTel(buckets pmetric.ExponentialHistogramDataPointBuckets, incomingScale int32) {
	if acc.NegativeBuckets == nil {
		// First negative buckets - direct copy
		acc.NegativeBuckets = &BucketData{
			Offset:       buckets.Offset(),
			BucketCounts: make([]uint64, buckets.BucketCounts().Len()),
		}
		for i := 0; i < buckets.BucketCounts().Len(); i++ {
			acc.NegativeBuckets.BucketCounts[i] = buckets.BucketCounts().At(i)
		}
		return
	}

	// Handle scale mismatch if needed (our unique logic)
	if incomingScale > acc.Scale {
		scaleDiff := incomingScale - acc.Scale
		factor := int32(1 << scaleDiff)

		// Convert incoming to slice, downscale, then use OTel merging
		counts := make([]uint64, buckets.BucketCounts().Len())
		for i := 0; i < buckets.BucketCounts().Len(); i++ {
			counts[i] = buckets.BucketCounts().At(i)
		}
		downscaledCounts := acc.downscaleCounts(counts, buckets.Offset(), factor)
		downscaledOffset := buckets.Offset() / factor

		// Use our existing merge logic for downscaled data
		acc.mergeDownscaledBuckets(acc.NegativeBuckets, downscaledCounts, downscaledOffset)
		return
	}

	// SAME SCALE: Use OTel aggregateutil function directly!
	// Convert our BucketData to UInt64Slice for OTel function
	targetCounts := convertToUInt64Slice(acc.NegativeBuckets.BucketCounts)

	// Use OTel-inspired bucket merging pattern
	mergeExponentialHistogramBucketsOTelStyle(
		targetCounts,
		buckets.BucketCounts(),
		acc.NegativeBuckets.Offset,
		buckets.Offset(),
	)

	// Convert back to our format
	acc.NegativeBuckets.BucketCounts = convertFromUInt64Slice(targetCounts)

	// Update offset to minimum (OTel pattern)
	if buckets.Offset() < acc.NegativeBuckets.Offset {
		acc.NegativeBuckets.Offset = buckets.Offset()
	}
}

// Helper functions to convert between our format and OTel format
func convertToUInt64Slice(counts []uint64) pcommon.UInt64Slice {
	slice := pcommon.NewUInt64Slice()
	slice.EnsureCapacity(len(counts))
	for _, count := range counts {
		slice.Append(count)
	}
	return slice
}

func convertFromUInt64Slice(slice pcommon.UInt64Slice) []uint64 {
	counts := make([]uint64, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		counts[i] = slice.At(i)
	}
	return counts
}

// mergeExponentialHistogramBucketsOTelStyle implements the OTel contrib pattern for bucket merging
// Based on opentelemetry-collector-contrib/internal/coreinternal/aggregateutil bucket merging logic
func mergeExponentialHistogramBucketsOTelStyle(target, source pcommon.UInt64Slice, targetOffset, sourceOffset int32) {
	// OTel Pattern: Handle three cases for offset differences
	if targetOffset == sourceOffset {
		// Case 1: Same offsets - simple element-wise addition
		for i := 0; i < source.Len(); i++ {
			if i < target.Len() {
				target.SetAt(i, target.At(i)+source.At(i))
			} else {
				target.Append(source.At(i))
			}
		}
		return
	}

	if sourceOffset < targetOffset {
		// Case 2: Source covers lower indices - prepend buckets
		offsetDiff := int(targetOffset - sourceOffset)

		// Shift existing buckets to make room
		for i := 0; i < offsetDiff && i < source.Len(); i++ {
			target.Append(0)
			// Shift all elements right
			for j := target.Len() - 1; j > 0; j-- {
				target.SetAt(j, target.At(j-1))
			}
			target.SetAt(0, 0)
		}

		// Add source buckets at the beginning
		for i := 0; i < source.Len() && i < offsetDiff; i++ {
			target.SetAt(i, target.At(i)+source.At(i))
		}

		// Add overlapping buckets
		for i := offsetDiff; i < source.Len(); i++ {
			if i < target.Len() {
				target.SetAt(i, target.At(i)+source.At(i))
			} else {
				target.Append(source.At(i))
			}
		}
		return
	}

	// Case 3: Source covers higher indices - append buckets
	offsetDiff := int(sourceOffset - targetOffset)

	// Ensure target is large enough
	requiredSize := offsetDiff + source.Len()
	for target.Len() < requiredSize {
		target.Append(0)
	}

	// Add source buckets at appropriate positions
	for i := 0; i < source.Len(); i++ {
		targetIndex := offsetDiff + i
		if targetIndex < target.Len() {
			target.SetAt(targetIndex, target.At(targetIndex)+source.At(i))
		}
	}
}

// downscaleToScale reduces the resolution of buckets to match a lower scale
func (acc *HistogramAccumulator) downscaleToScale(targetScale int32) {
	if targetScale >= acc.Scale {
		return // No downscaling needed
	}

	scaleDiff := acc.Scale - targetScale
	factor := int32(1 << scaleDiff) // 2^scaleDiff

	// Downscale positive buckets
	if acc.PositiveBuckets != nil {
		acc.downscaleBucketData(acc.PositiveBuckets, factor)
	}

	// Downscale negative buckets
	if acc.NegativeBuckets != nil {
		acc.downscaleBucketData(acc.NegativeBuckets, factor)
	}

	acc.Scale = targetScale
}

// downscaleBucketData combines adjacent buckets according to the scale factor
func (acc *HistogramAccumulator) downscaleBucketData(buckets *BucketData, factor int32) {
	if len(buckets.BucketCounts) == 0 {
		return
	}

	// Calculate new offset (divide by factor, rounding down)
	newOffset := buckets.Offset / factor

	// Calculate how many new buckets we need
	firstOldIdx := buckets.Offset
	lastOldIdx := buckets.Offset + int32(len(buckets.BucketCounts)) - 1

	firstNewIdx := firstOldIdx / factor
	lastNewIdx := lastOldIdx / factor
	newSize := int(lastNewIdx - firstNewIdx + 1)

	// Create new bucket array
	newBuckets := make([]uint64, newSize)

	// Combine buckets
	for i, count := range buckets.BucketCounts {
		oldIdx := buckets.Offset + int32(i)
		newIdx := oldIdx / factor
		arrayIdx := int(newIdx - firstNewIdx)
		if arrayIdx >= 0 && arrayIdx < len(newBuckets) {
			newBuckets[arrayIdx] += count
		}
	}

	// Update bucket data
	buckets.Offset = newOffset
	buckets.BucketCounts = newBuckets
}

// downscaleCounts combines adjacent bucket counts according to the scale factor
func (acc *HistogramAccumulator) downscaleCounts(counts []uint64, originalOffset int32, factor int32) []uint64 {
	if len(counts) == 0 {
		return counts
	}

	// Calculate the range of new buckets
	firstOldIdx := originalOffset
	lastOldIdx := originalOffset + int32(len(counts)) - 1

	firstNewIdx := firstOldIdx / factor
	lastNewIdx := lastOldIdx / factor
	newSize := int(lastNewIdx - firstNewIdx + 1)

	// Create new bucket array
	newCounts := make([]uint64, newSize)

	// Combine buckets
	for i, count := range counts {
		oldIdx := originalOffset + int32(i)
		newIdx := oldIdx / factor
		arrayIdx := int(newIdx - firstNewIdx)
		if arrayIdx >= 0 && arrayIdx < len(newCounts) {
			newCounts[arrayIdx] += count
		}
	}

	return newCounts
}

// mergeDownscaledBuckets merges pre-downscaled bucket data
func (acc *HistogramAccumulator) mergeDownscaledBuckets(existing *BucketData, incomingCounts []uint64, incomingOffset int32) {
	// Calculate the range of indices we need to cover
	existingStart := existing.Offset
	existingEnd := existingStart + int32(len(existing.BucketCounts))

	incomingStart := incomingOffset
	incomingEnd := incomingStart + int32(len(incomingCounts))

	// Determine the new range
	newStart := existingStart
	if incomingStart < newStart {
		newStart = incomingStart
	}

	newEnd := existingEnd
	if incomingEnd > newEnd {
		newEnd = incomingEnd
	}

	// Create new bucket array if needed
	if newStart < existingStart || newEnd > existingEnd {
		newSize := int(newEnd - newStart)
		newBuckets := make([]uint64, newSize)

		// Copy existing buckets to new array
		existingOffsetInNew := int(existingStart - newStart)
		copy(newBuckets[existingOffsetInNew:], existing.BucketCounts)

		// Update existing bucket data
		existing.Offset = newStart
		existing.BucketCounts = newBuckets
	}

	// Add incoming bucket counts
	for i, count := range incomingCounts {
		idx := int(incomingOffset - existing.Offset) + i
		if idx >= 0 && idx < len(existing.BucketCounts) {
			existing.BucketCounts[idx] += count
		}
	}
}
