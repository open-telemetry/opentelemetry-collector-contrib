package histotest

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/histo"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Histogram struct {
	Ts pcommon.Timestamp

	Bounds  histo.Bounds
	Buckets []uint64

	Count uint64
	Sum   *float64

	Min, Max *float64
}

func (hist Histogram) Into() pmetric.HistogramDataPoint {
	dp := pmetric.NewHistogramDataPoint()

	dp.SetTimestamp(hist.Ts)

	dp.ExplicitBounds().FromRaw(hist.Bounds)
	dp.BucketCounts().FromRaw(hist.Buckets)

	dp.SetCount(hist.Count)
	if hist.Sum != nil {
		dp.SetSum(*hist.Sum)
	}

	if hist.Min != nil {
		dp.SetMin(*hist.Min)
	}
	if hist.Max != nil {
		dp.SetMax(*hist.Max)
	}

	return dp
}
