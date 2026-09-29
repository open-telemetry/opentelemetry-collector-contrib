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
	ControllerConfig     scraperhelper.ControllerConfig `mapstructure:",squash"`
	ClientConfig         configtls.ClientConfig         `mapstructure:"tls,omitempty"`
	MetricsBuilderConfig metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Endpoint             string                         `mapstructure:"endpoint"`
	Username             string                         `mapstructure:"username"`
	Password             configopaque.String            `mapstructure:"password"`
	ProxyURL             string                         `mapstructure:"proxy_url"`
	MaxQueryMetrics      int                            `mapstructure:"max_query_metrics"`
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

	if c.ProxyURL != "" {
		proxyURL, proxyErr := url.Parse(c.ProxyURL)
		switch {
		case proxyErr != nil:
			err = multierr.Append(err, fmt.Errorf("unable to parse proxy_url %s: %w", c.ProxyURL, proxyErr))
		case proxyURL.Scheme != "http" && proxyURL.Scheme != "https" &&
			proxyURL.Scheme != "socks5" && proxyURL.Scheme != "socks5h":
			err = multierr.Append(err, errors.New("proxy_url scheme must be http, https, socks5 or socks5h"))
		case proxyURL.Host == "":
			err = multierr.Append(err, errors.New("proxy_url must include a host"))
		}
	}

	if _, tlsErr := c.ClientConfig.LoadTLSConfig(context.Background()); tlsErr != nil {
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
