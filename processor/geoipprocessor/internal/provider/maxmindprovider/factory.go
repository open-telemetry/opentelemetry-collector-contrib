// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maxmind // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider/maxmindprovider"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
	"go.opentelemetry.io/collector/processor"
) // This file implements Factory for the MaxMind provider.

const (
	// TypeStr the value of "type" key in configuration.
	TypeStr = "maxmind"
)

// Factory is the Factory for scraper.
type Factory struct{}

var _ provider.GeoIPProviderFactory = (*Factory)(nil)

// CreateDefaultConfig creates the default configuration for the Provider.
func (f *Factory) CreateDefaultConfig() provider.Config {
	return &Config{}
}

// CreateGeoIPProvider creates a provider based on this config.
func (f *Factory) CreateGeoIPProvider(ctx context.Context, settings processor.Settings, cfg provider.Config) (provider.GeoIPProvider, error) {
	maxMindConfig := cfg.(*Config)
	return newMaxMindProvider(maxMindConfig)
}
