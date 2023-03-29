// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/multierr"
)

var (
    errMissingEndpointFromConfig = errors.New("Missing receiver server endpoint from config.")
)

// Config defines configuration for the Generic Webhook receiver.
type Config struct {
	confighttp.HTTPServerSettings `mapstructure:",squash"`      // squash ensures fields are correctly decoded in embedded struct
    ReadTimeout  string              `mapstructure:"read_timeout"`      // wait time for reading request headers in ms. Default is twenty seconds.
    WriteTimeout string             `mapstructure:"write_timeout"`      // wait time for writing request response in ms. Default is twenty seconds.
    Path         string           `mapstructure:"path"`        // path for data collection. Default is <host>:<port>/services/collector
    HealthPath   string           `mapstructure:"health_path"` // path for health check api. Default is /services/collector/health
}

func (cfg *Config) Validate() error {
    var errs error

    if cfg.HTTPServerSettings.Endpoint == "" {
        errs = multierr.Append(errs, errMissingEndpointFromConfig)
    }

    return errs
}
