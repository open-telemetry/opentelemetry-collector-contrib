package nsxreceiver

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const typeStr = "nsx"

var errConfigNotNSX = errors.New("config was not a NSX receiver config")

// NewFactory creates a new receiver factory
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver))
}

func createDefaultConfig() config.Receiver {
	return &Config{}
}

func createMetricsReceiver(ctx context.Context, params component.ReceiverCreateSettings, rConf config.Receiver, consumer consumer.Metrics) (component.MetricsReceiver, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotNSX
	}
	return scraperhelper.NewScraperControllerReceiver(&cfg.ScraperControllerSettings, params, consumer)
}
