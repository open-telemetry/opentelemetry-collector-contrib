// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatch // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/aws/cloudwatch"

// HistogramDataPoint is a single data point that describes the values of a Histogram.
// The values of a histogram are represented by equal-length series of values and counts. Each value/count pair
// is a single bucket of the Histogram.
type HistogramDataPoint interface {
	ValuesAndCounts() ([]float64, []float64)
	Sum() float64
	SampleCount() float64
	Minimum() float64
	Maximum() float64
}
