// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package data // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"

import (
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/putil/pslice"
)

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

func (dp Histogram) Add(in Histogram) Histogram {
	// bounds different: no way to merge, so reset observation to new boundaries
	if !pslice.Equal(dp.ExplicitBounds(), in.ExplicitBounds()) {
		in.MoveTo(dp.HistogramDataPoint)
		return dp
	}

	// spec requires len(BucketCounts) == len(ExplicitBounds)+1.
	// given we have limited error handling at this stage (and already verified boundaries are correct),
	// doing a best-effort add of whatever we have appears reasonable.
	n := min(dp.BucketCounts().Len(), in.BucketCounts().Len())
	for i := 0; i < n; i++ {
		sum := dp.BucketCounts().At(i) + in.BucketCounts().At(i)
		dp.BucketCounts().SetAt(i, sum)
	}

	dp.SetTimestamp(in.Timestamp())
	dp.SetCount(dp.Count() + in.Count())

	if dp.HasSum() && in.HasSum() {
		dp.SetSum(dp.Sum() + in.Sum())
	} else {
		dp.RemoveSum()
	}

	if dp.HasMin() && in.HasMin() {
		dp.SetMin(math.Min(dp.Min(), in.Min()))
	} else {
		dp.RemoveMin()
	}

	if dp.HasMax() && in.HasMax() {
		dp.SetMax(math.Max(dp.Max(), in.Max()))
	} else {
		dp.RemoveMax()
	}

	return dp
}

type bounds struct {
	lower int32
	upper int32
}

// with is an accessory for Merge() to calculate ideal combined scale.
func (b bounds) with(o bounds) bounds {
	if o.empty() {
		return b
	}
	if b.empty() {
		return o
	}
	return bounds{
		lower: min(b.lower, o.lower),
		upper: max(b.upper, o.upper),
	}
}

// empty indicates whether there are any values in a bounds.
func (b bounds) empty() bool {
	return b == bounds{}
}

// boundsAtScale is an accessory for Add() to calculate ideal combined scale.
func (dp ExpHistogram) boundsAtScale(b expo.Buckets, scale int32) bounds {
	if b.BucketCounts().Len() == 0 {
		return bounds{}
	}
	shift := dp.Scale() - scale
	a := expo.Abs(b)
	return bounds{
		lower: int32(a.Lower()) >> shift,
		upper: int32(a.Upper()) >> shift,
	}
}

// downscaleNeeded computes how much downscaling is needed by shifting the
// upper and lower bounds until they are separated by no more than size.
func downscaleNeeded(b bounds, size int) int32 {
	var change int32

	for b.upper-b.lower > int32(size) {
		b.upper >>= 1
		b.lower >>= 1
		change++
	}
	return change
}

func (dp ExpHistogram) Add(in ExpHistogram) ExpHistogram {
	type H = ExpHistogram

	minScale := min(dp.Scale(), in.Scale())

	// logic is adapted from lightstep's algorithm for enforcing max buckets:
	// https://github.com/lightstep/go-expohisto/blob/4375bf4ef2858552204edb8b4572330c94a4a755/structure/exponential.go#L542
	// first, calculate the highest and lowest indices for each bucket, given the candidate min scale.
	// then, calculate how much downscaling is needed to fit the merged range within max bucket count.
	// finally, perform the actual downscaling.
	posBounds := dp.boundsAtScale(dp.Positive(), minScale)
	posBounds = posBounds.with(in.boundsAtScale(in.Positive(), minScale))

	negBounds := dp.boundsAtScale(dp.Negative(), minScale)
	negBounds = negBounds.with(in.boundsAtScale(in.Negative(), minScale))

	minScale = min(
		minScale-downscaleNeeded(posBounds, dp.MaxSize),
		minScale-downscaleNeeded(negBounds, dp.MaxSize),
	)

	from, to := expo.Scale(dp.Scale()), expo.Scale(minScale)
	expo.Downscale(dp.Positive(), from, to)
	expo.Downscale(dp.Negative(), from, to)
	dp.SetScale(minScale)

	from = expo.Scale(in.Scale())
	expo.Downscale(in.Positive(), from, to)
	expo.Downscale(in.Negative(), from, to)
	in.SetScale(minScale)

	expo.Merge(dp.Positive(), in.Positive())
	expo.Merge(dp.Negative(), in.Negative())

	if dp.ZeroThreshold() != in.ZeroThreshold() {
		hi, lo := expo.HiLo(dp, in, H.ZeroThreshold)
		expo.WidenZero(lo.DataPoint, hi.ZeroThreshold())
	}

	dp.SetTimestamp(in.Timestamp())
	dp.SetCount(dp.Count() + in.Count())
	dp.SetZeroCount(dp.ZeroCount() + in.ZeroCount())

	if dp.HasSum() && in.HasSum() {
		dp.SetSum(dp.Sum() + in.Sum())
	} else {
		dp.RemoveSum()
	}

	if dp.HasMin() && in.HasMin() {
		dp.SetMin(math.Min(dp.Min(), in.Min()))
	} else {
		dp.RemoveMin()
	}

	if dp.HasMax() && in.HasMax() {
		dp.SetMax(math.Max(dp.Max(), in.Max()))
	} else {
		dp.RemoveMax()
	}

	return dp
}

func (dp Summary) Add(Summary) Summary {
	panic("todo")
}
