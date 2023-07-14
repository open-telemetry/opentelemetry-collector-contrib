// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitproviderreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitproviderreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitproviderreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitproviderreceiver/internal/metadata"
)

const (
	scrapersKey = "scrapers"
)

// Config that is exposed to this github receiver through the OTEL config.yaml
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	Scrapers                                map[string]internal.Config `mapstructure:"scrapers"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
}

var _ component.Config = (*Config)(nil)
var _ confmap.Unmarshaler = (*Config)(nil)

// Validate the configuration passed through the OTEL config.yaml
func (cfg *Config) Validate() error {
	if len(cfg.Scrapers) == 0 {
		return errors.New("must specify at least one scraper")
	}
	return nil
}

// Unmarshal a config.Parser into the config struct.
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return nil
	}

	// load the non-dynamic config normally
	err := componentParser.Unmarshal(cfg)
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
			return fmt.Errorf("invalid scraper key: %q", key)
		}

		collectorCfg := factory.CreateDefaultConfig()
		collectorViperSection, err := scrapersSection.Sub(key)
		if err != nil {
			return err
		}

		err = collectorViperSection.Unmarshal(collectorCfg, confmap.WithErrorUnused())
		if err != nil {
			return fmt.Errorf("error reading settings for scraper type %q: %w", key, err)
		}

		cfg.Scrapers[key] = collectorCfg
	}

	return nil
}
