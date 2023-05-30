// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package riakreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var (
	errMissingUsername = errors.New(`"username" not specified in config`)
	errMissingPassword = errors.New(`"password" not specified in config`)

	errInvalidEndpoint = errors.New(`"endpoint" must be in the form of <scheme>://<hostname>:<port>`)
)

const defaultEndpoint = "http://localhost:8098"

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	Username                                string                        `mapstructure:"username"`
	Password                                string                        `mapstructure:"password"`
	MetricsBuilderConfig                    metadata.MetricsBuilderConfig `mapstructure:"metrics"`
}

// Validate validates the configuration by checking for missing or invalid fields
func (cfg *Config) Validate() error {
	var err error
	if cfg.Username == "" {
		err = multierr.Append(err, errMissingUsername)
	}

	if cfg.Password == "" {
		err = multierr.Append(err, errMissingPassword)
	}

	_, parseErr := url.Parse(cfg.Endpoint)
	if parseErr != nil {
		wrappedErr := fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), parseErr)
		err = multierr.Append(err, wrappedErr)
	}

	return err
}
