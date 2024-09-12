// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

const (
	scrapersKey = "scrapers"
)

// Config that is exposed to this github receiver through the OTEL config.yaml
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Scrapers                       map[string]internal.Config `mapstructure:"scrapers"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	AccessToken                    configopaque.String `mapstructure:"access_token"`
	LogType                        string              `mapstructure:"log_type"`                // "user", "organization", or "enterprise"
	Name                           string              `mapstructure:"name"`                    // The name of the user, organization, or enterprise
	PollInterval                   time.Duration       `mapstructure:"poll_interval,omitempty"` // The interval at which to poll the GitHub API
}

var _ component.Config = (*Config)(nil)
var _ confmap.Unmarshaler = (*Config)(nil)

// Validate the configuration passed through the OTEL config.yaml
func (cfg *Config) Validate() error {
	if len(cfg.Scrapers) == 0 {
		return errors.New("must specify at least one scraper")
	}
	if cfg.AccessToken == "" {
		return fmt.Errorf("missing access_token; required")
	}
	if cfg.LogType == "" {
		return fmt.Errorf("missing log_type; required")
	}
	if cfg.Name == "" {
		return fmt.Errorf("missing name; required")
	}
	if cfg.PollInterval == 0 {
		return fmt.Errorf("missing poll_interval; required")
	}
	if cfg.PollInterval < time.Duration(float64(time.Second)*0.72) {
		return fmt.Errorf("invalid poll_interval; must be at least 0.72 seconds")
	}
	return nil
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
			return fmt.Errorf("invalid scraper key: %q", key)
		}

		collectorCfg := factory.CreateDefaultConfig()
		collectorSection, err := scrapersSection.Sub(key)
		if err != nil {
			return err
		}

		err = collectorSection.Unmarshal(collectorCfg)
		if err != nil {
			return fmt.Errorf("error reading settings for scraper type %q: %w", key, err)
		}

		cfg.Scrapers[key] = collectorCfg
	}

	return nil
}
