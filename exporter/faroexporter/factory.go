// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter/internal/metadata"
)

const (
	defaultTimeout = time.Second * 5
)

// NewFactory creates a factory for Faro exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		component.Type(metadata.Type),
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	defaultClientHTTPSettings := confighttp.NewDefaultClientConfig()
	defaultClientHTTPSettings.Timeout = defaultTimeout
	defaultClientHTTPSettings.WriteBufferSize = 512 * 1024
	return &Config{
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),
		ClientConfig:  defaultClientHTTPSettings,
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	fe, err := createFaroExporter(cfg.(*Config), set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		fe.pushTraces,
		exporterhelper.WithStart(fe.start),
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithQueue(cfg.(*Config).QueueSettings),
		exporterhelper.WithRetry(cfg.(*Config).BackOffConfig),
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	fe, err := createFaroExporter(cfg.(*Config), set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		fe.pushLogs,
		exporterhelper.WithStart(fe.start),
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithQueue(cfg.(*Config).QueueSettings),
		exporterhelper.WithRetry(cfg.(*Config).BackOffConfig),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	fe, err := createFaroExporter(cfg.(*Config), set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		fe.pushMetrics,
		exporterhelper.WithStart(fe.start),
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithQueue(cfg.(*Config).QueueSettings),
		exporterhelper.WithRetry(cfg.(*Config).BackOffConfig),
	)
}
