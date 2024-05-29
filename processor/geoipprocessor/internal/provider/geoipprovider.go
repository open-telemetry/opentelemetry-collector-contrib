// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"net"

	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
)

// GeoIPProvider defines methods for obtaining the geographical location based on the provided IP address.
type GeoIPProvider interface {
	// Location returns a set of attributes representing the geographical location for the given IP address. It requires a context for managing request lifetime.
	Location(context.Context, net.IP) (attribute.Set, error)
}

// Config is the configuration of a GeoIPProvider.
type Config interface{}

// GeoIPProviderFactory can create GeoIPProvider instances.
type GeoIPProviderFactory interface {
	// CreateDefaultConfig creates the default configuration for the GeoIPProvider.
	CreateDefaultConfig() Config

	// CreateGeoIPProvider creates a provider based on this config.
	CreateGeoIPProvider(ctx context.Context, settings processor.CreateSettings, cfg Config) (GeoIPProvider, error)
}
