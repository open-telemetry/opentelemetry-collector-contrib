// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package generator

// PercentileRange represents a range for percentile calculations
type PercentileRange struct {
	Low  float64
	High float64
}

// HistogramInput represents the input data for a histogram metric
type HistogramInput struct {
	Count      uint64
	Sum        float64
	Min        *float64
	Max        *float64
	Boundaries []float64
	Counts     []uint64
	Attributes map[string]string
}

// ExpectedMetrics represents the expected calculated metrics from histogram data
type ExpectedMetrics struct {
	Count            uint64
	Sum              float64
	Average          float64
	Min              *float64
	Max              *float64
	PercentileRanges map[float64]PercentileRange
}

// HistogramResult combines input and expected metrics
type HistogramResult struct {
	Input    HistogramInput
	Expected ExpectedMetrics
}

// GenerationOptions configures histogram generation
type GenerationOptions struct {
	Seed     int64
	Endpoint string
}
