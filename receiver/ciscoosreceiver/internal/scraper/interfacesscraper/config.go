// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package interfacesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/scraper/interfacesscraper/internal/metadata"
)

// Config holds configuration for the interfaces scraper
type Config struct {
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	Device                        DeviceConfig `mapstructure:"-"` // Passed from receiver config
}

// DeviceConfig represents device connection configuration
type DeviceConfig struct {
	Host HostInfo
	Auth AuthConfig
}

// HostInfo contains device host information
type HostInfo struct {
	Name string
	IP   string
	Port int
}

// AuthConfig contains SSH authentication credentials
type AuthConfig struct {
	Username string
	Password string
	KeyFile  string
}
