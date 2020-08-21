// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datadogexporter

import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/config/configmodels"
)

var (
	errUnsetAPIKey = errors.New("the Datadog API key is unset")
	errConflict    = errors.New("'site' and 'api_key' must not be set with agent mode")
)

// Config defines configuration for the Datadog exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// ApiKey is the Datadog API key to associate your Agent's data with your organization.
	// Create a new API key here: https://app.datadoghq.com/account/settings
	APIKey string `mapstructure:"api_key"`

	// Site is the site of the Datadog intake to send data to.
	// The default value is "datadoghq.com".
	Site string `mapstructure:"site"`

	// MetricsURL is the host of the Datadog intake server or Dogstatsd server to send metrics to.
	// If not set, the value is obtained from the Site and the sending method.
	MetricsURL string `mapstructure:"metrics_url"`

	// Tags is the list of default tags to add to every metric or trace
	Tags []string `mapstructure:"tags"`

	// Mode states the mode for sending metrics and traces.
	// The possible values are "api" and "agent".
	// The default value is "agent".
	Mode string `mapstructure:"sending_method"`
}

// Sanitize tries to sanitize a given configuration
func (c *Config) Sanitize() error {
	if c.Mode == AgentMode {

		if c.APIKey != "" || c.Site != DefaultSite {
			return errConflict
		}

		if c.MetricsURL == "" {
			c.MetricsURL = "127.0.0.1:8125"
		}

	} else if c.Mode == APIMode {

		if c.APIKey == "" {
			return errUnsetAPIKey
		}

		// Sanitize API key
		c.APIKey = strings.TrimSpace(c.APIKey)

		if c.MetricsURL == "" {
			c.MetricsURL = fmt.Sprintf("https://api.%s", c.Site)
		}

	} else {
		return fmt.Errorf("Selected mode '%s' is invalid", c.Mode)
	}

	return nil
}
