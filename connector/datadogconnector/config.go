// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"
)

var _ component.Config = (*Config)(nil)

// Config defines configuration for the Datadog connector.
type Config struct {
	// Traces defines the Traces specific configuration
	Traces TracesConfig `mapstructure:"traces"`
}

// Deprecated: Use `datadog.TracesConfig` instead.
// TracesConfig defines the traces specific configuration options
type TracesConfig = datadog.TracesConfig

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	return c.Traces.Validate()
}
