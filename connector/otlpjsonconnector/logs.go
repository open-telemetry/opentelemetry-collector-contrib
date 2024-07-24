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

type connectorLogs struct {
	config       Config
	logsConsumer consumer.Logs
	logger       *zap.Logger

	component.StartFunc
	component.ShutdownFunc
}

// newLogsConnector is a function to create a new connector for logs extraction
func newLogsConnector(set connector.Settings, config component.Config, logsConsumer consumer.Logs) *connectorLogs {
	set.TelemetrySettings.Logger.Info("Building otlpjson connector for logs")
	cfg := config.(*Config)

	return &connectorLogs{
		config:       *cfg,
		logger:       set.TelemetrySettings.Logger,
		logsConsumer: logsConsumer,
	}
}

// Capabilities implements the consumer interface
func (c *connectorLogs) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs method is called for each instance of a log sent to the connector
func (c *connectorLogs) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	return nil
}
