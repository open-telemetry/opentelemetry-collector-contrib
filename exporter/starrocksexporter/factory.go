// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package starrocksexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal/metadata"
)

// NewFactory creates a factory for the StarRocks exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricExporter, metadata.MetricsStability),
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	c := cfg.(*Config)
	c.collectorVersion = set.BuildInfo.Version

	if c.IsDebugProtocol() {
		// Debug mode - logs data instead of sending
		exp := newDebugLogsExporter(set.Logger, c)
		return exporterhelper.NewLogs(
			ctx,
			set,
			cfg,
			exp.pushLogsData,
			exporterhelper.WithStart(exp.start),
			exporterhelper.WithShutdown(exp.shutdown),
			exporterhelper.WithTimeout(c.TimeoutSettings),
			exporterhelper.WithQueue(c.QueueSettings),
			exporterhelper.WithRetry(c.BackOffConfig),
		)
	}

	if c.IsHTTPProtocol() {
		// HTTP-based exporter using Stream Load API
		exp := newHTTPLogsExporter(set.Logger, set.TelemetrySettings, c)
		return exporterhelper.NewLogs(
			ctx,
			set,
			cfg,
			exp.pushLogsData,
			exporterhelper.WithStart(exp.start),
			exporterhelper.WithShutdown(exp.shutdown),
			exporterhelper.WithTimeout(c.TimeoutSettings),
			exporterhelper.WithQueue(c.QueueSettings),
			exporterhelper.WithRetry(c.BackOffConfig),
		)
	}

	// Default MySQL protocol exporter
	exp := newLogsExporter(set.Logger, c)
	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		exp.pushLogsData,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithTimeout(c.TimeoutSettings),
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	c := cfg.(*Config)
	c.collectorVersion = set.BuildInfo.Version

	if c.IsDebugProtocol() {
		// Debug mode - logs data instead of sending
		exp := newDebugTracesExporter(set.Logger, c)
		return exporterhelper.NewTraces(
			ctx,
			set,
			cfg,
			exp.pushTraceData,
			exporterhelper.WithStart(exp.start),
			exporterhelper.WithShutdown(exp.shutdown),
			exporterhelper.WithTimeout(c.TimeoutSettings),
			exporterhelper.WithQueue(c.QueueSettings),
			exporterhelper.WithRetry(c.BackOffConfig),
		)
	}

	if c.IsHTTPProtocol() {
		// HTTP-based exporter using Stream Load API
		exp := newHTTPTracesExporter(set.Logger, set.TelemetrySettings, c)
		return exporterhelper.NewTraces(
			ctx,
			set,
			cfg,
			exp.pushTraceData,
			exporterhelper.WithStart(exp.start),
			exporterhelper.WithShutdown(exp.shutdown),
			exporterhelper.WithTimeout(c.TimeoutSettings),
			exporterhelper.WithQueue(c.QueueSettings),
			exporterhelper.WithRetry(c.BackOffConfig),
		)
	}

	// Default MySQL protocol exporter
	exp := newTracesExporter(set.Logger, c)
	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		exp.pushTraceData,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithTimeout(c.TimeoutSettings),
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}

func createMetricExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	c := cfg.(*Config)
	c.collectorVersion = set.BuildInfo.Version

	if c.IsDebugProtocol() {
		// Debug mode - logs data instead of sending
		exp := newDebugMetricsExporter(set.Logger, c)
		return exporterhelper.NewMetrics(
			ctx,
			set,
			cfg,
			exp.pushMetricsData,
			exporterhelper.WithStart(exp.start),
			exporterhelper.WithShutdown(exp.shutdown),
			exporterhelper.WithTimeout(c.TimeoutSettings),
			exporterhelper.WithQueue(c.QueueSettings),
			exporterhelper.WithRetry(c.BackOffConfig),
		)
	}

	if c.IsHTTPProtocol() {
		// HTTP-based exporter using Stream Load API
		exp := newHTTPMetricsExporter(set.Logger, set.TelemetrySettings, c)
		return exporterhelper.NewMetrics(
			ctx,
			set,
			cfg,
			exp.pushMetricsData,
			exporterhelper.WithStart(exp.start),
			exporterhelper.WithShutdown(exp.shutdown),
			exporterhelper.WithTimeout(c.TimeoutSettings),
			exporterhelper.WithQueue(c.QueueSettings),
			exporterhelper.WithRetry(c.BackOffConfig),
		)
	}

	// Default MySQL protocol exporter
	exp := newMetricsExporter(set.Logger, c)
	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		exp.pushMetricsData,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithTimeout(c.TimeoutSettings),
		exporterhelper.WithQueue(c.QueueSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}
