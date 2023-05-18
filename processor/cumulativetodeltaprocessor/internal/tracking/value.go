// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracking // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"

import "go.opentelemetry.io/collector/pdata/pcommon"

type ValuePoint struct {
	ObservedTimestamp pcommon.Timestamp
	FloatValue        float64
	IntValue          int64
	HistogramValue    *HistogramPoint
}

type HistogramPoint struct {
	Count   uint64
	Sum     float64
	Buckets []uint64
}

func (point *HistogramPoint) Clone() HistogramPoint {
	bucketValues := make([]uint64, len(point.Buckets))
	copy(bucketValues, point.Buckets)

	return HistogramPoint{
		Count:   point.Count,
		Sum:     point.Sum,
		Buckets: bucketValues,
	}
}
