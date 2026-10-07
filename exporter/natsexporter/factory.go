// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
)

const (
	defaultLogsSubject    = `"otel_logs"`
	defaultMetricsSubject = `"otel_metrics"`
	defaultTracesSubject  = `"otel_traces"`
)

// NewFactory creates a factory for the NATS exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Endpoint: nats.DefaultURL,
		Pedantic: false,
		TLS:      configtls.NewDefaultClientConfig(),
		Logs:     SignalConfig{Subject: defaultLogsSubject},
		Metrics:  SignalConfig{Subject: defaultMetricsSubject},
		Traces:   SignalConfig{Subject: defaultTracesSubject},
	}
}

// exporters caches one natsExporter per configuration so the logs, metrics, and
// traces pipelines share a single instance (and a single NATS connection)
// instead of creating one per signal.
var exporters = sharedcomponent.NewSharedComponents()

func getOrCreateExporter(set exporter.Settings, cfg component.Config) *sharedcomponent.SharedComponent {
	return exporters.GetOrAdd(cfg, func() component.Component {
		return newExporter(set, cfg.(*Config))
	})
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	sc := getOrCreateExporter(set, cfg)
	exp := sc.Unwrap().(*natsExporter)
	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		exp.pushLogs,
		exporterhelper.WithStart(sc.Start),
		exporterhelper.WithShutdown(sc.Shutdown),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	sc := getOrCreateExporter(set, cfg)
	exp := sc.Unwrap().(*natsExporter)
	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		exp.pushMetrics,
		exporterhelper.WithStart(sc.Start),
		exporterhelper.WithShutdown(sc.Shutdown),
	)
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	sc := getOrCreateExporter(set, cfg)
	exp := sc.Unwrap().(*natsExporter)
	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		exp.pushTraces,
		exporterhelper.WithStart(sc.Start),
		exporterhelper.WithShutdown(sc.Shutdown),
	)
}
