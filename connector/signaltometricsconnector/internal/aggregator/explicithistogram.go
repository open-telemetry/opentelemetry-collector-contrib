// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregator // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/aggregator"

import (
	"sort"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type explicitHistogramDP struct {
	attrs pcommon.Map

	sum   float64
	count uint64

	// bounds represents the explicitly defined boundaries for the histogram
	// bucket. The boundaries for a bucket at index i are:
	//
	// (-Inf, bounds[i]] for i == 0
	// (bounds[i-1], bounds[i]] for 0 < i < len(bounds)
	// (bounds[i-1], +Inf) for i == len(bounds)
	//
	// Based on above representation, a bounds of length n represents n+1 buckets.
	bounds []float64

	// counts represents the count values of histogram for each bucket. The sum of
	// counts across all buckets must be equal to the count variable. The length of
	// counts must be one greater than the length of bounds slice.
	counts []uint64
}

func newExplicitHistogramDP(attrs pcommon.Map, bounds []float64) *explicitHistogramDP {
	return &explicitHistogramDP{
		attrs:  attrs,
		bounds: bounds,
		counts: make([]uint64, len(bounds)+1),
	}
}

func (dp *explicitHistogramDP) Aggregate(value float64, count int64) {
	dp.sum += value * float64(count)
	dp.count += uint64(count)
	dp.counts[sort.SearchFloat64s(dp.bounds, value)] += uint64(count)
}

func (dp *explicitHistogramDP) Copy(
	timestamp time.Time,
	dest pmetric.HistogramDataPoint,
) {
	dp.attrs.CopyTo(dest.Attributes())
	dest.ExplicitBounds().FromRaw(dp.bounds)
	dest.BucketCounts().FromRaw(dp.counts)
	dest.SetCount(dp.count)
	dest.SetSum(dp.sum)
	// TODO determine appropriate start time
	dest.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
}
