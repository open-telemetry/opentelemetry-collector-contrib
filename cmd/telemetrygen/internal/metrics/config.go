// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

// Config describes the test scenario.
type Config struct {
	common.Config
	NumMetrics int
	MetricName string
	MetricType metricType
	SpanID     string
	TraceID    string
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	// Use Gauge as default metric type.
	c.MetricName = "gen"
	c.MetricType = metricTypeGauge

	c.CommonFlags(fs)

	fs.StringVar(&c.HTTPPath, "otlp-http-url-path", "/v1/metrics", "Which URL path to write to")

	fs.Var(&c.MetricType, "metric-type", "Metric type enum. must be one of 'Gauge' or 'Sum'")
	fs.IntVar(&c.NumMetrics, "metrics", 1, "Number of metrics to generate in each worker (ignored if duration is provided)")

	fs.StringVar(&c.TraceID, "trace-id", "", "TraceID to use as exemplar")
	fs.StringVar(&c.SpanID, "span-id", "", "SpanID to use as exemplar")

}

// Validate validates the test scenario parameters.
func (c *Config) Validate() error {
	if c.TotalDuration <= 0 && c.NumMetrics <= 0 {
		return fmt.Errorf("either `metrics` or `duration` must be greater than 0")
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
