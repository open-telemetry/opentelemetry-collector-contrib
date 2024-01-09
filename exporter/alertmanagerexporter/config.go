// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertmanagerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alertmanagerexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for alertmanager exporter.
type Config struct {
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	BackoffConfig                  configretry.BackOffConfig `mapstructure:"retry_on_failure"`

	confighttp.HTTPClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	GeneratorURL                  string                   `mapstructure:"generator_url"`
	DefaultSeverity               string                   `mapstructure:"severity"`
	SeverityAttribute             string                   `mapstructure:"severity_attribute"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {

	if cfg.HTTPClientSettings.Endpoint == "" {
		return errors.New("endpoint must be non-empty")
	}
	if cfg.DefaultSeverity == "" {
		return errors.New("severity must be non-empty")
	}
	return nil
}
