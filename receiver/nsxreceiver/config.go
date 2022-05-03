// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nsxreceiver

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver"
)

// Config is the configuration for the NSX receiver
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	MetricsConfig           *MetricsConfig `mapstructure:"metrics"`
	LoggingConfig           *LoggingConfig `mapstructure:"logs"`
}

// MetricsConfig is the metrics configuration portion of the nsxreceiver
type MetricsConfig struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	Settings                                metadata.MetricsSettings `mapstructure:"settings"`
	Username                                string                   `mapstructure:"username"`
	Password                                string                   `mapstructure:"password"`
}

// LoggingConfig is the configuration of a syslog receiver
type LoggingConfig struct {
	*syslogreceiver.SysLogConfig `mapstructure:",squash"`
}

// Validate returns if the NSX configuration is valid
func (c *Config) Validate() error {
	return multierr.Combine(
		c.validateMetrics(),
		c.validateLogs(),
	)
}

func (c *Config) validateMetrics() error {
	mc := c.MetricsConfig
	// do not require the configuration validation to specify metrics if not needed
	// creating a default config by default creates a non-nil metrics config
	if mc == nil || mc.Endpoint == "" {
		return nil
	}

	var err error
	res, err := url.Parse(mc.Endpoint)
	if err != nil {
		err = multierr.Append(err, fmt.Errorf("unable to parse url %s: %w", c.MetricsConfig.Endpoint, err))
		return err
	}

	if res.Scheme != "http" && res.Scheme != "https" {
		err = multierr.Append(err, errors.New("url scheme must be http or https"))
	}

	if mc.Username == "" {
		err = multierr.Append(err, errors.New("username not provided and is required"))
	}

	if mc.Password == "" {
		err = multierr.Append(err, errors.New("password not provided and is required"))
	}
	return err
}

func (c *Config) validateLogs() error {
	if c.LoggingConfig != nil {
		return c.LoggingConfig.Validate()
	}
	return nil
}

// ID returns the underlying MetricsConfig's ID
func (c *Config) ID() config.ComponentID {
	return c.MetricsConfig.ID()
}
