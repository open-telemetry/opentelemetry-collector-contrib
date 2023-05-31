package gitmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitmetricsreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitmetricsreceiver/internal/scraper/githubscraper"
)

// This file implements a factory for the git metrics receiver

const (
	typeStr         = "gitmetrics"
	defaultInterval = 30 * time.Second
	defaultTimeout = 15 * time.Second
	stability      = component.StabilityLevelDevelopment
)

var (
    scraperFactories = map[string]internal.ScraperFactory{
        githubscraper.TypeStr: &githubscraper.NewFactory(),
    }

	configNotValid = errors.New("configuration is not valid for the git metrics receiver")
)

// NewFactory creates a factory for the git metrics receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
	)
}

// Gets a factory for defined scraper.
func getScraperFactory(key string) (internal.ScraperFactory, bool) {
    if factory, ok := scraperFactories[key]; ok {
        return factory, true
    }

    return nil, false
}

// Create the default config based on the const(s) defined above.
func createDefaultConfig() component.Config {
	return &Config{
        ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
		MetricsBuilderConfig: internal.DefaultMetricsBuilderConfig(),
	}
}

// Create the metrics receiver according to the OTEL conventions taking in the
// context, receiver params, configuration from the component, and consumer (process or exporter)
func createMetricsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {

	// check that the configuration is valid
	conf, ok := cfg.(*Config)
	if !ok {
		return nil, configNotValid
	}

    addScraperOpts, err := createAddScraperOpts(ctx, params, conf, scraperFactories)
    if err != nil {
        return nil, err
    }

    return scraperhelper.NewScraperControllerReceiver(
        &conf.ScraperControllerSettings,
        params,
        consumer,
        addScraperOpts...,
    )
}

func createAddScraperOpts(
	ctx context.Context,
	params receiver.CreateSettings, 
	cfg *Config,
    factories map[string]internal.ScraperFactory,
) ([]scraperhelper.ScraperControllerOption, error) {
    scraperControllerOptions := make([]scraperhelper.ScraperControllerOption, 0, len(cfg.Scrapers))

    for key, cfg := range cfg.Scrapers {
        gitMetricsScraper, ok, err := createGitMetricsScraper(ctx, params, cfg, factories)
        if err != nil {
            return nil, fmt.Errorf("failed to create scraper %q: %w", key, err)
        }

        if ok {
            scraperControllerOptions = append(scraperControllerOptions, scraperhelper.AddScraper(gitMetricsScraper))
            continue
        }

        return nil, fmt.Errorf("git metrics scraper factory not found for key: %q", key)

    }

    return scraperControllerOptions, nil
}

func createGitMetricsScraper(
    ctx context.Context,
    params receiver.CreateSettings,
    key string,
    cfg internal.Config,
    factories map[string]internal.ScraperFactory,
) (scraper scraperhelper.Scraper, ok bool, err error) {
    factory := factories[key]
    if factory == nil {
        ok = false
        return
    }

    ok = true
    scraper, err = factory.CreateMetricsScraper(ctx, params, cfg)
    if err != nil {
        return nil, false, err
    }

    return 
}
