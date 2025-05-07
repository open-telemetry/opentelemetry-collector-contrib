// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var (
	errInvalidEndpoint = errors.New(`"endpoint" must be in the form of <scheme>://<hostname>[:<port>]`)
	errMissingEndpoint = errors.New("at least one of 'endpoint' or 'endpoints' must be specified")
)

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Targets                        []*targetConfig `mapstructure:"targets"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// targetConfig defines configuration for individual HTTP checks.
type targetConfig struct {
	confighttp.ClientConfig `mapstructure:",squash"`
	Method                  string   `mapstructure:"method"`
	Endpoints               []string `mapstructure:"endpoints"` // Field for a list of endpoints
}

// Validate validates an individual targetConfig.
func (cfg *targetConfig) Validate() error {
	var err error

	// Ensure at least one of 'endpoint' or 'endpoints' is specified.
	if cfg.Endpoint == "" && len(cfg.Endpoints) == 0 {
		err = multierr.Append(err, errMissingEndpoint)
	}

	// Validate the single endpoint in ClientConfig.
	if cfg.Endpoint != "" {
		if _, parseErr := url.ParseRequestURI(cfg.Endpoint); parseErr != nil {
			err = multierr.Append(err, fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), parseErr))
		}
	}

	// Validate each endpoint in the Endpoints list.
	for _, endpoint := range cfg.Endpoints {
		if _, parseErr := url.ParseRequestURI(endpoint); parseErr != nil {
			err = multierr.Append(err, fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), parseErr))
		}
	}

	return err
}

// Validate validates the top-level Config by checking each targetConfig.
func (cfg *Config) Validate() error {
	var err error

	// Ensure at least one target is configured.
	if len(cfg.Targets) == 0 {
		err = multierr.Append(err, errors.New("no targets configured"))
	}

	// Validate each targetConfig.
	for _, target := range cfg.Targets {
		err = multierr.Append(err, target.Validate())
	}

	return err
}
