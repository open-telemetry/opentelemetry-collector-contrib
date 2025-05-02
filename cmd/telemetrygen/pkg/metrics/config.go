// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"errors"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// Config describes the test scenario.
type Config struct {
	common.Config
	NumMetrics             int
	MetricName             string
	MetricType             MetricType
	AggregationTemporality AggregationTemporality
	SpanID                 string
	TraceID                string
}

// NewConfig creates a new Config with default values.
func NewConfig() *Config {
	cfg := &Config{}
	cfg.SetDefaults()
	return cfg
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	c.CommonFlags(fs)

	fs.StringVar(&c.HTTPPath, "otlp-http-url-path", c.HTTPPath, "Which URL path to write to")

	fs.IntVar(&c.NumMetrics, "metrics", c.NumMetrics, "Number of metrics to generate in each worker (ignored if duration is provided)")

	fs.StringVar(&c.TraceID, "trace-id", c.TraceID, "TraceID to use as exemplar")
	fs.StringVar(&c.SpanID, "span-id", c.SpanID, "SpanID to use as exemplar")

	fs.Var(&c.MetricType, "metric-type", "Metric type enum. must be one of 'Gauge' or 'Sum'")
	fs.Var(&c.AggregationTemporality, "aggregation-temporality", "aggregation-temporality for metrics. Must be one of 'delta' or 'cumulative'")
}

// SetDefaults sets the default values for the configuration
// This is called before parsing the command line flags and when
// calling NewConfig()
func (c *Config) SetDefaults() {
	c.Config.SetDefaults()
	c.HTTPPath = "/v1/metrics"
	c.NumMetrics = 1

	c.MetricName = "gen"
	// Use Gauge as default metric type.
	c.MetricType = MetricTypeGauge
	// Use cumulative temporality as default.
	c.AggregationTemporality = AggregationTemporality(metricdata.CumulativeTemporality)

	c.TraceID = ""
	c.SpanID = ""
}

// Validate validates the test scenario parameters.
func (c *Config) Validate() error {
	if c.TotalDuration <= 0 && c.NumMetrics <= 0 {
		return errors.New("either `metrics` or `duration` must be greater than 0")
	}

	if c.TraceID != "" {
		if err := common.ValidateTraceID(c.TraceID); err != nil {
			return err
		}
	}

	if c.SpanID != "" {
		if err := common.ValidateSpanID(c.SpanID); err != nil {
			return err
		}
	}

	return nil
}
