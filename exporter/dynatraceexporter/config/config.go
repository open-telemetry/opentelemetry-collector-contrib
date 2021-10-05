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

package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/apiconstants"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Config defines configuration for the Dynatrace exporter.
type Config struct {
	config.ExporterSettings       `mapstructure:",squash"`
	confighttp.HTTPClientSettings `mapstructure:",squash"`

	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`
	ResourceToTelemetrySettings  resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`

	// Dynatrace API token with metrics ingest permission
	APIToken string `mapstructure:"api_token"`

	// DefaultDimensions will be added to all exported metrics
	DefaultDimensions map[string]string `mapstructure:"default_dimensions"`

	// String to prefix all metric names
	Prefix string `mapstructure:"prefix"`

	// Tags will be added to all exported metrics
	// Deprecated: Please use DefaultDimensions instead
	Tags []string `mapstructure:"tags"`
}

// ValidateAndConfigureHTTPClientSettings validates the configuration and sets default values
func (c *Config) ValidateAndConfigureHTTPClientSettings() error {
	if c.HTTPClientSettings.Headers == nil {
		c.HTTPClientSettings.Headers = make(map[string]string)
	}
	c.APIToken = strings.TrimSpace(c.APIToken)

	if c.Endpoint == "" {
		c.Endpoint = apiconstants.GetDefaultOneAgentEndpoint()
	} else {
		if c.APIToken == "" {
			return errors.New("api_token is required if Endpoint is provided")
		}

		c.HTTPClientSettings.Headers["Authorization"] = fmt.Sprintf("Api-Token %s", c.APIToken)
	}

	if !(strings.HasPrefix(c.Endpoint, "http://") || strings.HasPrefix(c.Endpoint, "https://")) {
		return errors.New("endpoint must start with https:// or http://")
	}

	c.HTTPClientSettings.Headers["Content-Type"] = "text/plain; charset=UTF-8"
	c.HTTPClientSettings.Headers["User-Agent"] = "opentelemetry-collector"

	return nil
}
