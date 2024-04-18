// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package data // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"

import (
	"math"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"go.opentelemetry.io/collector/pdata/pmetric"
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

// nolint
func (dp Histogram) Add(in Histogram) Histogram {
	panic("todo")
}

func (dp ExpHistogram) Add(in ExpHistogram) ExpHistogram {
	type H = ExpHistogram

	switch {
	case dp.Timestamp() >= in.Timestamp():
		panic("out of order")
	}

	if dp.Scale() != in.Scale() {
		hi, lo := expo.HiLo(dp, in, H.Scale)
		from, to := expo.Scale(hi.Scale()), expo.Scale(lo.Scale())
		expo.Downscale(hi.Positive(), from, to)
		expo.Downscale(hi.Negative(), from, to)
		hi.SetScale(lo.Scale())
	}

	if dp.ZeroThreshold() != in.ZeroThreshold() {
		hi, lo := expo.HiLo(dp, in, ExpHistogram.ZeroThreshold)
		expo.WidenZero(lo.ExponentialHistogramDataPoint, hi.ZeroThreshold())
	}

	expo.Merge(dp.Positive(), in.Positive())
	expo.Merge(dp.Negative(), in.Negative())

	dp.SetTimestamp(in.Timestamp())
	dp.SetCount(dp.Count() + in.Count())
	dp.SetZeroCount(dp.ZeroCount() + in.ZeroCount())

	optionals := []field{
		{get: H.Sum, set: H.SetSum, has: H.HasSum, del: H.RemoveSum, op: func(a, b float64) float64 { return a + b }},
		{get: H.Min, set: H.SetMin, has: H.HasMin, del: H.RemoveMin, op: math.Min},
		{get: H.Max, set: H.SetMax, has: H.HasMax, del: H.RemoveMax, op: math.Max},
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

func pos(i int) int {
	if i < 0 {
		i = -i
	}
	return i
}
