// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hydrolixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hydrolixexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config defines configuration for Hydrolix exporter.
type Config struct {
	// ClientConfig defines the HTTP client settings.
	confighttp.ClientConfig `mapstructure:",squash"`

	// Table is the Hydrolix table name where data will be written.
	Table string `mapstructure:"table"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.ClientConfig.Endpoint == "" {
		return errors.New("endpoint must be non-empty")
	}
	if cfg.Table == "" {
		return errors.New("table must be non-empty")
	}
	return nil
}
