package expo

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type DataPoint = pmetric.ExponentialHistogramDataPoint

// WidenZero widens the zero-bucket to span at least [-width,width], possibly wider
// if min falls in the middle of a bucket.
//
// Both buckets MUST be of same scale.
func WidenZero(dp DataPoint, width float64) {
	switch {
	case width == dp.ZeroThreshold():
		return
	case width < dp.ZeroThreshold():
		panic(fmt.Sprintf("min must be larger than current threshold (%f)", dp.ZeroThreshold()))
	}

	scale := Scale(dp.Scale())
	lo := scale.Idx(width)

	widen := func(bs pmetric.ExponentialHistogramDataPointBuckets) {
		abs := Abs(bs)
		for i := abs.Lower(); i <= lo; i++ {
			dp.SetZeroCount(dp.ZeroCount() + abs.Abs(i))
		}
		up := abs.Upper()
		abs.Slice(min(lo+1, up), up)
	}

	widen(dp.Positive())
	widen(dp.Negative())

	_, max := scale.Bounds(lo)
	dp.SetZeroThreshold(max)
}

// Slice drops data outside the range from <= i < to from the bucket counts. It behaves the same as Go's [a:b]
func (a Absolute) Slice(from, to int) {
	lo, up := a.Lower(), a.Upper()
	switch {
	case from > to:
		panic(fmt.Sprintf("bad bounds: must be from<=to (got %d<=%d)", from, to))
	case from < lo || to > up:
		panic(fmt.Sprintf("%d:%d is out of bounds for %d:%d", from, to, lo, up))
	}

	first := from - lo
	last := to - lo

	a.BucketCounts().FromRaw(a.BucketCounts().AsRaw()[first:last])
	a.SetOffset(int32(from))
}
