// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package influxdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"golang.org/x/exp/maps"
)

// V1Compatibility is used to specify if the exporter should use the v1.X InfluxDB API schema.
type V1Compatibility struct {
	// Enabled is used to specify if the exporter should use the v1.X InfluxDB API schema
	Enabled bool `mapstructure:"enabled"`
	// DB is used to specify the name of the V1 InfluxDB database that telemetry will be written to.
	DB string `mapstructure:"db"`
	// Username is used to optionally specify the basic auth username
	Username string `mapstructure:"username"`
	// Password is used to optionally specify the basic auth password
	Password configopaque.String `mapstructure:"password"`
}

// Config defines configuration for the InfluxDB exporter.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`

	// Org is the InfluxDB organization name of the destination bucket.
	Org string `mapstructure:"org"`
	// Bucket is the InfluxDB bucket name that telemetry will be written to.
	Bucket string `mapstructure:"bucket"`
	// Token is used to identify InfluxDB permissions within the organization.
	Token configopaque.String `mapstructure:"token"`
	// V1Compatibility is used to specify if the exporter should use the v1.X InfluxDB API schema.
	V1Compatibility V1Compatibility `mapstructure:"v1_compatibility"`

	// SpanDimensions are span attributes to be used as line protocol tags.
	// These are always included as tags:
	// - trace ID
	// - span ID
	// The default values are strongly recommended for use with Jaeger:
	// - service.name
	// - span.name
	// Other common attributes can be found here:
	// - https://github.com/open-telemetry/opentelemetry-collector/tree/main/semconv
	SpanDimensions []string `mapstructure:"span_dimensions"`

	// MetricsSchema indicates the metrics schema to emit to line protocol.
	// Options:
	// - telegraf-prometheus-v1
	// - telegraf-prometheus-v2
	MetricsSchema string `mapstructure:"metrics_schema"`

	// PayloadMaxLines is the maximum number of line protocol lines to POST in a single request.
	PayloadMaxLines int `mapstructure:"payload_max_lines"`
	// PayloadMaxBytes is the maximum number of line protocol bytes to POST in a single request.
	PayloadMaxBytes int `mapstructure:"payload_max_bytes"`
}

func (cfg *Config) Validate() error {
	uniqueDimensions := make(map[string]struct{}, len(cfg.SpanDimensions))
	duplicateDimensions := make(map[string]struct{})
	for _, k := range cfg.SpanDimensions {
		if _, found := uniqueDimensions[k]; found {
			duplicateDimensions[k] = struct{}{}
		} else {
			uniqueDimensions[k] = struct{}{}
		}
	}

	if len(duplicateDimensions) > 0 {
		return fmt.Errorf("duplicate span dimension(s) configured: %s",
			strings.Join(maps.Keys(duplicateDimensions), ","))
	}
	return nil
}
