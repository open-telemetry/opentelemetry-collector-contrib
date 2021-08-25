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

package tanzuobservabilityexporter

import (
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type TracesConfig struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
}

// Config defines configuration options for the exporter.
type Config struct {
	config.ExporterSettings      `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`

	// Traces defines the Traces exporter specific configuration
	Traces TracesConfig `mapstructure:"traces"`
}

func (c *Config) Validate() error {
	if c.Traces.Endpoint == "" {
		return fmt.Errorf("A non-empty traces.endpoint is required")
	}
	if _, err := url.Parse(c.Traces.Endpoint); err != nil {
		return fmt.Errorf("invalid traces.endpoint %s", err)
	}
	return nil
}
