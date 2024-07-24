// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/otlpjsonconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type connectorMetrics struct {
	config          Config
	metricsConsumer consumer.Metrics
	logger          *zap.Logger

	component.StartFunc
	component.ShutdownFunc
}

// newMetricsConnector is a function to create a new connector for metrics extraction
func newMetricsConnector(set connector.Settings, config component.Config, metricsConsumer consumer.Metrics) *connectorMetrics {
	set.TelemetrySettings.Logger.Info("Building otlpjson connector for metrics")
	cfg := config.(*Config)

	return &connectorMetrics{
		config:          *cfg,
		logger:          set.TelemetrySettings.Logger,
		metricsConsumer: metricsConsumer,
	}
}

// Capabilities implements the consumer interface.
func (c *connectorMetrics) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs method is called for each instance of a log sent to the connector
func (c *connectorMetrics) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	return nil
}
