// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"

import (
	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// ExpoHistToExponentialDataPoint copies `lightstep/go-expohisto` structure.Histogram to
// pmetric.ExponentialHistogramDataPoint
func ExpoHistToExponentialDataPoint(agg *structure.Histogram[float64], dp pmetric.ExponentialHistogramDataPoint) {
	dp.SetCount(agg.Count())
	dp.SetSum(agg.Sum())
	if agg.Count() != 0 {
		dp.SetMin(agg.Min())
		dp.SetMax(agg.Max())
	}

	dp.SetZeroCount(agg.ZeroCount())
	dp.SetScale(agg.Scale())

	for _, half := range []struct {
		inFunc  func() *structure.Buckets
		outFunc func() pmetric.ExponentialHistogramDataPointBuckets
	}{
		{agg.Positive, dp.Positive},
		{agg.Negative, dp.Negative},
	} {
		in := half.inFunc()
		out := half.outFunc()
		out.SetOffset(in.Offset())
		out.BucketCounts().EnsureCapacity(int(in.Len()))

		for i := uint32(0); i < in.Len(); i++ {
			out.BucketCounts().Append(in.At(i))
		}
	}
}
