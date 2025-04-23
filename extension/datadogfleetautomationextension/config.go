// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogfleetautomationextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogfleetautomationextension"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogfleetautomationextension/internal/httpserver"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

var _ component.Config = (*Config)(nil)

// defaultReporterPeriod will be used or removed in future commits
// const (
// defaultReporterPeriod is the default amount of time between sending fleet automation payloads to Datadog.
// defaultReporterPeriod = 20 * time.Minute
// )

// Config contains the information necessary for enabling the Datadog Fleet
// Automation Extension.
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
	invalidAPIKeyChars := datadogconfig.NonHexRegex.FindAllString(string(c.API.Key), -1)
	if len(invalidAPIKeyChars) > 0 {
		return fmt.Errorf("%w: invalid characters: %s", datadogconfig.ErrAPIKeyFormat, strings.Join(invalidAPIKeyChars, ", "))
	}
	return nil
}
