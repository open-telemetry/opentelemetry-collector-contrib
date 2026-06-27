// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/xexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter/internal/metadata"
)

func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xexporter.WithLogs(createLogsExporter, metadata.LogsStability),
		xexporter.WithTraces(createTracesExporter, metadata.TracesStability),
		xexporter.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createLogsExporter(ctx context.Context, set exporter.Settings, config component.Config) (exporter.Logs, error) {
	cfg := config.(*Config)
	gcsExp, err := newGCSExporter(ctx, cfg, set.Logger, signalTypeLogs)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		gcsExp.ConsumeLogs,
		exporterOptions(cfg, gcsExp.Start, gcsExp.Shutdown)...,
	)
}

func createTracesExporter(ctx context.Context, set exporter.Settings, config component.Config) (exporter.Traces, error) {
	cfg := config.(*Config)
	gcsExp, err := newGCSExporter(ctx, cfg, set.Logger, signalTypeTraces)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		gcsExp.ConsumeTraces,
		exporterOptions(cfg, gcsExp.Start, gcsExp.Shutdown)...,
	)
}

func exporterOptions(cfg *Config, start component.StartFunc, shutdown component.ShutdownFunc) []exporterhelper.Option {
	return []exporterhelper.Option{
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.RetrySettings),
		exporterhelper.WithStart(start),
		exporterhelper.WithShutdown(shutdown),
	}
}
