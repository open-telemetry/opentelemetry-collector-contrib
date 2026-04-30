// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
)

var (
	errAtLeastOneProtocol = errors.New("no protocols selected")
)

// Config represents the extension config settings within the collector's config.yaml
type Config struct {
	HTTP configoptional.Optional[confighttp.ServerConfig] `mapstructure:"http"`
}

// HTTPConfig defines the configuration for the HTTP server receiving traces.
type HTTPConfig struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	// prevent unkeyed literal initialization
	_ struct{}
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if !cfg.HTTP.HasValue() {
		return errAtLeastOneProtocol
	}
	return nil
}
