// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
)

// DeviceConfig represents configuration for a single Cisco device in the devices list.
type DeviceConfig struct {
	// DO NOT USE unkeyed struct initialization
	_ struct{} `mapstructure:"-"`

	Name string `mapstructure:"name"`
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`

	Auth connection.AuthConfig `mapstructure:"auth"`
}

// Config defines configuration for Cisco OS receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	// Devices is the list of Cisco devices to monitor.
	Devices []DeviceConfig `mapstructure:"devices"`

	Scrapers map[component.Type]component.Config `mapstructure:"-"`
}

var (
	_ xconfmap.Validator  = (*Config)(nil)
	_ confmap.Unmarshaler = (*Config)(nil)
)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	var err error

	if len(cfg.Devices) == 0 {
		err = multierr.Append(err, errors.New("must specify at least one device"))
	}

	if len(cfg.Scrapers) == 0 {
		err = multierr.Append(err, errors.New("must specify at least one scraper"))
	}

	for i, device := range cfg.Devices {
		if device.Host == "" {
			err = multierr.Append(err, fmt.Errorf("devices[%d].host cannot be empty", i))
		}
		if device.Port == 0 {
			err = multierr.Append(err, fmt.Errorf("devices[%d].port cannot be empty", i))
		}
		if device.Auth.Username == "" {
			err = multierr.Append(err, fmt.Errorf("devices[%d].auth.username cannot be empty", i))
		}
		if device.Auth.Password == "" && device.Auth.KeyFile == "" {
			err = multierr.Append(err, fmt.Errorf("devices[%d].auth.password or devices[%d].auth.key_file must be provided", i, i))
		}
	}

	return err
}

// Unmarshal a config.Parser into the config struct.
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return nil
	}

	// load the non-dynamic config normally
	if err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused()); err != nil {
		return err
	}

	// dynamically load the individual scraper configs based on the key name
	cfg.Scrapers = map[component.Type]component.Config{}

	scrapersSection, err := componentParser.Sub("scrapers")
	if err != nil {
		return err
	}

	for keyStr := range scrapersSection.ToStringMap() {
		key, err := component.NewType(keyStr)
		if err != nil {
			return fmt.Errorf("invalid scraper key name: %s", key)
		}

		factory, ok := scraperFactories[key]
		if !ok {
			return fmt.Errorf("invalid scraper key: %s", key)
		}

		scraperSection, err := scrapersSection.Sub(keyStr)
		if err != nil {
			return err
		}
		scraperCfg := factory.CreateDefaultConfig()
		if err = scraperSection.Unmarshal(scraperCfg); err != nil {
			return fmt.Errorf("error reading settings for scraper type %q: %w", key, err)
		}

		cfg.Scrapers[key] = scraperCfg
	}

	return nil
}
