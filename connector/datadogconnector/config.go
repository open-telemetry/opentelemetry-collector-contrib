// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"go.opentelemetry.io/collector/component"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

var _ component.Config = (*Config)(nil)

// Config defines configuration for the Datadog connector.
type Config struct {
	// Traces defines the Traces specific configuration
	Traces TracesConfig `mapstructure:"traces"`
}

// Deprecated: [v0.110.0] Use `datadog.TracesConnectorConfig` instead.
// TracesConfig defines the traces specific configuration options
type TracesConfig = datadogconfig.TracesConnectorConfig

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	return c.Traces.Validate()
}
