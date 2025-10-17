// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package agentcomponents // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"

import (
	"go.opentelemetry.io/collector/component"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

var _ component.Config = (*Config)(nil)

// Config defines configuration for the Datadog connector.
type Config struct {
	// Traces defines the Traces specific configuration
	Traces datadogconfig.TracesConnectorConfig `mapstructure:"traces"`
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	return c.Traces.Validate()
}
