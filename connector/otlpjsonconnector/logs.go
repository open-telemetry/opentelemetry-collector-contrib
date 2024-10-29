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
func (c *connectorLogs) ConsumeLogs(ctx context.Context, pl plog.Logs) error {
	// loop through the levels of logs
	logsUnmarshaler := &plog.JSONUnmarshaler{}
	for i := 0; i < pl.ResourceLogs().Len(); i++ {
		li := pl.ResourceLogs().At(i)
		for j := 0; j < li.ScopeLogs().Len(); j++ {
			logRecord := li.ScopeLogs().At(j)
			for k := 0; k < logRecord.LogRecords().Len(); k++ {
				lRecord := logRecord.LogRecords().At(k)
				token := lRecord.Body()
				var l plog.Logs
				l, err := logsUnmarshaler.UnmarshalLogs([]byte(token.AsString()))
				if err != nil {
					c.logger.Error("could not extract logs from otlp json", zap.Error(err))
					continue
				}
				err = c.logsConsumer.ConsumeLogs(ctx, l)
				if err != nil {
					c.logger.Error("could not consume logs from otlp json", zap.Error(err))
				}
			}
		}
	}
	return nil
}
