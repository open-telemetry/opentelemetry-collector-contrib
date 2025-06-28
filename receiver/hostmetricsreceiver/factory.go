// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/process"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/systemscraper"
)

const (
	defaultMetadataCollectionInterval = 5 * time.Minute
)

// This file implements Factory for HostMetrics receiver.
var (
	scraperFactories = mustMakeFactories(
		cpuscraper.NewFactory(),
		diskscraper.NewFactory(),
		filesystemscraper.NewFactory(),
		loadscraper.NewFactory(),
		memoryscraper.NewFactory(),
		networkscraper.NewFactory(),
		pagingscraper.NewFactory(),
		processesscraper.NewFactory(),
		processscraper.NewFactory(),
		systemscraper.NewFactory(),
	)
)

func mustMakeFactories(factories ...scraper.Factory) map[component.Type]scraper.Factory {
	fMap := map[component.Type]scraper.Factory{}
	for _, f := range factories {
		if _, ok := fMap[f.Type()]; ok {
			panic(fmt.Errorf("duplicate scraper factory %q", f.Type()))
		}
		fMap[f.Type()] = f
	}
	return fMap
}

// NewFactory creates a new factory for host metrics receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

// createDefaultConfig creates the default configuration for receiver.
func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig:           scraperhelper.NewDefaultControllerConfig(),
		MetadataCollectionInterval: defaultMetadataCollectionInterval,
	}
}

// createMetricsReceiver creates a metrics receiver based on provided config.
func createMetricsReceiver(
	ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	oCfg := cfg.(*Config)

	addScraperOptions, err := createAddScraperOptions(ctx, oCfg, scraperFactories)
	if err != nil {
		return nil, err
	}

	host.EnableBootTimeCache(true)
	process.EnableBootTimeCache(true)

	return scraperhelper.NewMetricsController(
		&oCfg.ControllerConfig,
		set,
		consumer,
		addScraperOptions...,
	)
}

func createLogsReceiver(
	_ context.Context, set receiver.Settings, cfg component.Config, consumer consumer.Logs,
) (receiver.Logs, error) {
	return &hostEntitiesReceiver{
		cfg:      cfg.(*Config),
		nextLogs: consumer,
		settings: &set,
	}, nil
}

func createAddScraperOptions(
	_ context.Context,
	cfg *Config,
	factories map[component.Type]scraper.Factory,
) ([]scraperhelper.ControllerOption, error) {
	scraperControllerOptions := make([]scraperhelper.ControllerOption, 0, len(cfg.Scrapers))

	envMap := gopsutilenv.SetGoPsutilEnvVars(cfg.RootPath)

	for key, cfg := range cfg.Scrapers {
		factory, err := getFactory(key, factories)
		if err != nil {
			return nil, err
		}
		factory = internal.NewEnvVarFactory(factory, envMap)
		scraperControllerOptions = append(scraperControllerOptions, scraperhelper.AddFactoryWithConfig(factory, cfg))
	}

	return scraperControllerOptions, nil
}

func getFactory(key component.Type, factories map[component.Type]scraper.Factory) (s scraper.Factory, err error) {
	factory, ok := factories[key]
	if !ok {
		return nil, fmt.Errorf("host metrics scraper factory not found for key: %q", key)
	}

	return factory, nil
}
