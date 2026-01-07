// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracking // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"

import (
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type ValuePoint struct {
	ObservedTimestamp         pcommon.Timestamp
	FloatValue                float64
	IntValue                  int64
	HistogramValue            *HistogramPoint
	ExponentialHistogramValue *ExponentialHistogramPoint
}

type HistogramPoint struct {
	Count        uint64
	Sum          float64
	BucketBounds []float64
	BucketCounts []uint64
}

func (point *HistogramPoint) Clone() HistogramPoint {
	return HistogramPoint{
		Count:        point.Count,
		Sum:          point.Sum,
		BucketBounds: slices.Clone(point.BucketBounds),
		BucketCounts: slices.Clone(point.BucketCounts),
	}
}

type ExponentialBuckets struct {
	Offset       int32
	BucketCounts []uint64
}

// Coarsen reduces an exponential histogram's scale by bitsLost,
// which amounts to dividing bucket indices by 2**bitsLost and merging buckets with the same resulting index.
func (buckets *ExponentialBuckets) Coarsen(bitsLost int32) (out ExponentialBuckets) {
	out.Offset = buckets.Offset >> bitsLost
	if len(buckets.BucketCounts) > 0 {
		maxBucket := buckets.Offset + int32(len(buckets.BucketCounts)) - 1
		out.BucketCounts = make([]uint64, (maxBucket>>bitsLost)-out.Offset+1)
		for index, bucketCount := range buckets.BucketCounts {
			newIndex := ((buckets.Offset + int32(index)) >> bitsLost) - out.Offset
			out.BucketCounts[newIndex] += bucketCount
		}
	}
	return out
}

// TrimZeros removes buckets below a given bucket index, returning the sum of their counts.
// This is used when increasing the zero threshold: the removed count will be added to the zero count.
func (buckets *ExponentialBuckets) TrimZeros(thresholdBucket int32) uint64 {
	thresholdIndex := thresholdBucket - buckets.Offset
	if thresholdIndex < 0 {
		return 0
	}
	trimmed := min(int(thresholdIndex)+1, len(buckets.BucketCounts))
	var zeroCount uint64
	for _, bucketCount := range buckets.BucketCounts[:trimmed] {
		zeroCount += bucketCount
	}
	buckets.BucketCounts = buckets.BucketCounts[trimmed:]
	buckets.Offset += int32(trimmed)
	return zeroCount
}

// Diff computes the delta between two sets of buckets with the same scale.
func (buckets *ExponentialBuckets) Diff(old *ExponentialBuckets) (out ExponentialBuckets) {
	for index, bucketCount := range buckets.BucketCounts {
		if bucketCount == 0 {
			continue
		}
		bucket := int(buckets.Offset) + index
		oldIndex := bucket - int(old.Offset)
		oldCount := uint64(0)
		if oldIndex >= 0 && oldIndex < len(old.BucketCounts) {
			oldCount = old.BucketCounts[oldIndex]
		}
		if bucketCount <= oldCount {
			// bucketCount < oldCount is a monotonicity error, we'll consider the diff to be zero as fallback
			continue
		}
		diff := bucketCount - oldCount
		if out.BucketCounts == nil {
			out.Offset = int32(bucket)
		} else {
			zeros := bucket - int(out.Offset) + len(out.BucketCounts)
			out.BucketCounts = append(out.BucketCounts, make([]uint64, zeros)...)
		}
		out.BucketCounts = append(out.BucketCounts, diff)
	}
	return out
}

type ExponentialHistogramPoint struct {
	Count         uint64
	Sum           float64
	ZeroCount     uint64
	ZeroThreshold float64
	Scale         int32
	Positive      ExponentialBuckets
	Negative      ExponentialBuckets
}

func (point *ExponentialHistogramPoint) Clone() ExponentialHistogramPoint {
	newPoint := *point
	newPoint.Positive.BucketCounts = slices.Clone(newPoint.Positive.BucketCounts)
	newPoint.Negative.BucketCounts = slices.Clone(newPoint.Negative.BucketCounts)
	return newPoint
}
