// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the Honeycomb Marker exporter.
type Config struct {
	APIKey  string  `mapstructure:"api_key"`
	APIURL  string  `mapstructure:"api_url"`
	Presets presets `mapstructure:"presets"`
}

type presets struct {
	K8sEvents bool `mapstructure:"k8s_events"`
}

func (cfg *Config) Validate() error {
	if cfg.APIKey == "" {
		return fmt.Errorf("invalid API Key")
	}

	if cfg.APIURL == "" {
		return fmt.Errorf("invalid URL")
	}

	return nil
}

var _ component.Config = (*Config)(nil)
