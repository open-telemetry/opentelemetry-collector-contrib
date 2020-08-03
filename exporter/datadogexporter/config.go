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
	unsetAPIKey = errors.New("Datadog API key is unset")
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

	// MetricsURL is the host of the Datadog intake server to send metrics to.
	// If not set, the value is obtained from the Site.
	MetricsURL string `mapstructure:"metrics_url"`
}

// Sanitize tries to sanitize a given configuration
func (c *Config) Sanitize() error {

	// Check API key is set
	if c.APIKey == "" {
		return unsetAPIKey
	}

	// Sanitize API key
	c.APIKey = strings.TrimSpace(c.APIKey)

	// Set Endpoint based on site if unset
	if c.MetricsURL == "" {
		c.MetricsURL = fmt.Sprintf("https://api.%s", c.Site)
	}

	return nil
}
