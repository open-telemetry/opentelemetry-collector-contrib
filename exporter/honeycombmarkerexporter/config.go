// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the Honeycomb Marker exporter.
type Config struct {
	// APIKey is the authentication token associated with the Honeycomb account.
	APIKey string `mapstructure:"api_key"`

	// API URL to use (defaults to https://api.honeycomb.io)
	APIURL string `mapstructure:"api_url"`

	// Markers is the list of markers to create
	Markers []marker `mapstructure:"markers"`
}

type marker struct {
	// MarkerType defines the type of marker.  Markers with the same type appear in Honeycomb with the same color
	MarkerType string `mapstructure:"type"`

	// MarkerColor is the color of the marker. Will only be used if the MarkerType does not already exist.
	MarkerColor string `mapstructure:"color"`

	// MessageField is the attribute that will be used as the message.
	// If necessary the value will be converted to a string.
	MessageField string `mapstructure:"message_field"`

	// UrlField is the attribute that will be used as the url.
	// If necessary the value will be converted to a string.
	UrlField string `mapstructure:"url_field"`

	// Rules are the OTTL rules that determine when a piece of telemetry should be turned into a Marker
	Rules Rules `mapstructure:"rules"`
}

type Rules struct {
	// ResourceConditions is the list of ottlresource conditions that determine a match
	ResourceConditions []string `mapstructure:"resource_conditions"`

	// LogConditions is the list of ottllog conditions that determine a match
	LogConditions []string `mapstructure:"log_conditions"`
}

func (cfg *Config) Validate() error {
	if cfg.APIKey == "" {
		return fmt.Errorf("invalid API Key")
	}

	if cfg.APIURL == "" {
		return fmt.Errorf("invalid URL")
	}

	return nil
}

var _ component.Config = (*Config)(nil)
