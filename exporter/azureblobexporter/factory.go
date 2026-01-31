// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pipeline"

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
		Auth: Authentication{
			Type: ConnectionString,
		},
		Container: TelemetryConfig{
			Metrics: "metrics",
			Logs:    "logs",
			Traces:  "traces",
		},
		BlobNameFormat: BlobNameFormat{
			MetricsFormat:     "2006/01/02/metrics_15_04_05.json",
			LogsFormat:        "2006/01/02/logs_15_04_05.json",
			TracesFormat:      "2006/01/02/traces_15_04_05.json",
			SerialNumEnabled:  true,
			SerialNumRange:    10000,
			Params:            map[string]string{},
			TemplateEnabled:   false,
			TimeParserEnabled: true,
			TimeParserRanges:  nil,
		},
		FormatType: formatTypeJSON,
		AppendBlob: AppendBlob{
			Enabled:   false,
			Separator: "\n",
		},
		Encodings:     Encodings{},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}
}

func createLogsExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config,
) (exporter.Logs, error) {
	cfg := config.(*Config)
	azBlobExporter := newAzureBlobExporter(cfg, params.Logger, pipeline.SignalLogs)

	return exporterhelper.NewLogs(ctx, params,
		config,
		azBlobExporter.ConsumeLogs,
		exporterhelper.WithStart(azBlobExporter.start),
		exporterhelper.WithRetry(cfg.BackOffConfig))
}

func createMetricsExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config,
) (exporter.Metrics, error) {
	cfg := config.(*Config)
	azBlobExporter := newAzureBlobExporter(cfg, params.Logger, pipeline.SignalMetrics)

	return exporterhelper.NewMetrics(ctx, params,
		config,
		azBlobExporter.ConsumeMetrics,
		exporterhelper.WithStart(azBlobExporter.start),
		exporterhelper.WithRetry(cfg.BackOffConfig))
}

func createTracesExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config,
) (exporter.Traces, error) {
	cfg := config.(*Config)
	azBlobExporter := newAzureBlobExporter(cfg, params.Logger, pipeline.SignalTraces)

	return exporterhelper.NewTraces(ctx,
		params,
		config,
		azBlobExporter.ConsumeTraces,
		exporterhelper.WithStart(azBlobExporter.start),
		exporterhelper.WithRetry(cfg.BackOffConfig))
}
