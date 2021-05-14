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

	"go.opentelemetry.io/collector/config"
)

type TracesConfig struct {
	// Wavefront Proxy URL of the form protocol://host[:port]
	Endpoint string `mapstructure:"endpoint"`

	// A default value to use if a span doesn't contain an "application" label
	DefaultApplication string `mapstructure:"default_application"`

	// A default value to use if a span doesn't contain a "service" or "service.name" label
	DefaultService string `mapstructure:"default_service"`
}

// Config defines configuration options for the exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// Traces defines the Traces exporter specific configuration
	Traces TracesConfig `mapstructure:"traces"`
}

func (c *Config) Validate() error {
	if c.Traces.Endpoint == "" {
		return fmt.Errorf("A non-empty traces.endpoint is required")
	}
	if c.Traces.DefaultApplication == "" {
		return fmt.Errorf("A non-empty traces.default_application is required")
	}
	if c.Traces.DefaultService == "" {
		return fmt.Errorf("A non-empty traces.default_service is required")
	}
	fmt.Printf("\n\n%#v\n\n", c)
	return nil
}
