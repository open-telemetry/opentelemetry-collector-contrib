// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"

// Start of PICT inputs for generating golden dataset metrics (pict_input_metrics.txt)

// PICTMetricInputs defines one pairwise combination of MetricData variations
type PICTMetricInputs struct {
	// Specifies the number of points on each metric.
	NumPtsPerMetric PICTNumPtsPerMetric
	// Specifies the types of metrics that can be generated.
	MetricType PICTMetricType
	// Specifies the number of labels on each datapoint.
	NumPtLabels PICTNumPtLabels
	// Specifies the number of attributes on each resource.
	NumResourceAttrs PICTNumResourceAttrs
}

// PICTMetricType enumerates the types of metrics that can be generated.
type PICTMetricType string

const (
	MetricTypeIntGauge                 PICTMetricType = "IntGauge"
	MetricTypeMonotonicIntSum          PICTMetricType = "MonotonicIntSum"
	MetricTypeNonMonotonicIntSum       PICTMetricType = "NonMonotonicIntSum"
	MetricTypeDoubleGauge              PICTMetricType = "DoubleGauge"
	MetricTypeMonotonicDoubleSum       PICTMetricType = "MonotonicDoubleSum"
	MetricTypeNonMonotonicDoubleSum    PICTMetricType = "NonMonotonicDoubleSum"
	MetricTypeDoubleExemplarsHistogram PICTMetricType = "DoubleExemplarsHistogram"
	MetricTypeIntExemplarsHistogram    PICTMetricType = "IntExemplarsHistogram"
)

// PICTNumPtLabels enumerates the number of labels on each datapoint.
type PICTNumPtLabels string

const (
	LabelsNone PICTNumPtLabels = "NoLabels"
	LabelsOne  PICTNumPtLabels = "OneLabel"
	LabelsMany PICTNumPtLabels = "ManyLabels"
)

// PICTNumPtsPerMetric enum for the number of points on each metric.
type PICTNumPtsPerMetric string

const (
	NumPtsPerMetricOne  PICTNumPtsPerMetric = "OnePt"
	NumPtsPerMetricMany PICTNumPtsPerMetric = "ManyPts"
)

// PICTNumResourceAttrs enum for the number of attributes on each resource.
type PICTNumResourceAttrs string

const (
	AttrsNone PICTNumResourceAttrs = "NoAttrs"
	AttrsOne  PICTNumResourceAttrs = "OneAttr"
	AttrsTwo  PICTNumResourceAttrs = "TwoAttrs"
)
