// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter/internal/metadata"
)

// NewFactory creates a LogicMonitor exporter factory
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),
	}
}

func createLogsExporter(ctx context.Context, set exporter.CreateSettings, cfg component.Config) (exporter.Logs, error) {
	lmLogExp := newLogsExporter(ctx, cfg, set)
	c := cfg.(*Config)

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		lmLogExp.PushLogData,
		exporterhelper.WithStart(lmLogExp.start),
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}

func createTracesExporter(ctx context.Context, set exporter.CreateSettings, cfg component.Config) (exporter.Traces, error) {
	lmTraceExp := newTracesExporter(ctx, cfg, set)
	c := cfg.(*Config)

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		lmTraceExp.PushTraceData,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithStart(lmTraceExp.start),
		exporterhelper.WithRetry(c.BackOffConfig),
		exporterhelper.WithQueue(c.QueueSettings))
}
