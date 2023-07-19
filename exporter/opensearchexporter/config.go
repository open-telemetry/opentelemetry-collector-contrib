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
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`
	Namespace                     string `mapstructure:"namespace"`
	Dataset                       string `mapstructure:"dataset"`
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
