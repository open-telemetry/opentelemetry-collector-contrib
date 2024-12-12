// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
)

const (
	scrapersKey = "scrapers"
)

// Config defines configuration for HostMetrics receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Scrapers                       map[string]internal.Config `mapstructure:"-"`
	// RootPath is the host's root directory (linux only).
	RootPath string `mapstructure:"root_path"`

	// Collection interval for metadata.
	// Metadata of the particular entity is collected when the entity changes.
	// In addition metadata of all entities is collected periodically even if no changes happen.
	// Setting the duration to 0 will disable periodic collection (however will not impact
	// metadata collection on changes).
	MetadataCollectionInterval time.Duration `mapstructure:"metadata_collection_interval"`
}

var (
	_ component.Config    = (*Config)(nil)
	_ confmap.Unmarshaler = (*Config)(nil)
)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	var err error
	if len(cfg.Scrapers) == 0 {
		err = multierr.Append(err, errors.New("must specify at least one scraper when using hostmetrics receiver"))
	}
	err = multierr.Append(err, validateRootPath(cfg.RootPath))
	return err
}

// Unmarshal a config.Parser into the config struct.
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return nil
	}

	// load the non-dynamic config normally
	err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused())
	if err != nil {
		return err
	}

	// dynamically load the individual collector configs based on the key name

	cfg.Scrapers = map[string]internal.Config{}

	scrapersSection, err := componentParser.Sub(scrapersKey)
	if err != nil {
		return err
	}

	for key := range scrapersSection.ToStringMap() {
		factory, ok := getScraperFactory(key)
		if !ok {
			return fmt.Errorf("invalid scraper key: %s", key)
		}

		collectorCfg := factory.CreateDefaultConfig()
		collectorViperSection, err := scrapersSection.Sub(key)
		if err != nil {
			return err
		}
		err = collectorViperSection.Unmarshal(collectorCfg)
		if err != nil {
			return fmt.Errorf("error reading settings for scraper type %q: %w", key, err)
		}

		collectorCfg.SetRootPath(cfg.RootPath)
		envMap := setGoPsutilEnvVars(cfg.RootPath, &osEnv{})
		collectorCfg.SetEnvMap(envMap)

		cfg.Scrapers[key] = collectorCfg
	}

	return nil
}
