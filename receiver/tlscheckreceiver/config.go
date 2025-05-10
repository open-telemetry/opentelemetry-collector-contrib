// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var (
	errMissingURL = errors.New(`"url" must be specified`)
	errInvalidURL = errors.New(`"url" must be in the form of <scheme>://<hostname>[:<port>]`)
)

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Targets                        []*targetConfig `mapstructure:"targets"`
}

type targetConfig struct {
	URL string `mapstructure:"url"`
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *targetConfig) Validate() error {
	var err error

	if cfg.URL == "" {
		err = multierr.Append(err, errMissingURL)
	} else {
		_, parseErr := url.ParseRequestURI(cfg.URL)
		if parseErr != nil {
			err = multierr.Append(err, fmt.Errorf("%s: %w", errInvalidURL.Error(), parseErr))
		}
	}

	return err
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *Config) Validate() error {
	var err error

	if len(cfg.Targets) == 0 {
		err = multierr.Append(err, errMissingURL)
	}

	for _, target := range cfg.Targets {
		err = multierr.Append(err, target.Validate())
	}

	return err
}
