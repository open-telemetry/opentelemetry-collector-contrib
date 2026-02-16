// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

var (
	_ xconfmap.Validator  = (*Config)(nil)
	_ confmap.Unmarshaler = (*Config)(nil)
)

// Config is the configuration of the receiver
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	configtls.ClientConfig         `mapstructure:"tls,omitempty"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Endpoint                       string              `mapstructure:"endpoint"`
	Username                       string              `mapstructure:"username"`
	Password                       configopaque.String `mapstructure:"password"`
	// Scrapers configures which metric groups are enabled. Nil means all groups enabled (backward compatible).
	Scrapers map[ScraperGroup]ScraperConfig `mapstructure:"-"`
}

// Implements the Unmarshaler interface
func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		return nil
	}
	if err := componentParser.Unmarshal(cfg, confmap.WithIgnoreUnused()); err != nil {
		return err
	}

	scrapersSection, err := componentParser.Sub("scrapers")
	if err != nil || scrapersSection == nil {
		return nil
	}
	scraperKeys := scrapersSection.ToStringMap()
	if len(scraperKeys) == 0 {
		return nil
	}

	cfg.Scrapers = make(map[ScraperGroup]ScraperConfig)
	validGroups := validScraperGroupSet()

	for keyStr := range scraperKeys {
		group := ScraperGroup(keyStr)
		if !validGroups[group] {
			return fmt.Errorf("invalid scraper group %q", keyStr)
		}
		scraperSection, err := scrapersSection.Sub(keyStr)
		if err != nil {
			return err
		}
		scraperCfg := ScraperConfig{Enabled: true}
		if err := scraperSection.Unmarshal(&scraperCfg); err != nil {
			return fmt.Errorf("error reading settings for scraper group %q: %w", keyStr, err)
		}
		cfg.Scrapers[group] = scraperCfg
	}
	return nil
}

// Validate checks to see if the supplied config will work for the receiver
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("no endpoint was provided")
	}

	var err error
	res, err := url.Parse(c.Endpoint)
	if err != nil {
		err = multierr.Append(err, fmt.Errorf("unable to parse url %s: %w", c.Endpoint, err))
		return err
	}

	if res.Scheme != "http" && res.Scheme != "https" {
		err = multierr.Append(err, errors.New("url scheme must be http or https"))
	}

	if c.Username == "" {
		err = multierr.Append(err, errors.New("username not provided and is required"))
	}

	if c.Password == "" {
		err = multierr.Append(err, errors.New("password not provided and is required"))
	}

	if _, tlsErr := c.LoadTLSConfig(context.Background()); tlsErr != nil {
		err = multierr.Append(err, fmt.Errorf("error loading tls configuration: %w", tlsErr))
	}

	return err
}

// SDKUrl returns the url for the vCenter SDK
func (c *Config) SDKUrl() (*url.URL, error) {
	res, err := url.Parse(c.Endpoint)
	if err != nil {
		return res, err
	}
	res.Path = "/sdk"
	return res, nil
}
