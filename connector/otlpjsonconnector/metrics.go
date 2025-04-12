// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/otlpjsonconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
	set.Logger.Info("Building otlpjson connector for metrics")
	cfg := config.(*Config)

	return &connectorMetrics{
		config:          *cfg,
		logger:          set.Logger,
		metricsConsumer: metricsConsumer,
	}
}

// Capabilities implements the consumer interface.
func (c *connectorMetrics) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs method is called for each instance of a log sent to the connector
func (c *connectorMetrics) ConsumeLogs(ctx context.Context, pl plog.Logs) error {
	// loop through the levels of logs
	metricsUnmarshaler := &pmetric.JSONUnmarshaler{}
	for i := 0; i < pl.ResourceLogs().Len(); i++ {
		li := pl.ResourceLogs().At(i)
		for j := 0; j < li.ScopeLogs().Len(); j++ {
			logRecord := li.ScopeLogs().At(j)
			for k := 0; k < logRecord.LogRecords().Len(); k++ {
				lRecord := logRecord.LogRecords().At(k)
				token := lRecord.Body()

				value := token.AsString()
				switch {
				case metricRegex.MatchString(value):
					var m pmetric.Metrics
					m, err := metricsUnmarshaler.UnmarshalMetrics([]byte(value))
					if err != nil {
						c.logger.Error("could not extract metrics from otlp json", zap.Error(err))
						continue
					}
					err = c.metricsConsumer.ConsumeMetrics(ctx, m)
					if err != nil {
						c.logger.Error("could not consume metrics from otlp json", zap.Error(err))
					}
				case logRegex.MatchString(value), traceRegex.MatchString(value):
					// If it's a log or trace payload, simply continue
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
