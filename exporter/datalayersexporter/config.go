// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datalayersexporter

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"golang.org/x/exp/maps"
)

type Trace struct {
	Table string `mapstructure:"table"`
	// SpanDimensions are span attributes to be used as line protocol tags.
	// These are always included as tags:
	// - trace ID
	// - span ID
	// The default values are strongly recommended for use with Jaeger:
	// - service.name
	// - span.name
	// Other common attributes can be found here:
	// - https://opentelemetry.io/docs/specs/semconv/
	SpanDimensions []string `mapstructure:"span_dimensions"`
	// SpanFields are span attributes to be used as line protocol fields.
	// SpanFields can be empty.
	SpanFields []string      `mapstructure:"span_fields"`
	Custom     []CustomTrace `mapstructure:"custom"`
}

type CustomTrace struct {
	Key            string   `mapstructure:"key"`
	Table          string   `mapstructure:"table"`
	SpanDimensions []string `mapstructure:"span_dimensions"`
	SpanFields     []string `mapstructure:"span_fields"`
}

// Config defines configuration for the InfluxDB exporter.
type Config struct {
	confighttp.ClientConfig   `mapstructure:",squash"`
	QueueSettings             exporterhelper.QueueConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`

	// Org is the InfluxDB organization name of the destination bucket.
	Org string `mapstructure:"org"`
	// Bucket is the InfluxDB bucket name that telemetry will be written to.
	Bucket string `mapstructure:"bucket"`
	// Token is used to identify InfluxDB permissions within the organization.
	Token configopaque.String `mapstructure:"token"`
	// DB is used to specify the name of the V1 InfluxDB database that telemetry will be written to.
	DB string `mapstructure:"db"`
	// Username is used to optionally specify the basic auth username
	Username string `mapstructure:"username"`
	// Password is used to optionally specify the basic auth password
	Password configopaque.String `mapstructure:"password"`

	Trace Trace `mapstructure:"trace"`
	// PayloadMaxLines is the maximum number of line protocol lines to POST in a single request.
	PayloadMaxLines int `mapstructure:"payload_max_lines"`
	// PayloadMaxBytes is the maximum number of line protocol bytes to POST in a single request.
	PayloadMaxBytes int `mapstructure:"payload_max_bytes"`
}

func (cfg *Config) Validate() error {
	globalSpanTags := make(map[string]struct{}, len(cfg.Trace.SpanDimensions))
	globalSpanFields := make(map[string]struct{}, len(cfg.Trace.SpanDimensions))
	duplicateTags := make(map[string]struct{})
	for _, k := range cfg.Trace.SpanDimensions {
		if _, found := globalSpanTags[k]; found {
			duplicateTags[k] = struct{}{}
		} else {
			globalSpanTags[k] = struct{}{}
		}
	}
	if len(duplicateTags) > 0 {
		return fmt.Errorf("duplicate span dimension(s) configured: %s",
			strings.Join(maps.Keys(duplicateTags), ","))
	}
	duplicateFields := make(map[string]struct{})
	for _, k := range cfg.Trace.SpanFields {
		if _, found := globalSpanFields[k]; found {
			duplicateTags[k] = struct{}{}
		} else {
			globalSpanFields[k] = struct{}{}
		}
	}
	if len(duplicateFields) > 0 {
		return fmt.Errorf("duplicate span fields(s) configured: %s",
			strings.Join(maps.Keys(duplicateFields), ","))
	}

	for _, custom := range cfg.Trace.Custom {
		customSpanTags := make(map[string]struct{}, len(cfg.Trace.SpanDimensions))
		customSpanFields := make(map[string]struct{})
		duplicateTags = make(map[string]struct{})
		for _, k := range custom.SpanDimensions {
			if _, found := customSpanTags[k]; found {
				duplicateTags[k] = struct{}{}
			} else {
				customSpanTags[k] = struct{}{}
			}
		}
		if len(duplicateTags) > 0 {
			return fmt.Errorf("duplicate custom span dimension(s) configured: %s",
				strings.Join(maps.Keys(duplicateTags), ","))
		}
		duplicateFields := make(map[string]struct{})
		for _, k := range custom.SpanFields {
			if _, found := customSpanFields[k]; found {
				duplicateFields[k] = struct{}{}
			} else {
				customSpanFields[k] = struct{}{}
			}
		}
		if len(duplicateFields) > 0 {
			return fmt.Errorf("duplicate custom span fields(s) configured: %s",
				strings.Join(maps.Keys(duplicateFields), ","))
		}

	}

	return nil
}
