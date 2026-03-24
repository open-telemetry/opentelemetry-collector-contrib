// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsekshyperpodreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsekshyperpodreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	otelscraper "go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsekshyperpodreceiver/internal/metadata"
)

// NewFactory creates a new receiver factory for the HyperPod health receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
	}
	cfg.CollectionInterval = 60 * time.Second
	return cfg
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := baseCfg.(*Config)
	s := newScraper(cfg, settings)
	scrp, err := otelscraper.NewMetrics(
		s.scrape,
		otelscraper.WithStart(s.start),
		otelscraper.WithShutdown(s.shutdown),
	)
	if err != nil {
		return nil, err
	}
	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, settings, consumer,
		scraperhelper.AddScraper(metadata.Type, scrp),
	)
}
