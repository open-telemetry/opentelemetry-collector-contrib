// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogprocessor"

import (
	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration options for datadogprocessor.
type Config struct {

	// MetricsExporter specifies the name of the metrics exporter to be used when
	// exporting stats metrics.
	MetricsExporter component.ID `mapstructure:"metrics_exporter"`

	// Traces defines the Traces processor specific configuration
	Traces TracesConfig `mapstructure:"traces"`
}

type TracesConfig struct {
	// If set to true the OpenTelemetry span name will used in the Datadog resource name.
	// If set to false the resource name will be filled with the instrumentation library name + span kind.
	// The default value is `false`.
	SpanNameAsResourceName bool `mapstructure:"span_name_as_resource_name"`
}

func createDefaultConfig() component.Config {
	return &Config{
		MetricsExporter: datadogComponent,
	}
}

// datadogComponent defines the default component that will be used for
// exporting metrics.
var datadogComponent = component.NewID(component.Type("datadog"))
