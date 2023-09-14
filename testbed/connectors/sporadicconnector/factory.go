package spoardicconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

func NewFactory() connector.Factory {
	return connector.NewFactory(
		"sporadic",
		createDefaultConfig,
		connector.WithTracesToTraces(createTracesToTraces, component.StabilityLevelDevelopment),
		connector.WithMetricsToMetrics(createMetricsToMetrics, component.StabilityLevelDevelopment),
		connector.WithLogsToLogs(createLogsToLogs, component.StabilityLevelDevelopment),
	)
}
func createTracesToTraces(_ context.Context, params connector.CreateSettings, cfg component.Config, nextConsumer consumer.Traces) (connector.Traces, error) {
	tc := newTracesConnector(params.Logger, cfg)
	tc.traceConsumer = nextConsumer
	return tc, nil
}

func createMetricsToMetrics(_ context.Context, params connector.CreateSettings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Metrics, error) {
	mc := newMetricsConnector(params.Logger, cfg)
	mc.metricsConsumer = nextConsumer
	return mc, nil
}

func createLogsToLogs(_ context.Context, params connector.CreateSettings, cfg component.Config, nextConsumer consumer.Logs) (connector.Logs, error) {
	lc := newLogsConnector(params.Logger, cfg)
	lc.logsConsumer = nextConsumer
	return lc, nil
}
