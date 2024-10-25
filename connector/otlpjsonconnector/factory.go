// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/otlpjsonconnector"

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/otlpjsonconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
)

type otlpjsonconnector struct {
	cfg    *Config
	set    connector.Settings
	logger *zap.Logger
	component.StartFunc
	component.ShutdownFunc
	logsConsumer      consumer.Logs
	traceConsumer     consumer.Traces
	logsConsumerbool  bool
	traceConsumerbool bool
}

func newOTLPJsonConnector(cfg *Config, set connector.Settings) component.Component {
	return &otlpjsonconnector{
		cfg: cfg,
		set: set,
	}
}

// Capabilities implements the consumer interface
func (o *otlpjsonconnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs method is called for each instance of a log sent to the connector
func (o *otlpjsonconnector) ConsumeLogs(ctx context.Context, pl plog.Logs) error {
	// loop through the levels of logs
	logsUnmarshaler := &plog.JSONUnmarshaler{}
	tracesUnmarshaler := &ptrace.JSONUnmarshaler{}

	for i := 0; i < pl.ResourceLogs().Len(); i++ {
		li := pl.ResourceLogs().At(i)

		for j := 0; j < li.ScopeLogs().Len(); j++ {
			logRecord := li.ScopeLogs().At(j)
			for k := 0; k < logRecord.LogRecords().Len(); k++ {
				lRecord := logRecord.LogRecords().At(k)
				token := lRecord.Body()

				fmt.Println("inside consume logs")
				//Check for matches and apply corresponding marshaller
				switch {
				case strings.Contains(token.AsString(), "resourceLogs") && o.logsConsumerbool:
					fmt.Println("this is inside logs")
					l, err := logsUnmarshaler.UnmarshalLogs([]byte(token.AsString()))
					if err != nil {
						o.logger.Error("could not extract logs from otlp json", zap.Error(err))
						continue
					}

					err = o.logsConsumer.ConsumeLogs(ctx, l)
					if err != nil {
						o.logger.Error("could not consume logs from otlp json", zap.Error(err))
					}
				case strings.Contains(token.AsString(), "resourceSpans") && o.traceConsumerbool:

					t, err := tracesUnmarshaler.UnmarshalTraces([]byte(token.AsString()))
					if err != nil {
						o.logger.Error("could not extract logs from otlp json", zap.Error(err))
						continue
					}
					err = o.traceConsumer.ConsumeTraces(ctx, t)
					if err != nil {
						o.logger.Error("could not consume logs from otlp json", zap.Error(err))
					}
				default:
					o.logger.Error("invalid otlp payload")
				}

			}
		}
	}
	return nil
}

func (o *otlpjsonconnector) registerLogsConnector(nextConsumer consumer.Logs) {
	o.logsConsumer = nextConsumer
	o.logsConsumerbool = true
}

func (o *otlpjsonconnector) registerTracesConnector(nextConsumer consumer.Traces) {
	o.traceConsumer = nextConsumer
	o.traceConsumerbool = true
}

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
	c context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (connector.Logs, error) {
	oCfg := cfg.(*Config)
	conn := otlpjsoncon.GetOrAdd(oCfg, func() component.Component {
		return newOTLPJsonConnector(oCfg, set)
	})
	conn.Unwrap().(*otlpjsonconnector).registerLogsConnector(nextConsumer)
	return conn.Unwrap().(*otlpjsonconnector), nil
}

// createTracesConnector returns a connector which consume logs and export traces
func createTracesConnector(
	_ context.Context,
	set connector.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (connector.Logs, error) {
	oCfg := cfg.(*Config)
	conn := otlpjsoncon.GetOrAdd(oCfg, func() component.Component {
		return newOTLPJsonConnector(oCfg, set)
	})
	conn.Unwrap().(*otlpjsonconnector).registerTracesConnector(nextConsumer)
	return conn.Unwrap().(*otlpjsonconnector), nil
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

var otlpjsoncon = sharedcomponent.NewSharedComponents()
