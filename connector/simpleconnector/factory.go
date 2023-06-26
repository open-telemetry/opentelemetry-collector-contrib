// Runs the metadat.yaml file to fully initalize the connector as a usable type
package simpleconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
)

const (
	Type                     = "simple"
	TracesToMetricsStability = component.StabilityLevelDevelopment
)

// NewFactory creates a factory for tailtracer receiver.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, TracesToMetricsStability),
	)
}

const (
	// our simple connector just counts the number of metrics a span has
	defCount = 0
)

func createDefaultConfig() component.Config {
	return &Config{
		count: defCount,
	}
}

func createTracesToMetricsConnector(ctx context.Context, params connector.CreateSettings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	c, err := newConnector(params.Logger, cfg)
	if err != nil {
		return nil, err
	}
	c.metricsConsumer = nextConsumer
	return c, nil
}
