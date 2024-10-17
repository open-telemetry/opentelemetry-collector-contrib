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
	errInvalidHost = errors.New(`"host" must be in the form of <hostname>:<port>`)
)

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Targets                        []*targetConfig `mapstructure:"targets"`
}

type targetConfig struct {
	Host string `mapstructure:"url"`
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *targetConfig) Validate() error {
	var err error

	if cfg.Host == "" {
		err = multierr.Append(err, ErrMissingTargets)
	} else {
		_, parseErr := url.ParseRequestURI(cfg.Host)
		if parseErr != nil {
			err = multierr.Append(err, fmt.Errorf("%s: %w", errInvalidHost.Error(), parseErr))
		}
	}

	return err
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *Config) Validate() error {
	var err error

	if len(cfg.Targets) == 0 {
		err = multierr.Append(err, ErrMissingTargets)
	}

	for _, target := range cfg.Targets {
		err = multierr.Append(err, target.Validate())
	}

	return err
}
