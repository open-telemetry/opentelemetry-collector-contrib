// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter/internal/metadata"
)

const (
	defaultTracesSubject  = "otel.traces"
	defaultMetricsSubject = "otel.metrics"
	defaultLogsSubject    = "otel.logs"
	defaultEncoding       = "otlp_proto"

	defaultConnectionTimeout = 2 * time.Second
	defaultMaxReconnects     = 60
	defaultReconnectWait     = 2 * time.Second
)

// NewFactory creates a factory for the NATS exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
		BackOffConfig:   configretry.NewDefaultBackOffConfig(),
		QueueSettings:   configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		Connection: ConnectionConfig{
			Timeout:       defaultConnectionTimeout,
			MaxReconnects: defaultMaxReconnects,
			ReconnectWait: defaultReconnectWait,
		},
		Traces:  SignalConfig{Subject: defaultTracesSubject, Encoding: defaultEncoding},
		Metrics: SignalConfig{Subject: defaultMetricsSubject, Encoding: defaultEncoding},
		Logs:    SignalConfig{Subject: defaultLogsSubject, Encoding: defaultEncoding},
	}
}

func createTracesExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Traces, error) {
	c := cfg.(*Config)
	exp := newTracesExporter(*c, set)
	return exporterhelper.NewTraces(ctx, set, cfg, exp.export,
		exporterhelperOptions(*c, exp.start, exp.close)...)
}

func createMetricsExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Metrics, error) {
	c := cfg.(*Config)
	exp := newMetricsExporter(*c, set)
	return exporterhelper.NewMetrics(ctx, set, cfg, exp.export,
		exporterhelperOptions(*c, exp.start, exp.close)...)
}

func createLogsExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Logs, error) {
	c := cfg.(*Config)
	exp := newLogsExporter(*c, set)
	return exporterhelper.NewLogs(ctx, set, cfg, exp.export,
		exporterhelperOptions(*c, exp.start, exp.close)...)
}

// exporterhelperOptions builds the exporterhelper options shared by every
// signal: capabilities, timeout, retry, queue, and lifecycle hooks.
func exporterhelperOptions(cfg Config, start component.StartFunc, shutdown component.ShutdownFunc) []exporterhelper.Option {
	return []exporterhelper.Option{
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(start),
		exporterhelper.WithShutdown(shutdown),
	}
}
