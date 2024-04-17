package expo

import "go.opentelemetry.io/collector/pdata/pmetric"

type DataPoint = pmetric.ExponentialHistogramDataPoint

func RaiseZero(dp DataPoint, threshold float64) {
	if dp.ZeroThreshold() > threshold {
		panic("new threshold must be greater")
	}

	scale := Scale(dp.Scale())
	idx := scale.Idx(threshold)

	move := func(bin pmetric.ExponentialHistogramDataPointBuckets) {
		bkt := Buckets(bin)
		for i := 0; i < idx; i++ {
			n := bkt.At(i)
			dp.SetZeroCount(dp.ZeroCount() + n)
			bkt.SetAt(i, 0)
		}
		bkt.Truncate(idx)
	}

	move(dp.Positive())
	move(dp.Negative())
}
