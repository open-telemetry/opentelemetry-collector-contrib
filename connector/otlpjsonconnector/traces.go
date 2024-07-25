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

type connectorTraces struct {
	config         Config
	tracesConsumer consumer.Traces
	logger         *zap.Logger

	component.StartFunc
	component.ShutdownFunc
}

// newTracesConnector is a function to create a new connector for traces extraction
func newTracesConnector(set connector.Settings, config component.Config, tracesConsumer consumer.Traces) *connectorTraces {
	set.TelemetrySettings.Logger.Info("Building otlpjson connector for traces")
	cfg := config.(*Config)

	return &connectorTraces{
		config:         *cfg,
		logger:         set.TelemetrySettings.Logger,
		tracesConsumer: tracesConsumer,
	}
}

// Capabilities implements the consumer interface.
func (c *connectorTraces) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs method is called for each instance of a log sent to the connector
func (c *connectorTraces) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	return nil
}
