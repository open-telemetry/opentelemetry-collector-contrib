// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mezmoexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter/internal/metadata"
)

// NewFactory creates a factory for Mezmo exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

// Create a default Memzo config
func createDefaultConfig() component.Config {
	return &Config{
		ClientConfig:  createDefaultClientConfig(),
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),
		IngestURL:     defaultIngestURL,
	}
}

// Create a log exporter for exporting to Mezmo
func createLogsExporter(ctx context.Context, settings exporter.Settings, exporterConfig component.Config) (exporter.Logs, error) {
	log := settings.Logger

	if exporterConfig == nil {
		return nil, errors.New("nil config")
	}
	expCfg := exporterConfig.(*Config)

	exp := newLogsExporter(expCfg, settings.TelemetrySettings, settings.BuildInfo, log)

	return exporterhelper.NewLogs(
		ctx,
		settings,
		expCfg,
		exp.pushLogData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(expCfg.BackOffConfig),
		exporterhelper.WithQueue(expCfg.QueueSettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.stop),
	)
}
