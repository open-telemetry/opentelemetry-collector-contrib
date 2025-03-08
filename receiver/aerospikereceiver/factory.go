// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

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
	r, err := newAerospikeReceiver(params, cfg, consumer)
	if err != nil {
		return nil, err
	}

	s, err := scraper.NewMetrics(r.scrape,
		scraper.WithStart(r.start),
		scraper.WithShutdown(r.shutdown),
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, params, consumer,
		scraperhelper.AddScraper(metadata.Type, s),
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
