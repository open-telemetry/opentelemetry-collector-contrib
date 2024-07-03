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
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Targets                        []*targetConfig   `mapstructure:"targets"`
	Sequences                      []*sequenceConfig `mapstructure:"sequences"`
}

type targetConfig struct {
	confighttp.ClientConfig `mapstructure:",squash"`
	Method                  string `mapstructure:"method"`
}

type sequenceConfig struct {
	Name  string          `mapstructure:"name"`
	Steps []*sequenceStep `mapstructure:"steps"`
}

type sequenceStep struct {
	confighttp.ClientConfig `mapstructure:",squash"`
	Method                  string `mapstructure:"method"`
	TargetIndex             int    `mapstructure:"target_index"`
	ResponseRef             string `mapstructure:"response_ref"`
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *targetConfig) Validate() (err error) {
	if cfg.Endpoint == "" {
		err = multierr.Append(err, errMissingEndpoint)
		return
	}
	earl, parseErr := url.ParseRequestURI(cfg.Endpoint)
	if parseErr != nil {
		err = multierr.Append(err, fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), parseErr))
		return
	}
	if earl == nil || earl.Scheme == "" {
		err = multierr.Append(err, fmt.Errorf("%s: %s", errInvalidEndpoint.Error(), cfg.Endpoint))
	}
	return
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *Config) Validate() error {
	var err error

	if len(cfg.Targets) == 0 && len(cfg.Sequences) == 0 {
		err = multierr.Append(err, errors.New("no checks configured"))
	}

	for _, target := range cfg.Targets {
		err = multierr.Append(err, target.Validate())
	}

	for _, sequence := range cfg.Sequences {
		err = multierr.Append(err, sequence.Validate())
	}

	return err
}

func (sc *sequenceConfig) Validate() error {
	var err error
	if sc.Name == "" {
		err = multierr.Append(err, errors.New("sequence name must be specified"))
	}
	if len(sc.Steps) == 0 {
		err = multierr.Append(err, errors.New("sequence must have at least one step"))
	}

	for i, step := range sc.Steps {
		if step.Endpoint == "" {
			err = multierr.Append(err, fmt.Errorf("missing endpoint in step %d of sequence %s", i+1, sc.Name))
		} else {
			u, parseErr := url.ParseRequestURI(step.Endpoint) // url.Parse(step.Endpoint)
			if u == nil || parseErr != nil {
				err = multierr.Append(err, fmt.Errorf("invalid endpoint in step %d of sequence %s: %w", i+1, sc.Name, parseErr))
			}
		}
		if step.Method == "" {
			err = multierr.Append(err, fmt.Errorf("missing method in step %d of sequence %s", i+1, sc.Name))
		}
	}
	return err
}
