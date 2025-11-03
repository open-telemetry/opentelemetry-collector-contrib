// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"

import "go.opentelemetry.io/collector/config/configopaque"

// DeviceConfig represents configuration for a single Cisco device using semantic conventions
type DeviceConfig struct {
	Device DeviceInfo `mapstructure:"device"`
	Auth   AuthConfig `mapstructure:"auth"`
}

// DeviceInfo follows semantic conventions for device identification
type DeviceInfo struct {
	// DO NOT USE unkeyed struct initialization
	_ struct{} `mapstructure:"-"`

	Host HostInfo `mapstructure:"host"`
}

// HostInfo contains host-specific information
type HostInfo struct {
	Name string `mapstructure:"name"`
	IP   string `mapstructure:"ip"`
	Port int    `mapstructure:"port"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Username string              `mapstructure:"username"`
	Password configopaque.String `mapstructure:"password"`
	KeyFile  string              `mapstructure:"key_file"`
}
