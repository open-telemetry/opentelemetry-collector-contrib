// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcsmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcsmetricsreceiver"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcsmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcsmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcsmetricsreceiver/internal/scraper/githubscraper"
)

// This file implements a factory for the vcs provider receiver

var (
	scraperFactories = map[string]internal.ScraperFactory{
		githubscraper.TypeStr: &githubscraper.Factory{},
	}

	errConfigNotValid = errors.New("configuration is not valid for the vcs provider receiver")
)

// NewFactory creates a factory for the vcs provider receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
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
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		// TODO: metrics builder configuration may need to be in each sub scraper,
		// TODO: for right now setting here because the metrics in this receiver will apply to all
		// TODO: scrapers defined as a common set of vcsmetrics
		// TODO: aqp completely remove these comments if the metrics build config
		// needs to be defined in each scraper
		// MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// Create the metrics receiver according to the OTEL conventions taking in the
// context, receiver params, configuration from the component, and consumer (process or exporter)
func createMetricsReceiver(
	ctx context.Context,
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {

	// check that the configuration is valid
	conf, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigNotValid
	}

	addScraperOpts, err := createAddScraperOpts(ctx, params, conf, scraperFactories)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&conf.ControllerConfig,
		params,
		consumer,
		addScraperOpts...,
	)
}

func createAddScraperOpts(
	ctx context.Context,
	params receiver.Settings,
	cfg *Config,
	factories map[string]internal.ScraperFactory,
) ([]scraperhelper.ScraperControllerOption, error) {
	scraperControllerOptions := make([]scraperhelper.ScraperControllerOption, 0, len(cfg.Scrapers))

	for key, cfg := range cfg.Scrapers {
		vcsMetricsScraper, err := createVCSMetricsScraper(ctx, params, key, cfg, factories)

		if err != nil {
			return nil, fmt.Errorf("failed to create scraper %q: %w", key, err)
		}

		scraperControllerOptions = append(scraperControllerOptions, scraperhelper.AddScraper(vcsMetricsScraper))
	}

	return scraperControllerOptions, nil
}

func createVCSMetricsScraper(
	ctx context.Context,
	params receiver.Settings,
	key string,
	cfg internal.Config,
	factories map[string]internal.ScraperFactory,
) (scraper scraperhelper.Scraper, err error) {
	factory := factories[key]
	if factory == nil {
		return nil, fmt.Errorf("factory not found for scraper %q", key)
	}

	scraper, err = factory.CreateMetricsScraper(ctx, params, cfg)
	if err != nil {
		return nil, err
	}

	return
}
