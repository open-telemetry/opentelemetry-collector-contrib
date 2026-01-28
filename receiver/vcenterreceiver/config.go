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
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

// Config is the configuration of the receiver
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	configtls.ClientConfig         `mapstructure:"tls,omitempty"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Endpoint                       string              `mapstructure:"endpoint"`
	Username                       string              `mapstructure:"username"`
	Password                       configopaque.String `mapstructure:"password"`
	// Scrapers defines which scraper groups are enabled.
	// If not specified, all scraper groups are enabled by default for backward compatibility.
	Scrapers map[ScraperGroup]ScraperConfig `mapstructure:"scrapers,omitempty"`
}

type ScraperConfig struct {
	Enabled bool `mapstructure:"enabled,omitempty"`
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

	// Validate scraper group names
	if c.Scrapers != nil {
		validGroups := make(map[ScraperGroup]bool)
		for _, group := range AllScraperGroups() {
			validGroups[group] = true
		}
		for group := range c.Scrapers {
			if !validGroups[group] {
				err = multierr.Append(err, fmt.Errorf("invalid scraper group: %q. Valid groups are: %v", group, AllScraperGroups()))
			}
		}
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
