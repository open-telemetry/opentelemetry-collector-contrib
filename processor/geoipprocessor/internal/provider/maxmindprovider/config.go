// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maxmind // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider/maxmindprovider"

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
)

// Config defines configuration for MaxMind provider.
type Config struct {
	// DatabasePath section allows specifying a local GeoIP database
	// file to retrieve the geographical metadata from.
	DatabasePath string `mapstructure:"database_path"`
}

var _ provider.Config = (*Config)(nil)

// Validate implements provider.Config.
func (c *Config) Validate() error {
	if c.DatabasePath == "" {
		return errors.New("a local geoIP database path must be provided")
	}
	return nil
}
