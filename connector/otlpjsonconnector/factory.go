// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/otlpjsonconnector"

import (
	"context"
	"regexp"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/otlpjsonconnector/internal/metadata"
)

var logRegex = regexp.MustCompile(`^\{\s*"resourceLogs"\s*:\s*\[`)
var metricRegex = regexp.MustCompile(`^\{\s*"resourceMetrics"\s*:\s*\[`)
var traceRegex = regexp.MustCompile(`^\{\s*"resourceSpans"\s*:\s*\[`)

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithLogsToTraces(createTracesConnector, component.StabilityLevelAlpha),
		connector.WithLogsToMetrics(createMetricsConnector, component.StabilityLevelAlpha),
		connector.WithLogsToLogs(createLogsConnector, component.StabilityLevelAlpha),
	)
}

// createDefaultConfig creates the default configuration.
func createDefaultConfig() component.Config {
	return &Config{}
}

// createLogsConnector returns a connector which consume logs and export logs
func createLogsConnector(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (connector.Logs, error) {
	return newLogsConnector(set, cfg, nextConsumer), nil
}

// createTracesConnector returns a connector which consume logs and export traces
func createTracesConnector(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (connector.Logs, error) {
	return newTracesConnector(set, cfg, nextConsumer), nil
}

// createMetricsConnector returns a connector which consume logs and export metrics
func createMetricsConnector(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Logs, error) {
	return newMetricsConnector(set, cfg, nextConsumer), nil
}
