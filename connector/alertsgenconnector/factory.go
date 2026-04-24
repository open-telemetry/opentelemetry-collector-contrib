// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/internal/metadata"
)

func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToLogs(createTracesToLogs, metadata.TracesToLogsStability),
		connector.WithMetricsToLogs(createMetricsToLogs, metadata.MetricsToLogsStability),
		connector.WithMetricsToMetrics(createMetricsToMetrics, metadata.MetricsToMetricsStability),
	)
}

// createTracesToLogs constructs the connector for a traces->logs pipeline.
func createTracesToLogs(
	_ context.Context,
	_ connector.Settings,
	_ component.Config,
	_ consumer.Logs,
) (connector.Traces, error) {
	return stub{}, nil
}

// createMetricsToLogs constructs the connector for a metrics->logs pipeline.
func createMetricsToLogs(
	_ context.Context,
	_ connector.Settings,
	_ component.Config,
	_ consumer.Logs,
) (connector.Metrics, error) {
	return stub{}, nil
}

// If you already had metrics->metrics support, keep it as-is. Leaving here so
// existing code paths keep working.
func createMetricsToMetrics(
	_ context.Context,
	_ connector.Settings,
	_ component.Config,
	_ consumer.Metrics,
) (connector.Metrics, error) {
	return stub{}, nil
}

type stub struct{}

func (stub) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (stub) Shutdown(_ context.Context) error {
	return nil
}

func (stub) Capabilities() consumer.Capabilities {
	panic("implement me")
}

func (stub) ConsumeMetrics(_ context.Context, _ pmetric.Metrics) error {
	panic("implement me")
}

func (stub) ConsumeTraces(_ context.Context, _ ptrace.Traces) error {
	panic("implement me")
}
