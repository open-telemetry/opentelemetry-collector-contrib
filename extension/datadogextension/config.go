// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

var _ component.Config = (*Config)(nil)

// Config contains the information necessary for enabling the Datadog Extension.
type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`
	// Define the site and API key (and whether to fail on invalid API key) in API.
	API datadogconfig.APIConfig `mapstructure:"api"`
	// If Hostname is empty extension will use available system APIs and cloud provider endpoints.
	Hostname string `mapstructure:"hostname"`
	// HTTPConfig is v2 config for the http metadata service.
	HTTPConfig *httpserver.Config `mapstructure:"http"`
}

// Validate ensures that the configuration is valid.
func (c *Config) Validate() error {
	if c.API.Site == "" {
		return datadogconfig.ErrEmptyEndpoint
	}
	if c.API.Key == "" {
		return datadogconfig.ErrUnsetAPIKey
	}
	if c.HTTPConfig == nil {
		return errors.New("http config is required")
	}
	return nil
}
