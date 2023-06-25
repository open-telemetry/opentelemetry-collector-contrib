// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"
)

// This file implements Factory for HostMetrics receiver.
var (
	scraperFactories = map[string]internal.ScraperFactory{
		cpuscraper.TypeStr:        &cpuscraper.Factory{},
		diskscraper.TypeStr:       &diskscraper.Factory{},
		loadscraper.TypeStr:       &loadscraper.Factory{},
		filesystemscraper.TypeStr: &filesystemscraper.Factory{},
		memoryscraper.TypeStr:     &memoryscraper.Factory{},
		networkscraper.TypeStr:    &networkscraper.Factory{},
		pagingscraper.TypeStr:     &pagingscraper.Factory{},
		processesscraper.TypeStr:  &processesscraper.Factory{},
		processscraper.TypeStr:    &processscraper.Factory{},
	}
)

// NewFactory creates a new factory for host metrics receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func getScraperFactory(key string) (internal.ScraperFactory, bool) {
	if factory, ok := scraperFactories[key]; ok {
		return factory, true
	}

	return nil, false
}

// createDefaultConfig creates the default configuration for receiver.
func createDefaultConfig() component.Config {
	return &Config{ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type)}
}

// createMetricsReceiver creates a metrics receiver based on provided config.
func createMetricsReceiver(
	ctx context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	oCfg := cfg.(*Config)

	addScraperOptions, err := createAddScraperOptions(ctx, set, oCfg, scraperFactories)
	if err != nil {
		return nil, err
	}

	if err = setGoPsutilEnvVars(oCfg.RootPath, &osEnv{}); err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&oCfg.ScraperControllerSettings,
		set,
		consumer,
		addScraperOptions...,
	)
}

func createAddScraperOptions(
	ctx context.Context,
	set receiver.CreateSettings,
	config *Config,
	factories map[string]internal.ScraperFactory,
) ([]scraperhelper.ScraperControllerOption, error) {
	scraperControllerOptions := make([]scraperhelper.ScraperControllerOption, 0, len(config.Scrapers))

	for key, cfg := range config.Scrapers {
		hostMetricsScraper, ok, err := createHostMetricsScraper(ctx, set, key, cfg, factories)
		if err != nil {
			return nil, fmt.Errorf("failed to create scraper for key %q: %w", key, err)
		}

		if ok {
			scraperControllerOptions = append(scraperControllerOptions, scraperhelper.AddScraper(hostMetricsScraper))
			continue
		}

		return nil, fmt.Errorf("host metrics scraper factory not found for key: %q", key)
	}

	return scraperControllerOptions, nil
}

func createHostMetricsScraper(ctx context.Context, set receiver.CreateSettings, key string, cfg internal.Config, factories map[string]internal.ScraperFactory) (scraper scraperhelper.Scraper, ok bool, err error) {
	factory := factories[key]
	if factory == nil {
		ok = false
		return
	}

	ok = true
	scraper, err = factory.CreateMetricsScraper(ctx, set, cfg)
	return
}

type environment interface {
	Lookup(k string) (string, bool)
	Set(k, v string) error
}

type osEnv struct{}

var _ environment = (*osEnv)(nil)

func (e *osEnv) Set(k, v string) error {
	return os.Setenv(k, v)
}

func (e *osEnv) Lookup(k string) (string, bool) {
	return os.LookupEnv(k)
}
