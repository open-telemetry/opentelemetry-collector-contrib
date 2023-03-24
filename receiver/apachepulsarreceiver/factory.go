package apachepulsarreceiver

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	typeStr   = "pulsar"
	stability = component.StabilityLevelDevelopment
)

var errConfigNotPulsar = errors.New("config was not a Pulsar receiver config")

func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		newDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
}

func newDefaultConfig() component.Config {
	return &component.Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)), find a way to replace this line
			CollectionInterval: 10 * time.Second,
		},
		// Endpoint: defaultEndpoint,
	}
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	pulsarConfig, ok := config.(*Config)
	if !ok {
		return nil, errConfigNotPulsar
	}

	// if err := addMissingConfigDefaults(pulsarConfig); err != nil {
	// 	return nil, fmt.Errorf("failed to validate added config defaults: %w", err)
	// }

	pulsarScraper := newScraper(params.Logger, pulsarConfig)
	scraper, err := scraperhelper.NewScraper(typeStr, pulsarScraper.scrape,
		scraperhelper.WithStart(pulsarScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&pulsarConfig.ScraperControllerSettings, params,
		consumer, scraperhelper.AddScraper(scraper))

}

// func addMissingConfigDefaults(cfg *Config) error {
// 	// Add the schema prefix to the endpoint if it doesn't contain one
// 	if !strings.Contains(cfg.Endpoint, "://") {
// 		cfg.Endpoint = "udp://" + cfg.Endpoint
// 	}

// 	u, err := url.Parse(cfg.Endpoint)
// 	if err == nil && u.Port() == "" {
// 		portSuffix := "8080"
// 		if cfg.Endpoint[len(cfg.Endpoint)-1:] != ":" {
// 			portSuffix = ":" + portSuffix
// 		}
// 		cfg.Endpoint += portSuffix
// 	}

// 	for _, metricCfg := range cfg.Metrics {
// 		if metricCfg.Unit == "" {
// 			metricCfg.Unit = "1"
// 		}
// 		if metricCfg.Gauge != nil && metricCfg.Gauge.ValueType == "" {
// 			metricCfg.Gauge.ValueType = "float"
// 		}
// 		if metricCfg.Sum != nil {
// 			if metricCfg.Sum.ValueType == "" {
// 				metricCfg.Sum.ValueType = "float"
// 			}
// 			if metricCfg.Sum.Aggregation == "" {
// 				metricCfg.Sum.Aggregation = "cumulative"
// 			}
// 		}
// 	}
// 	return cfg.Validate()
// }
