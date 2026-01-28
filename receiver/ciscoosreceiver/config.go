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

	// Scrapers is the scraper configuration with metric toggles.
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
		return err
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

var allowedKeys = map[string]struct{}{
	"collection_interval": {},
	"initial_delay":       {},
	"timeout":             {},
	"devices":             {},
	"scrapers":            {},
}

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return nil
	}

	for key := range componentParser.ToStringMap() {
		if _, ok := allowedKeys[key]; !ok {
			return fmt.Errorf("unknown configuration key: %q", key)
		}
	}

	cfg.Scrapers = map[component.Type]component.Config{}
	if componentParser.IsSet("scrapers") {
		scrapers, err := parseScrapersSection(componentParser, "scrapers")
		if err != nil {
			return fmt.Errorf("error parsing scrapers: %w", err)
		}
		cfg.Scrapers = scrapers
	}

	// WithIgnoreUnused() needed because "scrapers" has mapstructure:"-"
	if err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused()); err != nil {
		return err
	}

	return nil
}

func parseScrapersSection(parser *confmap.Conf, path string) (map[component.Type]component.Config, error) {
	scrapersSection, err := parser.Sub(path)
	if err != nil {
		return nil, err
	}

	scrapers := map[component.Type]component.Config{}
	for keyStr := range scrapersSection.ToStringMap() {
		key, err := component.NewType(keyStr)
		if err != nil {
			return nil, fmt.Errorf("invalid scraper key name: %s", keyStr)
		}

		factory, ok := scraperFactories[key]
		if !ok {
			return nil, fmt.Errorf("invalid scraper key: %s (available: %v)", key, getAvailableScraperTypes())
		}

		scraperCfg := factory.CreateDefaultConfig()
		scraperSubSection, err := scrapersSection.Sub(keyStr)
		if err != nil {
			return nil, fmt.Errorf("error getting scraper section for %q: %w", key, err)
		}
		if err = scraperSubSection.Unmarshal(scraperCfg); err != nil {
			return nil, fmt.Errorf("error reading settings for scraper type %q: %w", key, err)
		}

		scrapers[key] = scraperCfg
	}

	return scrapers, nil
}

func getAvailableScraperTypes() []string {
	types := make([]string, 0, len(scraperFactories))
	for key := range scraperFactories {
		types = append(types, key.String())
	}
	return types
}
