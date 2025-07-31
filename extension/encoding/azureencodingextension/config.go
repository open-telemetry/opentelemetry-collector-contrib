// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension"

type TracesConfig struct {
	TimeFormats []string `mapstructure:"time_formats"`
	// ... other traces-specific configurations
	// prevent unkeyed literal initialization
	_ struct{}
}

type LogsConfig struct {
	TimeFormats []string `mapstructure:"time_formats"`
	// ... other logs-specific configurations
	// prevent unkeyed literal initialization
	_ struct{}
}

type MetricsConfig struct {
	TimeFormats []string `mapstructure:"time_formats"`
	// ... other metrics-specific configurations
	// prevent unkeyed literal initialization
	_ struct{}
}

type Config struct {
	Traces  TracesConfig  `mapstructure:"traces"`
	Logs    LogsConfig    `mapstructure:"logs"`
	Metrics MetricsConfig `mapstructure:"metrics"`
	// prevent unkeyed literal initialization
	_ struct{}
}
