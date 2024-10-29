// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/metadata"
)

const (
	defaultEndpoint              = "localhost:3000"
	defaultTimeout               = 20 * time.Second
	defaultCollectClusterMetrics = false
)

// NewFactory creates a new ReceiverFactory with default configuration
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

// createMetricsReceiver creates a new MetricsReceiver using scraperhelper
func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	receiver, err := newAerospikeReceiver(params, cfg, consumer)
	if err != nil {
		return nil, err
	}

	scraper, err := scraperhelper.NewScraper(
		metadata.Type,
		receiver.scrape,
		scraperhelper.WithStart(receiver.start),
		scraperhelper.WithShutdown(receiver.shutdown),
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ControllerConfig, params, consumer,
		scraperhelper.AddScraper(scraper),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig:      scraperhelper.NewDefaultControllerConfig(),
		Endpoint:              defaultEndpoint,
		Timeout:               defaultTimeout,
		CollectClusterMetrics: defaultCollectClusterMetrics,
		MetricsBuilderConfig:  metadata.DefaultMetricsBuilderConfig(),
	}
}
