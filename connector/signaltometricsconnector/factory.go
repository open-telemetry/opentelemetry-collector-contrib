// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signaltometricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/metadata"
)

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetrics, metadata.TracesToMetricsStability),
		connector.WithMetricsToMetrics(createMetricsToMetrics, metadata.MetricsToMetricsStability),
		connector.WithLogsToMetrics(createLogsToMetrics, metadata.LogsToMetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &config.Config{}
}

func createTracesToMetrics(
	_ context.Context,
	set connector.Settings,
	_ component.Config,
	nextConsumer consumer.Metrics,
) (connector.Traces, error) {
	return newSignalToMetrics(set, nextConsumer), nil
}

func createMetricsToMetrics(
	_ context.Context,
	set connector.Settings,
	_ component.Config,
	nextConsumer consumer.Metrics,
) (connector.Metrics, error) {
	return newSignalToMetrics(set, nextConsumer), nil
}

func createLogsToMetrics(
	_ context.Context,
	set connector.Settings,
	_ component.Config,
	nextConsumer consumer.Metrics,
) (connector.Logs, error) {
	return newSignalToMetrics(set, nextConsumer), nil
}
