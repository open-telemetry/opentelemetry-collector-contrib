// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

const defaultEndpoint = "http://localhost:5984"

var (
	// Errors for missing required config fields.
	errMissingUsername = errors.New(`no "username" specified in config`)
	errMissingPassword = errors.New(`no "password" specified in config`)

	// Errors for invalid url components in the endpoint.
	errInvalidEndpoint = errors.New(`"endpoint" %q must be in the form of <scheme>://<hostname>:<port>`)
)

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
	Username                                string              `mapstructure:"username"`
	Password                                configopaque.String `mapstructure:"password"`
}

// Validate validates missing and invalid configuration fields.
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
		err = multierr.Append(err, fmt.Errorf(errInvalidEndpoint.Error(), parseErr))
	}
	return err
}
