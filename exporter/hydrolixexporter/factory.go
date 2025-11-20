// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hydrolixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hydrolixexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hydrolixexporter/internal/metadata"
)

// NewFactory creates a factory for Hydrolix exporter.
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
		ClientConfig: confighttp.ClientConfig{
			Timeout: 30 * time.Second,
		},
	}
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
		pushTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
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
		pushMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
	)
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
		pushLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
	)
}

// pushTraces is a placeholder that will be implemented in a follow-up PR
func pushTraces(_ context.Context, _ ptrace.Traces) error {
	// TODO: implement in follow-up PR
	return nil
}

// pushMetrics is a placeholder that will be implemented in a follow-up PR
func pushMetrics(_ context.Context, _ pmetric.Metrics) error {
	// TODO: implement in follow-up PR
	return nil
}

// pushLogs is a placeholder that will be implemented in a follow-up PR
func pushLogs(_ context.Context, _ plog.Logs) error {
	// TODO: implement in follow-up PR
	return nil
}
