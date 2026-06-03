// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package roundrobinconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/roundrobinconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/xconnector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/roundrobinconnector/internal/metadata"
)

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return xconnector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xconnector.WithTracesToTraces(createTracesToTraces, metadata.TracesToTracesStability),
		xconnector.WithMetricsToMetrics(createMetricsToMetrics, metadata.MetricsToMetricsStability),
		xconnector.WithLogsToLogs(createLogsToLogs, metadata.LogsToLogsStability),
		xconnector.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

// createDefaultConfig creates the default configuration.
func createDefaultConfig() component.Config {
	return &Config{}
}

// createLogsToLogs creates a log receiver based on provided config.
func createLogsToLogs(
	_ context.Context,
	_ connector.Settings,
	_ component.Config,
	nextConsumer consumer.Logs,
) (connector.Logs, error) {
	return newLogs(nextConsumer)
}

// createMetricsToMetrics creates a metrics receiver based on provided config.
func createMetricsToMetrics(
	_ context.Context,
	_ connector.Settings,
	_ component.Config,
	nextConsumer consumer.Metrics,
) (connector.Metrics, error) {
	return newMetrics(nextConsumer)
}

// createTracesToTraces creates a trace receiver based on provided config.
func createTracesToTraces(
	_ context.Context,
	_ connector.Settings,
	_ component.Config,
	nextConsumer consumer.Traces,
) (connector.Traces, error) {
	return newTraces(nextConsumer)
}
