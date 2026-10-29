// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// dataPointSlice interface defines common methods shared across slices of metric data points
type dataPointSlice[T dataPoint] interface {
	Len() int
	At(i int) T
}

// dataPoint interface defines common methods shared across metric data point types
// (HistogramDataPoint, ExponentialHistogramDataPoint, SummaryDataPoint)
type dataPoint interface {
	Attributes() pcommon.Map
	StartTimestamp() pcommon.Timestamp
	Timestamp() pcommon.Timestamp
}

// sumCountDataPoint extends dataPoint interface with methods to access Sum and Count values
// also common to HistogramDataPoint and ExponentialHistogramDataPoint and SummaryDataPoint
type sumCountDataPoint interface {
	dataPoint
	Sum() float64
	Count() uint64
}
