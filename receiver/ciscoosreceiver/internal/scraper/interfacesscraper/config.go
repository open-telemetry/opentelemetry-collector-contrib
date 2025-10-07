// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfacesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
)

// Config holds configuration for the interfaces scraper.
type Config struct {
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	// No interface filtering - collect all interfaces
	// Devices is passed from the main receiver config (not from YAML)
	Devices []DeviceConfig `mapstructure:"-"`
}

// DeviceConfig represents a single device configuration (flattened structure for scraper use)
type DeviceConfig struct {
	Host HostInfo
	Auth AuthConfig
}

// HostInfo contains host-specific information
type HostInfo struct {
	Name string
	IP   string
	Port int
}

// AuthConfig contains authentication information
type AuthConfig struct {
	Username string
	Password string
	KeyFile  string
}
