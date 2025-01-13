// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zookeeperreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver/internal/metadata"
)

const (
	defaultEndpoint           = "localhost:2181"
	defaultCollectionInterval = 10 * time.Second
	defaultTimeout            = 10 * time.Second
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = defaultCollectionInterval
	cfg.Timeout = defaultTimeout

	return &Config{
		ControllerConfig: cfg,
		TCPAddrConfig: confignet.TCPAddrConfig{
			Endpoint: defaultEndpoint,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// CreateMetrics creates zookeeper (metrics) receiver.
func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	rConfig := config.(*Config)
	zms, err := newZookeeperMetricsScraper(params, rConfig)
	if err != nil {
		return nil, err
	}

	scrp, err := scraper.NewMetrics(
		zms.scrape,
		scraper.WithShutdown(zms.shutdown),
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&rConfig.ControllerConfig,
		params,
		consumer,
		scraperhelper.AddScraper(metadata.Type, scrp),
	)
}
