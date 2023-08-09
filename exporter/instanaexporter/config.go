// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package instanaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter"

import (
	"errors"
	"net/url"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
)

// Config defines configuration for the Instana exporter
type Config struct {
	Endpoint string `mapstructure:"endpoint"`

	AgentKey configopaque.String `mapstructure:"agent_key"`

	confighttp.HTTPClientSettings `mapstructure:",squash"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {

	if cfg.Endpoint == "" {
		return errors.New("no Instana endpoint set")
	}

	if cfg.AgentKey == "" {
		return errors.New("no Instana agent key set")
	}

	if !strings.HasPrefix(cfg.Endpoint, "https://") {
		return errors.New("endpoint must start with https://")
	}
	_, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return errors.New("endpoint must be a valid URL")
	}

	return nil
}
