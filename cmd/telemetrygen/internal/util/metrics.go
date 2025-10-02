// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// ExpoHistToSDKExponentialDataPoint copies `lightstep/go-expohisto` structure.Histogram to
// metricdata.ExponentialHistogramDataPoint
func ExpoHistToSDKExponentialDataPoint(agg *structure.Histogram[float64], dp *metricdata.ExponentialHistogramDataPoint[int64]) {
	dp.Count = agg.Count()
	dp.Sum = int64(agg.Sum())
	dp.ZeroCount = agg.ZeroCount()
	dp.Scale = agg.Scale()
	dp.ZeroThreshold = 0.0 // go-expohisto doesn't expose ZeroThreshold, use default

	// Convert positive buckets
	posBuckets := agg.Positive()
	dp.PositiveBucket.Offset = posBuckets.Offset()
	dp.PositiveBucket.Counts = make([]uint64, posBuckets.Len())
	for i := uint32(0); i < posBuckets.Len(); i++ {
		dp.PositiveBucket.Counts[i] = posBuckets.At(i)
	}

	// Convert negative buckets
	negBuckets := agg.Negative()
	dp.NegativeBucket.Offset = negBuckets.Offset()
	dp.NegativeBucket.Counts = make([]uint64, negBuckets.Len())
	for i := uint32(0); i < negBuckets.Len(); i++ {
		dp.NegativeBucket.Counts[i] = negBuckets.At(i)
	}
}
