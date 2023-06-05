// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package purefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver/internal"
)

var _ component.Config = (*Config)(nil)

// Config relating to Array Metric Scraper.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`

	// Settings contains settings for the individual scrapers
	Settings *Settings `mapstructure:"settings"`

	// Array represents the list of arrays to query
	Array []internal.ScraperConfig `mapstructure:"array"`

	// Hosts represents the list of hosts to query
	Hosts []internal.ScraperConfig `mapstructure:"hosts"`

	// Directories represents the list of directories to query
	Directories []internal.ScraperConfig `mapstructure:"directories"`

	// Pods represents the list of pods to query
	Pods []internal.ScraperConfig `mapstructure:"pods"`

	// Volumes represents the list of volumes to query
	Volumes []internal.ScraperConfig `mapstructure:"volumes"`

	// Env represents the respective environment value valid to scrape
	Env string `mapstructure:"env"`
}

type Settings struct {
	ReloadIntervals *ReloadIntervals `mapstructure:"reload_intervals"`
}

type ReloadIntervals struct {
	Array       time.Duration `mapstructure:"array"`
	Hosts       time.Duration `mapstructure:"hosts"`
	Directories time.Duration `mapstructure:"directories"`
	Pods        time.Duration `mapstructure:"pods"`
	Volumes     time.Duration `mapstructure:"volumes"`
}

func (c *Config) Validate() error {
	var errs error

	if c.Settings.ReloadIntervals.Array == 0 {
		errs = multierr.Append(errs, errors.New("reload interval for 'array' must be provided"))
	}
	if c.Settings.ReloadIntervals.Hosts == 0 {
		errs = multierr.Append(errs, errors.New("reload interval for 'hosts' must be provided"))
	}
	if c.Settings.ReloadIntervals.Directories == 0 {
		errs = multierr.Append(errs, errors.New("reload interval for 'directories' must be provided"))
	}
	if c.Settings.ReloadIntervals.Pods == 0 {
		errs = multierr.Append(errs, errors.New("reload interval for 'pods' must be provided"))
	}
	if c.Settings.ReloadIntervals.Volumes == 0 {
		errs = multierr.Append(errs, errors.New("reload interval for 'volumes' must be provided"))
	}

	return errs
}
