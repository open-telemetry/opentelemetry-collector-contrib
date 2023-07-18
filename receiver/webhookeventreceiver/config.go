// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/multierr"
)

var (
	errMissingEndpointFromConfig   = errors.New("missing receiver server endpoint from config")
	errReadTimeoutExceedsMaxValue  = errors.New("The duration specified for read_timeout exceeds the maximum allowed value of 10s")
	errWriteTimeoutExceedsMaxValue = errors.New("The duration specified for write_timeout exceeds the maximum allowed value of 10s")
	errRequiredHeader              = errors.New("both key and value are required to assign a required_header")
)

// Config defines configuration for the Generic Webhook receiver.
type Config struct {
	confighttp.HTTPServerSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	ReadTimeout                   string                   `mapstructure:"read_timeout"`  // wait time for reading request headers in ms. Default is twenty seconds.
	WriteTimeout                  string                   `mapstructure:"write_timeout"` // wait time for writing request response in ms. Default is twenty seconds.
	Path                          string                   `mapstructure:"path"`          // path for data collection. Default is <host>:<port>/services/collector
	HealthPath                    string                   `mapstructure:"health_path"`   // path for health check api. Default is /services/collector/health
	RequiredHeader                RequiredHeader                   `mapstructure:"required_header"`   // optional setting to set a required header for all requests to have
}

type RequiredHeader struct {
	Key													string `mapstructure:"key"`
	Value													string `mapstructure:"value"`
}

func (cfg *Config) Validate() error {
	var errs error

	maxReadWriteTimeout, _ := time.ParseDuration("10s")

	if cfg.HTTPServerSettings.Endpoint == "" {
		errs = multierr.Append(errs, errMissingEndpointFromConfig)
	}

	// If a user defines a custom read/write timeout there is a maximum value
	// of 10s imposed here.
	if cfg.ReadTimeout != "" {
		readTimeout, err := time.ParseDuration(cfg.ReadTimeout)
		if err != nil {
			errs = multierr.Append(errs, err)
		}

		if readTimeout > maxReadWriteTimeout {
			errs = multierr.Append(errs, errReadTimeoutExceedsMaxValue)
		}
	}

	if cfg.WriteTimeout != "" {
		writeTimeout, err := time.ParseDuration(cfg.WriteTimeout)
		if err != nil {
			errs = multierr.Append(errs, err)
		}

		if writeTimeout > maxReadWriteTimeout {
			errs = multierr.Append(errs, errWriteTimeoutExceedsMaxValue)
		}
	}

	if (cfg.RequiredHeader.Key != "" && cfg.RequiredHeader.Value == "") || (cfg.RequiredHeader.Value != "" && cfg.RequiredHeader.Key == "") {
		errs = multierr.Append(errs, errRequiredHeader)
	}

	return errs
}
