// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecopsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlesecopsexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for the Google SecOps Exporter.
type Config struct {
	TimeoutConfig    exporterhelper.TimeoutConfig                             `mapstructure:",squash"`
	QueueBatchConfig configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
	BackOffConfig    configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`
	// The remaining config fields will be added in the next PR alongside the exporter implementation.
}

// Validate checks if the configuration is valid.
func (cfg *Config) Validate() error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	return nil
}
