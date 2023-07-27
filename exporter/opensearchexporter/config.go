// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for OpenSearch exporter.
type Config struct {
	confighttp.HTTPClientSettings  `mapstructure:"http"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	Namespace                      string `mapstructure:"namespace"`
	Dataset                        string `mapstructure:"dataset"`
}

var (
	errConfigNoEndpoint = errors.New("endpoint must be specified")
)

// Validate validates the opensearch server configuration.
func (cfg *Config) Validate() error {
	if len(cfg.Endpoint) == 0 {
		return errConfigNoEndpoint
	}
	return nil
}

// withDefaultConfig create a new default configuration
// and applies provided functions to it.
func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := newDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}
