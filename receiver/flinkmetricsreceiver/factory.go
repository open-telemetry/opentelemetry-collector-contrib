// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flinkmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver/internal/metadata"
)

var errConfigNotflinkmetrics = errors.New("config was not a flinkmetrics receiver config")

// NewFactory creates a new receiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 10 * time.Second
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint
	clientConfig.Timeout = 10 * time.Second

	return &Config{
		ControllerConfig:     cfg,
		ClientConfig:         clientConfig,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotflinkmetrics
	}
	ns := newflinkScraper(cfg, params)
	s, err := scraper.NewMetrics(ns.scrape, scraper.WithStart(ns.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ControllerConfig, params, consumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}
