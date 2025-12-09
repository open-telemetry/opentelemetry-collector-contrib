// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package intervalprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

// HistogramType defines the type of histogram to produce
type HistogramType string

const (
	// HistogramTypeExplicit produces explicit bucket histograms with user-defined boundaries
	HistogramTypeExplicit HistogramType = "explicit"
	// HistogramTypeExponential produces exponential histograms with automatic bucket boundaries
	HistogramTypeExponential HistogramType = "exponential"
)

const (
	// DefaultExponentialHistogramMaxSize is the default maximum number of buckets for exponential histograms
	DefaultExponentialHistogramMaxSize int32 = 160
)

var (
	ErrInvalidIntervalValue           = errors.New("invalid interval value")
	ErrInvalidHistogramBuckets        = errors.New("histogram buckets must be sorted in ascending order")
	ErrEmptyHistogramMetricName       = errors.New("histogram aggregation metric name cannot be empty")
	ErrMissingHistogramBuckets        = errors.New("explicit histogram aggregation requires at least one bucket boundary")
	ErrInvalidHistogramType           = errors.New("histogram type must be 'explicit' or 'exponential'")
	ErrExponentialHistogramMaxSize    = errors.New("exponential histogram max_size must be positive")
	ErrBucketsNotAllowedForExponential = errors.New("buckets should not be specified for exponential histograms")
)

var _ component.Config = (*Config)(nil)

// Config defines the configuration for the processor.
type Config struct {
	// Interval is the time interval at which the processor will aggregate metrics.
	Interval time.Duration `mapstructure:"interval"`
	// PassThrough is a configuration that determines whether gauge and summary metrics should be passed through
	// as they are or aggregated.
	PassThrough PassThrough `mapstructure:"pass_through"`
	// AggregateToHistogram is a list of configurations for aggregating gauge or counter metrics into histograms.
	AggregateToHistogram []AggregateToHistogram `mapstructure:"aggregate_to_histogram"`
}

type PassThrough struct {
	// Gauge is a flag that determines whether gauge metrics should be passed through
	// as they are or aggregated.
	Gauge bool `mapstructure:"gauge"`
	// Summary is a flag that determines whether summary metrics should be passed through
	// as they are or aggregated.
	Summary bool `mapstructure:"summary"`
}

// AggregateToHistogram defines the configuration for aggregating gauge or counter metrics into a histogram.
type AggregateToHistogram struct {
	// MetricName is the name of the source metric to aggregate (supports exact match).
	MetricName string `mapstructure:"metric_name"`
	// OutputName is the name of the resulting histogram metric. If empty, defaults to MetricName + "_histogram".
	OutputName string `mapstructure:"output_name"`
	// HistogramType specifies whether to produce explicit or exponential histograms.
	// Defaults to "explicit" if not specified.
	HistogramType HistogramType `mapstructure:"histogram_type"`
	// Buckets are the explicit bucket boundaries for explicit histograms.
	// Required when histogram_type is "explicit", ignored for "exponential".
	Buckets []float64 `mapstructure:"buckets"`
	// MaxSize is the maximum number of buckets for exponential histograms.
	// Only used when histogram_type is "exponential". Defaults to 160.
	MaxSize int32 `mapstructure:"max_size"`
}

// Validate checks whether the input configuration has all of the required fields for the processor.
// An error is returned if there are any invalid inputs.
func (config *Config) Validate() error {
	if config.Interval <= 0 {
		return ErrInvalidIntervalValue
	}

	for _, agg := range config.AggregateToHistogram {
		if agg.MetricName == "" {
			return ErrEmptyHistogramMetricName
		}

		// Default to explicit if not specified
		histType := agg.HistogramType
		if histType == "" {
			histType = HistogramTypeExplicit
		}

		switch histType {
		case HistogramTypeExplicit:
			if len(agg.Buckets) == 0 {
				return ErrMissingHistogramBuckets
			}
			// Validate buckets are sorted in ascending order
			for i := 1; i < len(agg.Buckets); i++ {
				if agg.Buckets[i] <= agg.Buckets[i-1] {
					return ErrInvalidHistogramBuckets
				}
			}
		case HistogramTypeExponential:
			if len(agg.Buckets) > 0 {
				return ErrBucketsNotAllowedForExponential
			}
			if agg.MaxSize < 0 {
				return ErrExponentialHistogramMaxSize
			}
		default:
			return ErrInvalidHistogramType
		}
	}

	return nil
}
