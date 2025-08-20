// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracking // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"

import "go.opentelemetry.io/collector/pdata/pcommon"

type ValuePoint struct {
	ObservedTimestamp pcommon.Timestamp
	FloatValue        float64
	IntValue          int64
	HistogramValue    *HistogramPoint
	ExpHistogramValue *ExpHistogramPoint
}

type HistogramPoint struct {
	Count   uint64
	Sum     float64
	Buckets []uint64
}

func (point *HistogramPoint) Clone() HistogramPoint {
	bucketValues := make([]uint64, len(point.Buckets))
	copy(bucketValues, point.Buckets)

	return HistogramPoint{
		Count:   point.Count,
		Sum:     point.Sum,
		Buckets: bucketValues,
	}
}

type ExpHistogramPoint struct {
	Scale               int32
	NegativeOffsetIndex int32
	PositiveOffsetIndex int32
	ZeroThreshold       float64
	Count               uint64
	Sum                 float64
	ZeroCount           uint64
	PosBuckets          []uint64
	NegBuckets          []uint64
}

func (point *ExpHistogramPoint) Clone() ExpHistogramPoint {
	posBucketValues := make([]uint64, len(point.PosBuckets))
	copy(posBucketValues, point.PosBuckets)
	negBucketValues := make([]uint64, len(point.NegBuckets))
	copy(negBucketValues, point.NegBuckets)

	return ExpHistogramPoint{
		Count:               point.Count,
		Sum:                 point.Sum,
		Scale:               point.Scale,
		ZeroCount:           point.ZeroCount,
		ZeroThreshold:       point.ZeroThreshold,
		PositiveOffsetIndex: point.PositiveOffsetIndex,
		NegativeOffsetIndex: point.NegativeOffsetIndex,
		PosBuckets:          posBucketValues,
		NegBuckets:          negBucketValues,
	}
}

func (point *ExpHistogramPoint) isCompatible(other *ExpHistogramPoint) bool {
	// NOTE: this constraint could be relaxed but requires changes to tracker to handle perfect subsetting and handling
	// index offsetting
	return point.Scale == other.Scale &&
		point.PositiveOffsetIndex == other.PositiveOffsetIndex &&
		point.NegativeOffsetIndex == other.NegativeOffsetIndex &&
		point.ZeroThreshold == other.ZeroThreshold
}

func (point *ExpHistogramPoint) isReset(prev *ExpHistogramPoint) bool {
	// A drop in count or decrease in number of buckets indicates a reset
	return point.Count < prev.Count ||
		point.ZeroCount < prev.ZeroCount ||
		len(point.PosBuckets) < len(prev.PosBuckets) ||
		len(point.NegBuckets) < len(prev.NegBuckets)
}
