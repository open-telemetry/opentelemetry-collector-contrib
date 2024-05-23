package geoipprocessor

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/conventions"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
)

// Field represents an attribute used to look for the IP value.
type Field string

// Config holds the configuration for the GeoIP service.
type Config struct {
	// Providers specifies the sources to extract geographical information about a given IP (e.g., GeoLite2).
	Providers map[string]provider.Config `mapstructure:"providers"`

	// Metadata allows to extract geoIP metadata from a list of metadata fields.
	// The field accepts a list of strings.
	//
	// Metadata fields supported right now are,
	//   - geo.continent_name
	//   - geo.city_name
	//
	// Specifying anything other than these values will result in an error.
	// By default, the following fields are extracted and added to spans, metrics and logs as resource attributes:
	//   - geo.continent_name
	//   - geo.city_name
	Metadata []string `mapstructure:"metadata"`

	// Fields defines a list of attributes to look for the IP value.
	// The first matching attribute will be used to gather the IP geographical metadata.
	// Example:
	//   fields:
	//     - client.ip
	Fields []Field `mapstructure:"fields"`
}

func (cfg *Config) Validate() error {
	for _, field := range cfg.Metadata {
		switch field {
		case conventions.AttributeContinentName, conventions.AttributeGeoCityName:
		default:
			return fmt.Errorf("\"%s\" is not a supported metadata field", field)
		}
	}
	return nil
}
