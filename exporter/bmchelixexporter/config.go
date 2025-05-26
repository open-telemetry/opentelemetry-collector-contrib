// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bmchelixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
)

// Config struct is used to store the configuration of the exporter
type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`
	APIKey                  configopaque.String       `mapstructure:"api_key"`
	RetryConfig             configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

// validate the configuration
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if c.APIKey == "" {
		return errors.New("api key is required")
	}
	if c.Timeout <= 0 {
		return errors.New("timeout must be a positive integer")
	}

	return nil
}
