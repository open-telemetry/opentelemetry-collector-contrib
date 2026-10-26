// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package exphistogram contains utility functions for exponential histogram conversions.
package exphistogram // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/exphistogram"

import (
	"math"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const MaxScale = 38

var log2E = 1 / math.Log(2)

// LowerBoundary calculates the lower boundary given index and scale.
// Adopted from https://opentelemetry.io/docs/specs/otel/metrics/data-model/#producer-expectations
func LowerBoundary(index, scale int) float64 {
	if scale <= 0 {
		return LowerBoundaryNegativeScale(index, scale)
	}
	// Use this form in case the equation above computes +Inf
	// as the lower boundary of a valid bucket.
	inverseFactor := math.Ldexp(math.Ln2, -scale)
	return 2.0 * math.Exp(float64(index-(1<<scale))*inverseFactor)
}

// LowerBoundaryNegativeScale calculates the lower boundary for scale <= 0.
// Adopted from https://opentelemetry.io/docs/specs/otel/metrics/data-model/#producer-expectations
func LowerBoundaryNegativeScale(index, scale int) float64 {
	return math.Ldexp(1, index<<-scale)
}

// bucketValueFunc computes the representative value for a bucket given its
// lower and upper boundaries.
type bucketValueFunc func(lb, ub float64) float64

// midpointBucketValue returns the midpoint (centroid) of the bucket boundaries.
func midpointBucketValue(lb, ub float64) float64 {
	return lb + (ub-lb)/2
}

// rawBucketValue returns the upper boundary of the bucket directly.
func rawBucketValue(_, ub float64) float64 {
	return ub
}

// ToRaw converts an OTLP exponential histogram data point to counts and
// boundary values without any midpoint approximation. Each bucket's
// representative value is the upper boundary of the bucket.
func ToRaw(dp pmetric.ExponentialHistogramDataPoint) (counts []int64, values []float64) {
	return toHistogram(dp, rawBucketValue)
}

// ToTDigest converts an OTLP exponential histogram data point to T-Digest counts and mean centroid values.
func ToTDigest(dp pmetric.ExponentialHistogramDataPoint) (counts []int64, values []float64) {
	return toHistogram(dp, midpointBucketValue)
}

// ToNativeExponentialHistogram returns the native exponential histogram
func ToNativeExponentialHistogram(dp pmetric.ExponentialHistogramDataPoint) pcommon.Value {
	return toNativeExponentialHistogram(dp)
}

func ExplicitToNativeExponentialHistogram(dp pmetric.HistogramDataPoint, temporality pmetric.AggregationTemporality) pcommon.Value {
	return explicitToNativeExponentialHistogram(dp, temporality)
}

func toHistogram(dp pmetric.ExponentialHistogramDataPoint, valueFn bucketValueFunc) (counts []int64, values []float64) {
	scale := int(dp.Scale())

	offset := int(dp.Negative().Offset())
	bucketCounts := dp.Negative().BucketCounts()
	for i := bucketCounts.Len() - 1; i >= 0; i-- {
		count := bucketCounts.At(i)
		if count == 0 {
			continue
		}
		lb := -LowerBoundary(offset+i+1, scale)
		ub := -LowerBoundary(offset+i, scale)
		counts = append(counts, safeUint64ToInt64(count))
		values = append(values, valueFn(lb, ub))
	}

	if zeroCount := dp.ZeroCount(); zeroCount != 0 {
		counts = append(counts, safeUint64ToInt64(zeroCount))
		values = append(values, 0)
	}

	offset = int(dp.Positive().Offset())
	for i, count := range dp.Positive().BucketCounts().All() {
		if count == 0 {
			continue
		}
		lb := LowerBoundary(offset+i, scale)
		ub := LowerBoundary(offset+i+1, scale)
		counts = append(counts, safeUint64ToInt64(count))
		values = append(values, valueFn(lb, ub))
	}
	return counts, values
}

func toNativeExponentialHistogram(dp pmetric.ExponentialHistogramDataPoint) pcommon.Value {
	vm := pcommon.NewValueMap()
	m := vm.Map()

	m.PutInt("scale", int64(dp.Scale()))

	if dp.ZeroCount() > 0 {
		zeroMap := m.PutEmptyMap("zero")
		zeroMap.PutInt("count", safeUint64ToInt64(dp.ZeroCount()))
		if dp.ZeroThreshold() > 0 {
			zeroMap.PutDouble("threshold", dp.ZeroThreshold())
		}
	}

	putExponentialBuckets(m, "negative", dp.Negative())
	putExponentialBuckets(m, "positive", dp.Positive())

	if dp.HasSum() {
		m.PutDouble("sum", dp.Sum())
	}
	if dp.HasMin() {
		m.PutDouble("min", dp.Min())
	}
	if dp.HasMax() {
		m.PutDouble("max", dp.Max())
	}
	return vm
}

func computeIndex(value float64, scale int) int64 {
	abs := math.Abs(value)
	return int64(math.Ceil(math.Ldexp(math.Log(abs)*log2E, scale))) - 1
}

type expBucket struct {
	index int64
	count int64
}

type bucketBuffer struct {
	negative  []expBucket
	positive  []expBucket
	zeroCount int64
}

func (b *bucketBuffer) append(center float64, count int64) {
	if count == 0 {
		return
	}
	switch {
	case center < 0:
		addOrMerge(&b.negative, center, count)
	case center > 0:
		addOrMerge(&b.positive, center, count)
	default:
		b.zeroCount += count
	}
}

func addOrMerge(buckets *[]expBucket, center float64, count int64) {
	index := computeIndex(center, MaxScale)
	if n := len(*buckets); n > 0 && (*buckets)[n-1].index == index {
		(*buckets)[n-1].count += count
		return
	}
	*buckets = append(*buckets, expBucket{index: index, count: count})
}

func (b *bucketBuffer) writeBuckets(m pcommon.Map) {
	if b.zeroCount > 0 {
		m.PutEmptyMap("zero").PutInt("count", b.zeroCount)
	}
	if len(b.negative) > 0 {
		nm := m.PutEmptyMap("negative")
		idx := nm.PutEmptySlice("indices")
		cnt := nm.PutEmptySlice("counts")

		for i := range slices.Backward(b.negative) {
			idx.AppendEmpty().SetInt(b.negative[i].index)
			cnt.AppendEmpty().SetInt(b.negative[i].count)
		}
	}
	if len(b.positive) > 0 {
		pm := m.PutEmptyMap("positive")
		idx := pm.PutEmptySlice("indices")
		cnt := pm.PutEmptySlice("counts")

		for i := range b.positive {
			idx.AppendEmpty().SetInt(b.positive[i].index)
			cnt.AppendEmpty().SetInt(b.positive[i].count)
		}
	}
}

func explicitToNativeExponentialHistogram(dp pmetric.HistogramDataPoint, temporality pmetric.AggregationTemporality) pcommon.Value {
	vm := pcommon.NewValueMap()
	m := vm.Map()
	m.PutInt("scale", MaxScale)

	var buf bucketBuffer
	bucketCounts := dp.BucketCounts()
	size := bucketCounts.Len()
	cumulative := temporality == pmetric.AggregationTemporalityCumulative

	if size > 0 && dp.ExplicitBounds().Len() > 0 {
		minHandled := false
		bounds := dp.ExplicitBounds()

		lastPopulatedBucket := size - 1
		for lastPopulatedBucket >= 0 && bucketCounts.At(lastPopulatedBucket) == 0 {
			lastPopulatedBucket--
		}

		for i := range size {
			count := safeUint64ToInt64(bucketCounts.At(i))
			if count > 0 {
				centroid := explicitBucketCentroid(size, bounds, i)
				injectMaxBucketAfterIteration := false

				if !cumulative {
					if dp.HasMin() && !minHandled {
						if centroid > dp.Min() && count > 1 {
							// min is smaller than the centroid and not the only value in the bucket, we inject a separate, single value
							// bucket for it
							buf.append(dp.Min(), 1)
							count--
						} else {
							centroid = dp.Min()
						}
						minHandled = true
					}
					if dp.HasMax() && lastPopulatedBucket == i {
						if centroid < dp.Max() && count > 1 {
							// max is bigger than the centroid and not the only value in the bucket, we inject a separate, single value
							// bucket for it
							injectMaxBucketAfterIteration = true
							count--
						} else {
							centroid = dp.Max()
						}
					}
				}
				buf.append(centroid, count)
				if injectMaxBucketAfterIteration {
					buf.append(dp.Max(), 1)
				}
			}
		}
		buf.writeBuckets(m)
		writeSummaryStatistics(dp, m)
	} else if dp.Count() > 0 {
		value := 0.0
		if dp.HasSum() && !cumulative {
			value = dp.Sum() / float64(dp.Count())
		}
		buf.append(value, safeUint64ToInt64(dp.Count()))

		buf.writeBuckets(m)
		writeSummaryStatistics(dp, m)
	}
	return vm
}

func writeSummaryStatistics(dp pmetric.HistogramDataPoint, m pcommon.Map) {
	if dp.HasSum() {
		m.PutDouble("sum", dp.Sum())
	}
	if dp.HasMin() {
		m.PutDouble("min", dp.Min())
	}
	if dp.HasMax() {
		m.PutDouble("max", dp.Max())
	}
}

func explicitBucketCentroid(bucketCounts int, bounds pcommon.Float64Slice, bucketIndex int) float64 {
	switch bucketIndex {
	case 0:
		v := bounds.At(bucketIndex)
		if v > 0 {
			v /= 2
		}
		return v
	case bucketCounts - 1:
		return bounds.At(bucketIndex - 1)
	default:
		lo, hi := bounds.At(bucketIndex-1), bounds.At(bucketIndex)
		return lo + (hi-lo)/2.0
	}
}

func putExponentialBuckets(m pcommon.Map, key string, b pmetric.ExponentialHistogramDataPointBuckets) {
	counts := b.BucketCounts()
	offset := int64(b.Offset())

	var n int
	for i := 0; i < counts.Len(); i++ {
		if counts.At(i) != 0 {
			n++
		}
	}
	if n == 0 {
		return
	}

	bm := m.PutEmptyMap(key)
	indices := bm.PutEmptySlice("indices")
	indices.EnsureCapacity(n)
	outCounts := bm.PutEmptySlice("counts")
	outCounts.EnsureCapacity(n)

	for i := 0; i < counts.Len(); i++ {
		c := counts.At(i)
		if c == 0 {
			continue
		}
		indices.AppendEmpty().SetInt(offset + int64(i))
		outCounts.AppendEmpty().SetInt(safeUint64ToInt64(c))
	}
}

func safeUint64ToInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v) //nolint:goset // overflow checked
}
