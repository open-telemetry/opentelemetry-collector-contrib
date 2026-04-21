// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zookeeperreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/scraper/zookeeperscraper"
)

const (
	defaultCollectionInterval = 10 * time.Second
	defaultTimeout            = 10 * time.Second
)

var sFact = zookeeperscraper.NewFactory()

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
		Config:           *sFact.CreateDefaultConfig().(*zookeeperscraper.Config),
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
	return scraperhelper.NewMetricsController(
		&rConfig.ControllerConfig,
		params,
		consumer,
		scraperhelper.AddFactoryWithConfig(sFact, &rConfig.Config),
	)
}
