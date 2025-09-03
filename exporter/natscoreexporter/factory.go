// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natscoreexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter"

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/marshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/metadata"
)

const (
	defaultLogsSubject      = "\"otel_logs\""
	defaultLogsMarshaler    = marshaler.OtlpProtoBuiltinMarshalerName
	defaultMetricsSubject   = "\"otel_metrics\""
	defaultMetricsMarshaler = marshaler.OtlpProtoBuiltinMarshalerName
	defaultTracesSubject    = "\"otel_spans\""
	defaultTracesMarshaler  = marshaler.OtlpProtoBuiltinMarshalerName
)

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
		TLS:      configtls.NewDefaultClientConfig(),
		Logs: LogsConfig{
			Subject:              defaultLogsSubject,
			BuiltinMarshalerName: defaultLogsMarshaler,
		},
		Metrics: MetricsConfig{
			Subject:              defaultMetricsSubject,
			BuiltinMarshalerName: defaultMetricsMarshaler,
		},
		Traces: TracesConfig{
			Subject:              defaultTracesSubject,
			BuiltinMarshalerName: defaultTracesMarshaler,
		},
		Auth: AuthConfig{},
	}
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		func(_ context.Context, _ plog.Logs) error {
			return nil
		},
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		func(_ context.Context, _ pmetric.Metrics) error {
			return nil
		},
	)
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		func(_ context.Context, _ ptrace.Traces) error {
			return nil
		},
	)
}
