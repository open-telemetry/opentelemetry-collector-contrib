// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregator // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/aggregator"

import (
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type exponentialHistogramDP struct {
	attrs pcommon.Map
	data  *structure.Histogram[float64]
}

func newExponentialHistogramDP(attrs pcommon.Map, maxSize int32) *exponentialHistogramDP {
	return &exponentialHistogramDP{
		attrs: attrs,
		data: structure.NewFloat64(
			structure.NewConfig(structure.WithMaxSize(maxSize)),
		),
	}
}

func (dp *exponentialHistogramDP) Aggregate(value float64, count int64) {
	dp.data.UpdateByIncr(value, uint64(count))
}

func (dp *exponentialHistogramDP) Copy(
	timestamp time.Time,
	dest pmetric.ExponentialHistogramDataPoint,
) {
	dp.attrs.CopyTo(dest.Attributes())
	dest.SetZeroCount(dp.data.ZeroCount())
	dest.SetScale(dp.data.Scale())
	dest.SetCount(dp.data.Count())
	dest.SetSum(dp.data.Sum())
	if dp.data.Count() > 0 {
		dest.SetMin(dp.data.Min())
		dest.SetMax(dp.data.Max())
	}
	// TODO determine appropriate start time
	dest.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

	copyBucketRange(dp.data.Positive(), dest.Positive())
	copyBucketRange(dp.data.Negative(), dest.Negative())
}

// copyBucketRange copies a bucket range from exponential histogram
// datastructure to the OTel representation.
func copyBucketRange(
	src *structure.Buckets,
	dest pmetric.ExponentialHistogramDataPointBuckets,
) {
	dest.SetOffset(src.Offset())
	dest.BucketCounts().EnsureCapacity(int(src.Len()))
	for i := uint32(0); i < src.Len(); i++ {
		dest.BucketCounts().Append(src.At(i))
	}
}
