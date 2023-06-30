package exampleconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

const (
	defaultVal = 0
	// this is the name used to refer to the connector in the config.yaml
	typeStr = "exmaple"
)

// NewFactory creates a factory for tailtracer connector.
func NewFactory() connector.Factory {
	//  OTel connector factory to make a factory for connectors
	return connector.NewFactory(
		typeStr,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, component.StabilityLevelAlpha))
}

func createDefaultConfig() component.Config {
	return &Config{
		Count: defaultVal,
	}
}

// defines the consumer type of the connector
// we want to consume traces and export metrics therefore define nextConsumer as metrics, consumer is the next component in the pipeline
func createTracesToMetricsConnector(ctx context.Context, params connector.CreateSettings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	c, err := newConnector(params.Logger, cfg)
	if err != nil {
		return nil, err
	}
	c.metricsConsumer = nextConsumer
	return c, nil
}
