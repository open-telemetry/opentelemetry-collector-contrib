// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogprocessor"

// Config defines the configuration options for datadogprocessor.
type Config struct {
	// Traces defines the Traces processor specific configuration
	Traces TracesConfig `mapstructure:"traces"`
}

type TracesConfig struct {
	// If set to true the OpenTelemetry span name will used in the Datadog resource name.
	// If set to false the resource name will be filled with the instrumentation library name + span kind.
	// The default value is `false`.
	SpanNameAsResourceName bool `mapstructure:"span_name_as_resource_name"`
}
