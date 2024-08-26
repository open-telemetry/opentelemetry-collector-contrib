// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package histotest // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/histo/histotest"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/histo"
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
	if hist.Bounds == nil {
		dp.ExplicitBounds().FromRaw(histo.DefaultBounds)
	}
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

type Bounds histo.Bounds

func (bs Bounds) Observe(observations ...float64) Histogram {
	dp := histo.Bounds(bs).Observe(observations...)
	return Histogram{
		Ts:      dp.Timestamp(),
		Bounds:  dp.ExplicitBounds().AsRaw(),
		Buckets: dp.BucketCounts().AsRaw(),
		Count:   dp.Count(),
		Sum:     ptr(dp.Sum()),
		Min:     ptr(dp.Min()),
		Max:     ptr(dp.Max()),
	}
}

func ptr[T any](v T) *T {
	return &v
}
