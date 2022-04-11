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

package tanzuobservabilityexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

type TracesConfig struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
}

type MetricsConfig struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	ResourceAttributes            resourcetotelemetry.Settings `mapstructure:"resource_attributes"`
}

// Config defines configuration options for the exporter.
type Config struct {
	config.ExporterSettings      `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`

	// Traces defines the Traces exporter specific configuration
	Traces  TracesConfig  `mapstructure:"traces"`
	Metrics MetricsConfig `mapstructure:"metrics"`
}

func (c *Config) Validate() error {
	tracesURL, err := parseEndpoint("traces", c.Traces.Endpoint)
	if err != nil {
		return err
	}
	metricsURL, err := parseEndpoint("metrics", c.Metrics.Endpoint)
	if err != nil {
		return err
	}
	if tracesURL.Hostname() != metricsURL.Hostname() {
		return errors.New("host for metrics and traces must be the same")
	}
	return nil
}

func parseEndpoint(name string, endpoint string) (*url.URL, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("A non-empty %s.endpoint is required", name)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid %s.endpoint %s", name, err)
	}
	return u, nil
}
