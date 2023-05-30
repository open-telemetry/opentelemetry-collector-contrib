// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

// Config is the configuration of the receiver
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	configtls.TLSClientSetting              `mapstructure:"tls,omitempty"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
	Endpoint                                string `mapstructure:"endpoint"`
	Username                                string `mapstructure:"username"`
	Password                                string `mapstructure:"password"`
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

	if _, tlsErr := c.LoadTLSConfig(); err != nil {
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
