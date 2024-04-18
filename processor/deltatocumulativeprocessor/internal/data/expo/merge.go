// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expo // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Merge combines the counts of buckets a and b into a.
// Both buckets MUST be of same scale
func Merge(arel, brel pmetric.ExponentialHistogramDataPointBuckets) {
	a, b := Abs(arel), Abs(brel)

	lo := min(a.Lower(), b.Lower())
	up := max(a.Upper(), b.Upper())

	size := up - lo

	counts := pcommon.NewUInt64Slice()
	counts.Append(make([]uint64, size-counts.Len())...)

	for i := 0; i < counts.Len(); i++ {
		counts.SetAt(i, a.Abs(lo+i)+b.Abs(lo+i))
	}

	a.SetOffset(int32(lo))
	counts.MoveTo(a.BucketCounts())
}

func Abs(buckets buckets) Absolute {
	return Absolute{buckets: buckets}
}

type buckets = pmetric.ExponentialHistogramDataPointBuckets

// Absolute addresses bucket counts using an absolute scale, such that the
// following holds true:
//
//	for i := range counts: Scale(…).Idx(counts[i]) == i
//
// It spans from [[Absolute.Lower]:[Absolute.Upper]]
type Absolute struct {
	buckets
}

// Abs returns the value at absolute index 'at'. The following holds true:
//
//	Scale(…).Idx(At(i)) == i
func (a Absolute) Abs(at int) uint64 {
	if i, ok := a.idx(at); ok {
		return a.BucketCounts().At(i)
	}
	return 0
}

// Upper returns the minimal index outside the set, such that every i < Upper
func (a Absolute) Upper() int {
	return a.BucketCounts().Len() + int(a.Offset())
}

// Lower returns the minimal index inside the set, such that every i >= Lower
func (a Absolute) Lower() int {
	return int(a.Offset())
}

func (a Absolute) idx(at int) (int, bool) {
	idx := at - a.Lower()
	return idx, idx >= 0 && idx < a.BucketCounts().Len()
}
