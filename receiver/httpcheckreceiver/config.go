// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var (
	errMissingEndpoint = errors.New(`"endpoint" must be specified`)
	errInvalidEndpoint = errors.New(`"endpoint" must be in the form of <scheme>://<hostname>[:<port>]`)
)

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
	Targets                                 []*targetConfig `mapstructure:"targets"`
}

type targetConfig struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	Method                        string `mapstructure:"method"`
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *targetConfig) Validate() error {
	var err error

	if cfg.Endpoint == "" {
		err = multierr.Append(err, errMissingEndpoint)
	} else {
		_, parseErr := url.ParseRequestURI(cfg.Endpoint)
		if parseErr != nil {
			err = multierr.Append(err, fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), parseErr))
		}
	}

	return err
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *Config) Validate() error {
	var err error

	if len(cfg.Targets) == 0 {
		err = multierr.Append(err, errors.New("no targets configured"))
	}

	for _, target := range cfg.Targets {
		err = multierr.Append(err, target.Validate())
	}

	return err
}
