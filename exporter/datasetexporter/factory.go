// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package datasetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter/internal/metadata"
)

// NewFactory created new factory with DataSet exporters.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		BufferSettings:     newDefaultBufferSettings(),
		TracesSettings:     newDefaultTracesSettings(),
		LogsSettings:       newDefaultLogsSettings(),
		ServerHostSettings: newDefaultServerHostSettings(),
		BackOffConfig:      configretry.NewDefaultBackOffConfig(),
		QueueSettings:      exporterhelper.NewDefaultQueueConfig(),
		TimeoutSettings:    exporterhelper.NewDefaultTimeoutConfig(),
		Debug:              debugDefault,
	}
}

// castConfig casts it to the Dataset Config struct.
func castConfig(c component.Config) *Config {
	return c.(*Config)
}
