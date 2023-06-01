// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

var (
	errBadDataSource = errors.New("datasource is invalid")
	errBadEndpoint   = errors.New("endpoint must be specified as host:port")
	errBadPort       = errors.New("invalid port in endpoint")
	errEmptyEndpoint = errors.New("endpoint must be specified")
	errEmptyPassword = errors.New("password must be set")
	errEmptyService  = errors.New("service must be specified")
	errEmptyUsername = errors.New("username must be set")
)

type Config struct {
	DataSource                              string `mapstructure:"datasource"`
	Endpoint                                string `mapstructure:"endpoint"`
	Password                                string `mapstructure:"password"`
	Service                                 string `mapstructure:"service"`
	Username                                string `mapstructure:"username"`
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
}

func (c Config) Validate() error {
	var allErrs error

	// If DataSource is defined it takes precedence over the rest of the connection options.
	if c.DataSource == "" {

		if c.Endpoint == "" {
			allErrs = multierr.Append(allErrs, errEmptyEndpoint)
		}

		host, portStr, err := net.SplitHostPort(c.Endpoint)
		if err != nil {
			return multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadEndpoint, err))
		}

		if host == "" {
			allErrs = multierr.Append(allErrs, errBadEndpoint)
		}

		port, err := strconv.ParseInt(portStr, 10, 32)
		if err != nil {
			allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadPort, err))
		}

		if port < 0 || port > 65535 {
			allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %d", errBadPort, port))
		}

		if c.Username == "" {
			allErrs = multierr.Append(allErrs, errEmptyUsername)
		}

		if c.Password == "" {
			allErrs = multierr.Append(allErrs, errEmptyPassword)
		}

		if c.Service == "" {
			allErrs = multierr.Append(allErrs, errEmptyService)
		}
	} else {
		if _, err := url.Parse(c.DataSource); err != nil {
			allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadDataSource, err))
		}
	}
	return allErrs
}
