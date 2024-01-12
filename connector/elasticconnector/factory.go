package elasticconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

const (
	typeStr = "elastic"
)

func createDefaultConfig() component.Config {
	return &Config{}
}

// NewFactory creates a factory for the elasticconnector component.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		typeStr,
		createDefaultConfig,
		connector.WithTracesToTraces(createTracesToTraces, component.StabilityLevelAlpha),
		connector.WithMetricsToMetrics(createMetricsToMetrics, component.StabilityLevelAlpha),
		connector.WithLogsToLogs(createLogsToLogs, component.StabilityLevelAlpha),
	)
}

func createTracesToTraces(
	_ context.Context,
	params connector.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (connector.Traces, error) {
	c, err := newConnector(params.Logger, cfg)
	if err != nil {
		return nil, err
	}
	c.tracesConsumer = nextConsumer
	return c, nil
}

func createMetricsToMetrics(
	_ context.Context,
	params connector.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (connector.Metrics, error) {
	c, err := newConnector(params.Logger, cfg)
	if err != nil {
		return nil, err
	}
	c.metricsConsumer = nextConsumer
	return c, nil
}

func createLogsToLogs(
	_ context.Context,
	params connector.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (connector.Logs, error) {
	c, err := newConnector(params.Logger, cfg)
	if err != nil {
		return nil, err
	}
	c.logsConsumer = nextConsumer
	return c, nil
}
