// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "logicmonitor"
	// The stability level of the exporter.
	stability = component.StabilityLevelAlpha
)

// NewFactory creates a LogicMonitor exporter factory
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, stability),
		exporter.WithTraces(createTracesExporter, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		RetrySettings: exporterhelper.NewDefaultRetrySettings(),
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
		exporterhelper.WithRetry(c.RetrySettings),
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
		exporterhelper.WithRetry(c.RetrySettings),
		exporterhelper.WithQueue(c.QueueSettings))
}
