// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/metadata"
)

// Config represents the receiver configuration
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`

	Devices  []DeviceConfig `mapstructure:"devices"`
	Scrapers ScrapersConfig `mapstructure:"scrapers"`
}

// DeviceConfig represents configuration for a single Cisco device
type DeviceConfig struct {
	Host     string `mapstructure:"host"`
	KeyFile  string `mapstructure:"key_file"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// ScrapersConfig represents which scrapers are enabled
type ScrapersConfig struct {
	BGP         bool `mapstructure:"bgp"`
	Environment bool `mapstructure:"environment"`
	Facts       bool `mapstructure:"facts"`
	Interfaces  bool `mapstructure:"interfaces"`
	Optics      bool `mapstructure:"optics"`
}

// Validate checks if the receiver configuration is valid
func (cfg *Config) Validate() error {
	if len(cfg.Devices) == 0 {
		return errors.New("at least one device must be configured")
	}

	for _, device := range cfg.Devices {
		if device.Host == "" {
			return errors.New("device host cannot be empty")
		}

		// Authentication validation logic:
		// 1. If using key file: username + key_file (password optional)
		// 2. If not using key file: username + password required
		if device.KeyFile != "" {
			// Key file authentication: requires username
			if device.Username == "" {
				return errors.New("device username cannot be empty")
			}
		} else {
			// Password authentication: requires both username and password
			if device.Username == "" {
				return errors.New("device username cannot be empty")
			}
			if device.Password == "" {
				return errors.New("device password cannot be empty")
			}
		}
	}

	// Check if at least one scraper is enabled
	if !cfg.Scrapers.BGP && !cfg.Scrapers.Environment && !cfg.Scrapers.Facts && !cfg.Scrapers.Interfaces && !cfg.Scrapers.Optics {
		return errors.New("at least one scraper must be enabled")
	}

	return nil
}
