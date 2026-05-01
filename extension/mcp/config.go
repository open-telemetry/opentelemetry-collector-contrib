// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

var errHTTPEndpointRequired = errors.New("http endpoint required")

// Config represents the extension config settings within the collector's config.yaml
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the extension configuration is valid
func (cfg *Config) Validate() error {
	if cfg.NetAddr.Endpoint == "" {
		return errHTTPEndpointRequired
	}
	return nil
}
