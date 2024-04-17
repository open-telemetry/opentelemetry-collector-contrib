package expo

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func Merge(arel, brel pmetric.ExponentialHistogramDataPointBuckets) {
	a, b := Absolute{buckets: arel}, Absolute{buckets: brel}

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

type buckets = pmetric.ExponentialHistogramDataPointBuckets

type Absolute struct {
	buckets
}

func (a Absolute) Abs(at int) uint64 {
	if i, ok := a.idx(at); ok {
		return a.BucketCounts().At(i)
	}
	return 0
}

func (a Absolute) Upper() int {
	return a.BucketCounts().Len() + int(a.Offset())
}

func (a Absolute) Lower() int {
	return int(a.Offset())
}

func (a Absolute) idx(at int) (int, bool) {
	idx := at - a.Lower()
	return idx, idx >= 0 && idx < a.BucketCounts().Len()
}
