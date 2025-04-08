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
	TimeoutSettings exporterhelper.TimeoutConfig    `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	QueueSettings   exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	BackoffConfig   configretry.BackOffConfig       `mapstructure:"retry_on_failure"`

	confighttp.ClientConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	GeneratorURL            string                   `mapstructure:"generator_url"`
	DefaultSeverity         string                   `mapstructure:"severity"`
	SeverityAttribute       string                   `mapstructure:"severity_attribute"`
	APIVersion              string                   `mapstructure:"api_version"`
	EventLabels             []string                 `mapstructure:"event_labels"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return errors.New("endpoint must be non-empty")
	}
	if cfg.DefaultSeverity == "" {
		return errors.New("severity must be non-empty")
	}
	return nil
}
