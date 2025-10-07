// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"
)

// Config defines configuration for Cisco OS receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Devices                        []DeviceConfig `mapstructure:"devices"`
}

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

var _ xconfmap.Validator = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	var err error

	if len(cfg.Devices) == 0 {
		err = errors.New("at least one device must be configured")
	}

	for i, device := range cfg.Devices {
		if device.Device.Host.IP == "" {
			err = multierr.Append(err, fmt.Errorf("device[%d]: host.ip cannot be empty", i))
		}
		if device.Device.Host.Port == 0 {
			err = multierr.Append(err, fmt.Errorf("device[%d]: host.port cannot be empty", i))
		}
		if device.Auth.Username == "" {
			err = multierr.Append(err, fmt.Errorf("device[%d]: auth.username cannot be empty", i))
		}
		if device.Auth.Password == "" && device.Auth.KeyFile == "" {
			err = multierr.Append(err, fmt.Errorf("device[%d]: auth.password or auth.key_file must be provided", i))
		}
	}

	return err
}
