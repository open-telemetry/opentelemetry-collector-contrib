// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package purefbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefbreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefbreceiver/internal"
)

var _ component.Config = (*Config)(nil)

// Config relating to Array Metric Scraper.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`

	// Settings contains settings for the individual scrapers
	Settings *Settings `mapstructure:"settings"`

	// Arrays represents the list of arrays to query
	Arrays []internal.ScraperConfig `mapstructure:"arrays"`

	// Clients represents the list of clients metrics
	Clients []internal.ScraperConfig `mapstructure:"clients"`

	// Usage represents the list of usage to query
	Usage []internal.ScraperConfig `mapstructure:"usage"`

	// Env represents the respective environment value valid to scrape
	Env string `mapstructure:"env"`
}

type Settings struct {
	ReloadIntervals *ReloadIntervals `mapstructure:"reload_intervals"`
}

type ReloadIntervals struct {
	Array   time.Duration `mapstructure:"array"`
	Clients time.Duration `mapstructure:"clients"`
	Usage   time.Duration `mapstructure:"usage"`
}

func (c *Config) Validate() error {
	var err error

	if c.Settings.ReloadIntervals.Array == 0 {
		err = multierr.Append(err, errors.New("reload interval for 'arrays' must be provided"))
	}
	if c.Settings.ReloadIntervals.Clients == 0 {
		err = multierr.Append(err, errors.New("reload interval for 'clients' must be provided"))
	}
	if c.Settings.ReloadIntervals.Usage == 0 {
		err = multierr.Append(err, errors.New("reload interval for 'usage' must be provided"))
	}

	return err
}
