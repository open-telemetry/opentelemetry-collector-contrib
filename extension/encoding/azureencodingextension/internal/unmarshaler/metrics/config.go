// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/metrics"

import (
	"fmt"
)

type MetricAggregation string

const (
	AggregationCount   MetricAggregation = "count"
	AggregationTotal   MetricAggregation = "total"
	AggregationMinimum MetricAggregation = "minimum"
	AggregationMaximum MetricAggregation = "maximum"
	AggregationAverage MetricAggregation = "average"
)

type MetricsConfig struct {
	// TimeFormats is a list of time formats parsing layouts for Azure Metrics Records
	TimeFormats []string `mapstructure:"time_formats"`
	// Aggregations is a list of metric aggregations to be exposed in OTEL result, default - all
	Aggregations []MetricAggregation `mapstructure:"aggregations"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (cfg *MetricsConfig) Validate() error {
	for _, agg := range cfg.Aggregations {
		switch agg {
		case AggregationTotal, AggregationCount, AggregationMinimum, AggregationMaximum, AggregationAverage:
			// valid aggregation
		default:
			return fmt.Errorf("invalid aggregation %q", agg)
		}
	}

	return nil
}
