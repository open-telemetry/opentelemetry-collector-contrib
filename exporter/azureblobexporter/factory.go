// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter/internal/metadata"
)

// NewFactory creates a factory for Azure Blob exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createLogsExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config) (exporter.Logs, error) {

	azBlobExporter := newAzureBlobExporter(config.(*Config), params)

	return exporterhelper.NewLogsExporter(ctx, params,
		config,
		azBlobExporter.ConsumeLogs,
		exporterhelper.WithStart(azBlobExporter.start))
}

func createMetricsExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config) (exporter.Metrics, error) {

	azBlobExporter := newAzureBlobExporter(config.(*Config), params)

	return exporterhelper.NewMetricsExporter(ctx, params,
		config,
		azBlobExporter.ConsumeMetrics,
		exporterhelper.WithStart(azBlobExporter.start))
}

func createTracesExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config) (exporter.Traces, error) {

	azBlobExporter := newAzureBlobExporter(config.(*Config), params)

	return exporterhelper.NewTracesExporter(ctx,
		params,
		config,
		azBlobExporter.ConsumeTraces,
		exporterhelper.WithStart(azBlobExporter.start))
}
