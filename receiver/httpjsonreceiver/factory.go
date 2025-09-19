package httpjsonreceiver // import "httpjsonreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var (
	componentType = component.MustNewType("httpjson") // Fixed: Use MustNewType
)

const (
	stability = component.StabilityLevelBeta
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		componentType, // Use the component.Type variable
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		CollectionInterval: 60 * time.Second,
		InitialDelay:       time.Second,
		Timeout:            10 * time.Second,
		ResourceAttributes: make(map[string]string),
	}
}

func createMetricsReceiver(
	ctx context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	return NewReceiver(cfg, consumer, params)
}
