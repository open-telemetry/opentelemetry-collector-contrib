package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	typeStr = "aerospike"
)

// NewFactory creates a new ReceiverFactory with default configuration
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsReceiver(createMetricsReceiver),
	)
}

// createMetricsReceiver creates a new MetricsReceiver using scraperhelper
func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	rConf config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	cfg := rConf.(*Config)
	receiver, err := newAerospikeReceiver(params, cfg, consumer)
	if err != nil {
		return nil, err
	}

	scraper, err := scraperhelper.NewScraper(typeStr, receiver.scrape,
		scraperhelper.WithStart(receiver.start),
		scraperhelper.WithShutdown(receiver.shutdown),
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings, params, consumer,
		scraperhelper.AddScraper(scraper),
	)
}
