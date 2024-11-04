// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter/internal/metadata"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	httpClientConfig := confighttp.NewDefaultClientConfig()
	httpClientConfig.Timeout = 60 * time.Second

	return &Config{
		ClientConfig:  httpClientConfig,
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		Table: Table{
			Logs:    "otel_logs",
			Traces:  "otel_traces",
			Metrics: "otel_metrics",
		},
		Database:          "otel",
		CreateSchema:      true,
		HistoryDays:       0,
		CreateHistoryDays: 0,
		ReplicationNum:    1,
		TimeZone:          time.Local.String(),
	}
}

func createLogsExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Logs, error) {
	c := cfg.(*Config)
	exporter := newLogsExporter(set.Logger, c, set.TelemetrySettings)
	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		exporter.pushLogData,
		exporterhelper.WithStart(exporter.start),
		exporterhelper.WithShutdown(exporter.shutdown),
		// we config the timeout option in http client, so we don't need to set timeout here
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}

func createTracesExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Traces, error) {
	c := cfg.(*Config)
	exporter := newTracesExporter(set.Logger, c, set.TelemetrySettings)
	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		exporter.pushTraceData,
		exporterhelper.WithStart(exporter.start),
		exporterhelper.WithShutdown(exporter.shutdown),
		// we config the timeout option in http client, so we don't need to set timeout here
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}

func createMetricsExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Metrics, error) {
	c := cfg.(*Config)
	exporter := newMetricsExporter(set.Logger, c, set.TelemetrySettings)
	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		exporter.pushMetricData,
		exporterhelper.WithStart(exporter.start),
		exporterhelper.WithShutdown(exporter.shutdown),
		// we config the timeout option in http client, so we don't need to set timeout here
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}
