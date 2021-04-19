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
	"time"

	"go.opentelemetry.io/collector/config"
)

// EndpointConfig defines configuration for a single endpoint in the New Relic exporter.
type EndpointConfig struct {
	// APIKey is the required authentication credentials for New Relic APIs. This field specifies the default key.
	APIKey string `mapstructure:"apikey"`

	// APIKeyHeader may be specified to instruct the exporter to extract the API key from the request context.
	APIKeyHeader string `mapstructure:"api_key_header"`

	// HostOverride overrides the endpoint.
	HostOverride string `mapstructure:"host_override"`

	// Timeout is the total amount of time spent attempting a request,
	// including retries, before abandoning and dropping data. Default is 15
	// seconds.
	Timeout time.Duration `mapstructure:"timeout"`

	// Insecure disables TLS on the endpoint.
	insecure bool
}

// Config defines configuration options for the New Relic exporter.
type Config struct {
	*config.ExporterSettings `mapstructure:"-"`

	// CommonConfig stores the base configuration for each endpoint.
	CommonConfig EndpointConfig `mapstructure:",squash"`

	// TracesConfig stores the configuration for the traces endpoint.
	TracesConfig EndpointConfig `mapstructure:"traces"`

	// MetricsConfig stores the configuration for the metrics endpoint.
	MetricsConfig EndpointConfig `mapstructure:"metrics"`

	// LogsConfig stores the configuration for the logs endpoint.
	LogsConfig EndpointConfig `mapstructure:"logs"`
}

// GetTracesConfig merges the common configuration section with the traces specific section.
func (c Config) GetTracesConfig() EndpointConfig {
	return mergeConfig(c.CommonConfig, c.TracesConfig)
}

// GetMetricsConfig merges the common configuration section with the metrics specific section.
func (c Config) GetMetricsConfig() EndpointConfig {
	return mergeConfig(c.CommonConfig, c.MetricsConfig)
}

// GetLogsConfig merges the common configuration section with the logs specific section.
func (c Config) GetLogsConfig() EndpointConfig {
	return mergeConfig(c.CommonConfig, c.LogsConfig)
}

func mergeConfig(baseConfig EndpointConfig, config EndpointConfig) EndpointConfig {
	if config.APIKey == "" {
		config.APIKey = baseConfig.APIKey
	}

	if config.APIKeyHeader == "" {
		config.APIKeyHeader = baseConfig.APIKeyHeader
	}

	if config.HostOverride == "" {
		config.HostOverride = baseConfig.HostOverride
	}

	if config.Timeout == 0 {
		config.Timeout = baseConfig.Timeout
	}
	return config
}
