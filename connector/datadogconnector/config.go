// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

var _ component.Config = (*Config)(nil)

// Config defines configuration for the Datadog connector.
type Config struct {
	// Traces defines the Traces specific configuration
	Traces datadogconfig.TracesConnectorConfig `mapstructure:"traces"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Traces.IgnoreMissingDatadogFields {
		return errors.New("ignore_missing_datadog_fields is not yet supported in the connector")
	}
	return c.Traces.Validate()
}
