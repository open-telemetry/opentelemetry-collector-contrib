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

var (
	// ErrUnsetAPIKey is returned when the API key is not set.
	errUnsetAPIKey = datadogconfig.ErrUnsetAPIKey
	// ErrEmptyEndpoint is returned when endpoint is empty
	errEmptyEndpoint = datadogconfig.ErrEmptyEndpoint
	// ErrAPIKeyFormat is returned if API key contains invalid characters
	errAPIKeyFormat = datadogconfig.ErrAPIKeyFormat
	// NonHexRegex is a regex of characters that are always invalid in a Datadog API Key
	nonHexRegex = datadogconfig.NonHexRegex
)

var _ component.Config = (*Config)(nil)

const (
	// defaultSite is the default site for the Datadog API.
	defaultSite = datadogconfig.DefaultSite
	// defaultReporterPeriod is the default amount of time between sending fleet automation payloads to Datadog.
	// defaultReporterPeriod = 20 * time.Minute
)

// Config contains the information necessary for enabling the Datadog Fleet
// Automation Extension.
type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`
	API                     APIConfig `mapstructure:"api"`
	// If Hostname is empty extension will use available system APIs and cloud provider endpoints.
	Hostname string `mapstructure:"hostname"`
	// HTTPConfig is v2 config for the http metadata service.
	HTTPConfig *httpserver.Config `mapstructure:"http"`
}

// APIConfig contains the information necessary for configuring the Datadog API.
type APIConfig = datadogconfig.APIConfig

// Validate ensures that the configuration is valid.
func (c *Config) Validate() error {
	if c.API.Site == "" {
		return errEmptyEndpoint
	}
	if c.API.Key == "" {
		return errUnsetAPIKey
	}
	invalidAPIKeyChars := nonHexRegex.FindAllString(string(c.API.Key), -1)
	if len(invalidAPIKeyChars) > 0 {
		return fmt.Errorf("%w: invalid characters: %s", errAPIKeyFormat, strings.Join(invalidAPIKeyChars, ", "))
	}
	return nil
}
