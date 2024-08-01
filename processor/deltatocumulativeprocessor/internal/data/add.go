// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package data // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"

import (
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
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

	if dp.Scale() != in.Scale() {
		hi, lo := expo.HiLo(dp, in, H.Scale)
		from, to := expo.Scale(hi.Scale()), expo.Scale(lo.Scale())
		expo.Downscale(hi.Positive(), from, to)
		expo.Downscale(hi.Negative(), from, to)
		hi.SetScale(lo.Scale())
	}

	if dp.ZeroThreshold() != in.ZeroThreshold() {
		hi, lo := expo.HiLo(dp, in, H.ZeroThreshold)
		expo.WidenZero(lo.DataPoint, hi.ZeroThreshold())
	}

	expo.Merge(dp.Positive(), in.Positive())
	expo.Merge(dp.Negative(), in.Negative())

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
