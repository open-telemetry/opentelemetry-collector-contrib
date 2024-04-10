// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package data // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// failure handler
// operations of this package are expected to have no failure cases during
// spec-compliant operation.
// if spec-compliant assumptions are broken however, we want to fail loud
// and clear. can be overwritten during testing
var fail = func(msg string) {
	panic(msg)
}

func (dp Number) Add(in Number) Number {
	switch in.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		v := dp.DoubleValue() + in.DoubleValue()
		dp.SetDoubleValue(v)
	case pmetric.NumberDataPointValueTypeInt:
		v := dp.IntValue() + in.IntValue()
		dp.SetIntValue(v)
	}
	dp.SetTimestamp(in.Timestamp())
	return dp
}

// nolint
func (dp Histogram) Add(in Histogram) Histogram {
	panic("todo")
}

func (dp ExpHistogram) Add(in ExpHistogram) ExpHistogram {
	switch {
	case dp.Timestamp() >= in.Timestamp():
		fail("out of order")
	case dp.Scale() != in.Scale():
		fail("scale changed")
	case dp.ZeroCount() != in.ZeroCount():
		fail("zero count changed")
	}

	aggregate := func(dpBuckets, inBuckets pmetric.ExponentialHistogramDataPointBuckets) {
		var (
			dp   = Buckets{data: dpBuckets.BucketCounts(), offset: int(dpBuckets.Offset())}
			in   = Buckets{data: inBuckets.BucketCounts(), offset: int(inBuckets.Offset())}
			aggr = Buckets{data: pcommon.NewUInt64Slice()}
		)
		aggr.offset = int(min(dpBuckets.Offset(), inBuckets.Offset()))
		if aggr.offset == dp.offset {
			aggr.data = dp.data
		}
		aggr.EnsureLen(max(dp.Len(), in.Len()))

		for i := 0; i < aggr.Len(); i++ {
			aggr.SetAt(i, dp.At(i)+in.At(i))
		}

		aggr.CopyTo(dpBuckets)
	}

	aggregate(dp.Positive(), in.Positive())
	aggregate(dp.Negative(), in.Negative())

	dp.SetTimestamp(in.Timestamp())
	dp.SetCount(dp.Count() + in.Count())

	type T = ExpHistogram
	optionals := []field{
		{get: T.Sum, set: T.SetSum, has: T.HasSum, del: T.RemoveSum, op: func(a, b float64) float64 { return a + b }},
		{get: T.Min, set: T.SetMin, has: T.HasMin, del: T.RemoveMin, op: func(a, b float64) float64 { return min(a, b) }},
		{get: T.Max, set: T.SetMax, has: T.HasMax, del: T.RemoveMax, op: func(a, b float64) float64 { return max(a, b) }},
	}
	for _, f := range optionals {
		if f.has(dp) && f.has(in) {
			f.set(dp, f.op(f.get(dp), f.get(in)))
		} else {
			f.del(dp)
		}
	}

	return dp
}

type field struct {
	get func(ExpHistogram) float64
	set func(ExpHistogram, float64)
	has func(ExpHistogram) bool
	del func(ExpHistogram)
	op  func(a, b float64) float64
}

type Buckets struct {
	data   pcommon.UInt64Slice
	offset int
}

func (o Buckets) Len() int {
	return o.data.Len() + o.offset
}

func (o Buckets) At(i int) uint64 {
	idx, ok := o.idx(i)
	if !ok {
		return 0
	}
	return o.data.At(idx)
}

func (o Buckets) SetAt(i int, v uint64) {
	idx, ok := o.idx(i)
	if !ok {
		return
	}
	o.data.SetAt(idx, v)
}

func (o Buckets) EnsureLen(n int) {
	sz := n - o.offset
	o.data.EnsureCapacity(sz)
	o.data.Append(make([]uint64, sz-o.data.Len())...)
}

func (o Buckets) idx(i int) (int, bool) {
	idx := i - o.offset
	return idx, idx >= 0 && idx < o.data.Len()
}

func (o Buckets) CopyTo(dst pmetric.ExponentialHistogramDataPointBuckets) {
	o.data.CopyTo(dst.BucketCounts())
	dst.SetOffset(int32(o.offset))
}
