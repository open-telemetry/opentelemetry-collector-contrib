// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const defaultErrorModeIgnoreGateID = "connector.routing.defaultErrorModeIgnore"

var defaultErrorModeIgnoreFeatureGate = featuregate.GlobalRegistry().MustRegister(
	defaultErrorModeIgnoreGateID,
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("Changes the default error_mode of the routing connector from propagate to ignore"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/48418"),
	featuregate.WithRegisterFromVersion("v0.152.0"),
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
	defaultErrorMode := ottl.PropagateError
	if defaultErrorModeIgnoreFeatureGate.IsEnabled() {
		defaultErrorMode = ottl.IgnoreError
	}
	return &Config{
		ErrorMode: defaultErrorMode,
	}
}

// createTracesToTraces creates a traces to traces connector based on provided config.
func createTracesToTraces(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	traces consumer.Traces,
) (connector.Traces, error) {
	return newTracesConnector(set, cfg, traces)
}

// createMetricsToMetrics creates a metrics to metrics connector based on provided config.
func createMetricsToMetrics(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	metrics consumer.Metrics,
) (connector.Metrics, error) {
	return newMetricsConnector(set, cfg, metrics)
}

// createLogsToLogs creates a logs to logs connector based on provided config.
func createLogsToLogs(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	logs consumer.Logs,
) (connector.Logs, error) {
	return newLogsConnector(set, cfg, logs)
}
