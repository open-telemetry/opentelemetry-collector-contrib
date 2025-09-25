// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver"

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
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

// IndividualQueryConfig represents individual query filtering configuration
type IndividualQueryConfig struct {
	Enabled        bool     `mapstructure:"enabled"`
	SearchText     string   `mapstructure:"search_text"`
	ExcludeSchemas []string `mapstructure:"exclude_schemas"`
	MaxQueries     int      `mapstructure:"max_queries"`
}

type Config struct {
	DataSource                     string                 `mapstructure:"datasource"`
	Endpoint                       string                 `mapstructure:"endpoint"`
	Password                       string                 `mapstructure:"password"`
	Service                        string                 `mapstructure:"service"`
	Username                       string                 `mapstructure:"username"`
	IndividualQuerySettings        *IndividualQueryConfig `mapstructure:"individual_query_settings"`
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
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
			return multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadEndpoint, err.Error()))
		}

		if host == "" {
			allErrs = multierr.Append(allErrs, errBadEndpoint)
		}

		port, err := strconv.ParseInt(portStr, 10, 32)
		if err != nil {
			allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadPort, err.Error()))
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
			allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadDataSource, err.Error()))
		}
	}

	return allErrs
}
