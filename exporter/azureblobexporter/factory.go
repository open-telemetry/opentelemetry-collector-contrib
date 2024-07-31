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

const (
	// the format of encoded telemetry data
	formatTypeJSON  = "json"
	formatTypeProto = "proto"
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
	return &Config{
		Auth: &Authentication{
			Type: ConnectionString,
		},
		Container: &Container{
			Metrics: "metrics",
			Logs:    "logs",
			Traces:  "traces",
		},
		BlobNameFormat: &BlobNameFormat{
			FormatType: "{{.Year}}/{{.Month}}/{{.Day}}/{{.BlobName}}_{{.Hour}}_{{.Minute}}_{{.Second}}_{{.SerialNum}}.{{.FileExtension}}",
			BlobName: &BlobName{
				Metrics: "metrics",
				Logs:    "logs",
				Traces:  "traces",
			},
			Year:   "2006",
			Month:  "01",
			Day:    "02",
			Hour:   "15",
			Minute: "04",
			Second: "05",
		},
		FormatType: "json",
	}
}

func createLogsExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config) (exporter.Logs, error) {

	azBlobExporter := newAzureBlobExporter(config.(*Config), params.Logger)

	return exporterhelper.NewLogsExporter(ctx, params,
		config,
		azBlobExporter.ConsumeLogs,
		exporterhelper.WithStart(azBlobExporter.start))
}

func createMetricsExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config) (exporter.Metrics, error) {

	azBlobExporter := newAzureBlobExporter(config.(*Config), params.Logger)

	return exporterhelper.NewMetricsExporter(ctx, params,
		config,
		azBlobExporter.ConsumeMetrics,
		exporterhelper.WithStart(azBlobExporter.start))
}

func createTracesExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config) (exporter.Traces, error) {

	azBlobExporter := newAzureBlobExporter(config.(*Config), params.Logger)

	return exporterhelper.NewTracesExporter(ctx,
		params,
		config,
		azBlobExporter.ConsumeTraces,
		exporterhelper.WithStart(azBlobExporter.start))
}
