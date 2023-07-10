// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snowflakereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver/internal/metadata"
)

var (
	errMissingUsername  = errors.New("You must provide a valid snowflake username")
	errMissingPassword  = errors.New("You must provide a password for the snowflake username")
	errMissingAccount   = errors.New("You must provide a valid account name")
	errMissingWarehouse = errors.New("You must provide a valid warehouse name")
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
	Username                                string `mapstructure:"username"`
	Password                                configopaque.String `mapstructure:"password"`
	Account                                 string `mapstructure:"account"`
	Schema                                  string `mapstructure:"schema"`
	Warehouse                               string `mapstructure:"warehouse"`
	Database                                string `mapstructure:"database"`
	Role                                    string `mapstructure:"role"`
}

func (cfg *Config) Validate() error {
	var errs error
	if cfg.Username == "" {
		errs = multierr.Append(errs, errMissingUsername)
	}

	if cfg.Password == "" {
		errs = multierr.Append(errs, errMissingPassword)
	}

	if cfg.Account == "" {
		errs = multierr.Append(errs, errMissingAccount)
	}

	if cfg.Warehouse == "" {
		errs = multierr.Append(errs, errMissingWarehouse)
	}

	return errs
}
