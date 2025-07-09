// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/otlpjsonconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
	set.Logger.Info("Building otlpjson connector for traces")
	cfg := config.(*Config)

	return &connectorTraces{
		config:         *cfg,
		logger:         set.Logger,
		tracesConsumer: tracesConsumer,
	}
}

// Capabilities implements the consumer interface.
func (c *connectorTraces) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs method is called for each instance of a log sent to the connector
func (c *connectorTraces) ConsumeLogs(ctx context.Context, pl plog.Logs) error {
	// loop through the levels of logs
	tracesUnmarshaler := &ptrace.JSONUnmarshaler{}
	for i := 0; i < pl.ResourceLogs().Len(); i++ {
		li := pl.ResourceLogs().At(i)
		for j := 0; j < li.ScopeLogs().Len(); j++ {
			logRecord := li.ScopeLogs().At(j)
			for k := 0; k < logRecord.LogRecords().Len(); k++ {
				lRecord := logRecord.LogRecords().At(k)
				token := lRecord.Body()

				value := token.AsString()
				switch {
				case traceRegex.MatchString(value):
					var t ptrace.Traces
					t, err := tracesUnmarshaler.UnmarshalTraces([]byte(value))
					if err != nil {
						c.logger.Error("could not extract traces from otlp json", zap.Error(err))
						continue
					}
					err = c.tracesConsumer.ConsumeTraces(ctx, t)
					if err != nil {
						c.logger.Error("could not consume traces from otlp json", zap.Error(err))
					}
				case metricRegex.MatchString(value), logRegex.MatchString(value):
					// If it's a metric or log payload, continue to the next iteration
					continue
				default:
					// If no regex matches, log the invalid payload
					c.logger.Error("Invalid otlp payload")
				}
			}
		}
	}
	return nil
}
