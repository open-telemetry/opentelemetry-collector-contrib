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

package newrelicexporter

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configparser"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// EndpointConfig defines configuration for a single endpoint in the New Relic exporter.
type EndpointConfig struct {
	// APIKey is the required authentication credentials for New Relic APIs. This field specifies the default key.
	APIKey string `mapstructure:"apikey"`

	// APIKeyHeader may be specified to instruct the exporter to extract the API key from the request context.
	APIKeyHeader string `mapstructure:"api_key_header"`

	// HostOverride overrides the endpoint.
	HostOverride string `mapstructure:"host_override"`

	// TimeoutSettings is the total amount of time spent attempting a request,
	// including retries, before abandoning and dropping data. Default is 5
	// seconds.
	TimeoutSettings exporterhelper.TimeoutSettings `mapstructure:",squash"`

	// RetrySettings defines configuration for retrying batches in case of export failure.
	// The current supported strategy is exponential backoff.
	RetrySettings exporterhelper.RetrySettings `mapstructure:"retry"`

	// Insecure disables TLS on the endpoint.
	insecure bool
}

// Config defines configuration options for the New Relic exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`

	// CommonConfig stores the base configuration for each endpoint.
	CommonConfig EndpointConfig `mapstructure:",squash"`

	// TracesConfig stores the configuration for the traces endpoint.
	TracesConfig EndpointConfig `mapstructure:"traces"`

	// MetricsConfig stores the configuration for the metrics endpoint.
	MetricsConfig EndpointConfig `mapstructure:"metrics"`

	// LogsConfig stores the configuration for the logs endpoint.
	LogsConfig EndpointConfig `mapstructure:"logs"`
}

func (c *Config) Unmarshal(componentSection *configparser.Parser) error {
	if err := componentSection.UnmarshalExact(c); err != nil {
		return err
	}

	baseEndpointConfig := c.CommonConfig
	endpointConfigs := map[string]*EndpointConfig{
		"metrics": &c.MetricsConfig,
		"traces":  &c.TracesConfig,
		"logs":    &c.LogsConfig,
	}

	for section, sectionCfg := range endpointConfigs {

		// Default to whatever the common config is
		*sectionCfg = baseEndpointConfig

		p, err := componentSection.Sub(section)
		if err != nil {
			return err
		}

		// Overlay with the section specific configuration
		if err := p.UnmarshalExact(sectionCfg); err != nil {
			return err
		}
	}
	return nil
}
