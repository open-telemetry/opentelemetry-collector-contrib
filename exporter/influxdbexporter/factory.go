// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package influxdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"

import (
	"context"
	"time"

	"github.com/influxdata/influxdb-observability/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter/internal/metadata"
)

// NewFactory creates a factory for Jaeger Thrift over HTTP exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTraceExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createTraceExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Traces, error) {
	cfg := config.(*Config)

	exporter, err := newTracesExporter(cfg, set)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		exporter.pushTraces,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithStart(exporter.Start),
		exporterhelper.WithShutdown(exporter.Shutdown),
	)
}

func createMetricsExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Metrics, error) {
	cfg := config.(*Config)

	exporter, err := newMetricsExporter(cfg, set)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		exporter.pushMetrics,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithStart(exporter.Start),
	)
}

func createLogsExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Logs, error) {
	cfg := config.(*Config)

	exporter, err := newLogsExporter(cfg, set)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		exporter.pushLogs,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithStart(exporter.Start),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout: 5 * time.Second,
			Headers: map[string]configopaque.String{
				"User-Agent": "OpenTelemetry -> Influx",
			},
		},
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),
		RetrySettings: exporterhelper.NewDefaultRetrySettings(),
		MetricsSchema: common.MetricsSchemaTelegrafPrometheusV1.String(),
	}
}
