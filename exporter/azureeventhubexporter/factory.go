// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/xexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubexporter/internal/metadata"
)

// NewFactory creates a factory for the Azure Event Hub exporter.
func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xexporter.WithLogs(createLogsExporter, metadata.LogsStability),
		xexporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		xexporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
		QueueSettings:   configoptional.Default(exporterhelper.NewDefaultQueueConfig()),
		BackOffConfig:   configretry.NewDefaultBackOffConfig(),
	}
}

func createLogsExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Logs, error) {
	exp := newExporter(cfg.(*Config), set.Logger)
	return exporterhelper.NewLogs(ctx, set, cfg,
		exp.ConsumeLogs,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithTimeout(exp.config.TimeoutSettings),
		exporterhelper.WithQueue(exp.config.QueueSettings),
		exporterhelper.WithRetry(exp.config.BackOffConfig),
	)
}

func createMetricsExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Metrics, error) {
	exp := newExporter(cfg.(*Config), set.Logger)
	return exporterhelper.NewMetrics(ctx, set, cfg,
		exp.ConsumeMetrics,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithTimeout(exp.config.TimeoutSettings),
		exporterhelper.WithQueue(exp.config.QueueSettings),
		exporterhelper.WithRetry(exp.config.BackOffConfig),
	)
}

func createTracesExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Traces, error) {
	exp := newExporter(cfg.(*Config), set.Logger)
	return exporterhelper.NewTraces(ctx, set, cfg,
		exp.ConsumeTraces,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithTimeout(exp.config.TimeoutSettings),
		exporterhelper.WithQueue(exp.config.QueueSettings),
		exporterhelper.WithRetry(exp.config.BackOffConfig),
	)
}
