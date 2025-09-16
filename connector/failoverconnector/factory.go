// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/metadata"
)

func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToTraces(createTracesToTraces, metadata.TracesToTracesStability),
		connector.WithMetricsToMetrics(createMetricsToMetrics, metadata.MetricsToMetricsStability),
		connector.WithLogsToLogs(createLogsToLogs, metadata.LogsToLogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),
		RetryInterval: 10 * time.Minute,
		RetryGap:      0,
		MaxRetries:    0,
	}
}

func createTracesToTraces(
	ctx context.Context,
	set connector.Settings,
	cfg component.Config,
	traces consumer.Traces,
) (connector.Traces, error) {
	t, err := newTracesToTraces(set, cfg, traces)
	if err != nil {
		return nil, err
	}
	expSettings := exporter.Settings{
		ID:                set.ID,
		TelemetrySettings: set.TelemetrySettings,
		BuildInfo:         set.BuildInfo,
	}

	oCfg := cfg.(*Config)

	// If queue is disabled, return the raw failover connector
	if !oCfg.QueueSettings.Enabled {
		return t, nil
	}

	// If queue is enabled, wrap with exporterhelper
	wrapped, err := exporterhelper.NewTraces(ctx, expSettings, cfg,
		t.ConsumeTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithQueue(oCfg.QueueSettings),
	)
	if err != nil {
		return nil, err
	}

	// Return testable wrapper that exposes internal failover router
	return newWrappedTracesConnector(wrapped, t.(*tracesFailover)), nil
}

func createMetricsToMetrics(
	ctx context.Context,
	set connector.Settings,
	cfg component.Config,
	metrics consumer.Metrics,
) (connector.Metrics, error) {
	t, err := newMetricsToMetrics(set, cfg, metrics)
	if err != nil {
		return nil, err
	}
	expSettings := exporter.Settings{
		ID:                set.ID,
		TelemetrySettings: set.TelemetrySettings,
		BuildInfo:         set.BuildInfo,
	}

	oCfg := cfg.(*Config)

	// If queue is disabled, return the raw failover connector directly (original behavior)
	if !oCfg.QueueSettings.Enabled {
		return t, nil
	}

	// If queue is enabled, wrap with exporterhelper
	wrapped, err := exporterhelper.NewMetrics(ctx, expSettings, cfg,
		t.ConsumeMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithQueue(oCfg.QueueSettings),
	)
	if err != nil {
		return nil, err
	}

	// Return testable wrapper that exposes internal failover router
	return newWrappedMetricsConnector(wrapped, t.(*metricsFailover)), nil
}

func createLogsToLogs(
	ctx context.Context,
	set connector.Settings,
	cfg component.Config,
	logs consumer.Logs,
) (connector.Logs, error) {
	t, err := newLogsToLogs(set, cfg, logs)
	if err != nil {
		return nil, err
	}
	expSettings := exporter.Settings{
		ID:                set.ID,
		TelemetrySettings: set.TelemetrySettings,
		BuildInfo:         set.BuildInfo,
	}

	oCfg := cfg.(*Config)

	// If queue is disabled, return the raw failover connector directly (original behavior)
	if !oCfg.QueueSettings.Enabled {
		return t, nil
	}

	// If queue is enabled, wrap with exporterhelper
	wrapped, err := exporterhelper.NewLogs(ctx, expSettings, cfg,
		t.ConsumeLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithQueue(oCfg.QueueSettings),
	)
	if err != nil {
		return nil, err
	}

	// Return testable wrapper that exposes internal failover router
	return newWrappedLogsConnector(wrapped, t.(*logsFailover)), nil
}
