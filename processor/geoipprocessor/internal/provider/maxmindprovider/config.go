package maxmind

import (
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
)

// Config defines configuration for MaxMind provider.
type Config struct {
	// GeoIPDatabasePath section allows specifying a local GeoIP database
	// file to retrieve the geographical metadata from.
	GeoIPDatabasePath string `mapstructure:"geoip_database_path"`
}

var _ provider.Config = (*Config)(nil)

// Validate implements provider.Config.
func (c *Config) Validate() error {
	if c.GeoIPDatabasePath == "" {
		return errors.New("a local geoIP database path must be provided")
	}
	return nil
}
