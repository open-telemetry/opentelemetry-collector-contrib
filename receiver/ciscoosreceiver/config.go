// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ciscoosreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver"

import (
	"errors"
	"fmt"
	"maps"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"
)

// DeviceConfig represents configuration for a single Cisco device in the devices list.
type DeviceConfig struct {
	Name string `mapstructure:"name"`
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`

	Auth connection.AuthConfig `mapstructure:"auth"`

	// Scrapers is the resolved scraper configuration for this device.
	Scrapers map[component.Type]component.Config `mapstructure:"-"`
}

// Config defines configuration for Cisco OS receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	// Devices is the list of Cisco devices to monitor.
	Devices []DeviceConfig `mapstructure:"devices"`

	// Scrapers is the global scraper configuration with metric toggles.
	// Applied to all devices unless a device has its own scrapers section.
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
		if len(device.Scrapers) == 0 {
			err = multierr.Append(err, fmt.Errorf("devices[%d] must have at least one scraper (from global or per-device)", i))
		}
	}

	return err
}

// Unmarshal a config.Parser into the config struct.
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return nil
	}

	// Load the non-dynamic config normally (devices list, collection_interval, timeout)
	if err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused()); err != nil {
		return err
	}

	// Parse global scrapers section
	cfg.Scrapers = map[component.Type]component.Config{}
	if componentParser.IsSet("scrapers") {
		globalScrapers, err := parseScrapersSection(componentParser, "scrapers")
		if err != nil {
			return fmt.Errorf("error parsing global scrapers: %w", err)
		}
		cfg.Scrapers = globalScrapers
	}

	devicesRaw := componentParser.Get("devices")
	if devicesRaw != nil {
		devicesSlice, ok := devicesRaw.([]any)
		if !ok {
			return errors.New("devices must be a list")
		}

		for i := range cfg.Devices {
			if i >= len(devicesSlice) {
				break
			}

			deviceMap, ok := devicesSlice[i].(map[string]any)
			if !ok {
				continue
			}

			// Check if this device has its own scrapers section
			scrapersRaw, hasScrapers := deviceMap["scrapers"]
			if !hasScrapers || scrapersRaw == nil {
				cfg.Devices[i].Scrapers = cloneScraperConfigs(cfg.Scrapers)
			} else {
				scrapersMap, ok := scrapersRaw.(map[string]any)
				if !ok {
					return fmt.Errorf("devices[%d].scrapers must be a map", i)
				}

				deviceScrapers, err := parseScrapersFromMap(scrapersMap)
				if err != nil {
					return fmt.Errorf("error parsing scrapers for device %d: %w", i, err)
				}

				cfg.Devices[i].Scrapers = mergeScraperConfigs(cfg.Scrapers, deviceScrapers)
			}
		}
	} else {
		for i := range cfg.Devices {
			cfg.Devices[i].Scrapers = cloneScraperConfigs(cfg.Scrapers)
		}
	}

	return nil
}

// parseScrapersSection parses a scrapers section from the config at the given path.
func parseScrapersSection(parser *confmap.Conf, path string) (map[component.Type]component.Config, error) {
	scrapersSection, err := parser.Sub(path)
	if err != nil {
		return nil, err
	}
	return parseScrapersSectionFromConf(scrapersSection)
}

// parseScrapersSectionFromConf parses scrapers from a confmap.Conf section.
func parseScrapersSectionFromConf(scrapersSection *confmap.Conf) (map[component.Type]component.Config, error) {
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

// parseScrapersFromMap parses scrapers from a raw map[string]any (used for per-device scrapers).
func parseScrapersFromMap(scrapersMap map[string]any) (map[component.Type]component.Config, error) {
	scrapers := map[component.Type]component.Config{}

	for keyStr, scraperData := range scrapersMap {
		key, err := component.NewType(keyStr)
		if err != nil {
			return nil, fmt.Errorf("invalid scraper key name: %s", keyStr)
		}

		factory, ok := scraperFactories[key]
		if !ok {
			return nil, fmt.Errorf("invalid scraper key: %s (available: %v)", key, getAvailableScraperTypes())
		}

		scraperCfg := factory.CreateDefaultConfig()

		if scraperData != nil {
			scraperDataMap, ok := scraperData.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("scraper %q config must be a map", keyStr)
			}
			scraperConf := confmap.NewFromStringMap(scraperDataMap)
			if err = scraperConf.Unmarshal(scraperCfg); err != nil {
				return nil, fmt.Errorf("error reading settings for scraper type %q: %w", key, err)
			}
		}

		scrapers[key] = scraperCfg
	}

	return scrapers, nil
}

// cloneScraperConfigs creates a shallow copy of scraper configs map.
func cloneScraperConfigs(src map[component.Type]component.Config) map[component.Type]component.Config {
	if src == nil {
		return nil
	}
	dst := make(map[component.Type]component.Config, len(src))
	maps.Copy(dst, src)
	return dst
}

// mergeScraperConfigs merges device-specific scrapers with global scrapers.
func mergeScraperConfigs(global, device map[component.Type]component.Config) map[component.Type]component.Config {
	if len(device) == 0 {
		return cloneScraperConfigs(global)
	}

	result := make(map[component.Type]component.Config, len(device))
	maps.Copy(result, device)
	return result
}

// getAvailableScraperTypes returns a list of available scraper types for error messages
func getAvailableScraperTypes() []string {
	types := make([]string, 0, len(scraperFactories))
	for key := range scraperFactories {
		types = append(types, key.String())
	}
	return types
}
