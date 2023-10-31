// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToTraces(createTracesToTraces, metadata.TracesToTracesStability),
		connector.WithMetricsToMetrics(createMetricsToMetrics, metadata.MetricsToMetricsStability),
		connector.WithLogsToLogs(createLogsToLogs, metadata.LogsToLogsStability),
	)
}

// createDefaultConfig creates the default configuration.
func createDefaultConfig() component.Config {
	return &Config{
		ErrorMode: ottl.PropagateError,
	}
}

// createTracesToTraces creates a traces to traces connector based on provided config.
func createTracesToTraces(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	traces consumer.Traces,
) (connector.Traces, error) {
	return newTracesConnector(set, cfg, traces)
}

// createMetricsToMetrics creates a metrics to metrics connector based on provided config.
func createMetricsToMetrics(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	metrics consumer.Metrics,
) (connector.Metrics, error) {
	return newMetricsConnector(set, cfg, metrics)
}

// createLogsToLogs creates a logs to logs connector based on provided config.
func createLogsToLogs(
	_ context.Context,
	set connector.CreateSettings,
	cfg component.Config,
	logs consumer.Logs,
) (connector.Logs, error) {
	return newLogsConnector(set, cfg, logs)
}
