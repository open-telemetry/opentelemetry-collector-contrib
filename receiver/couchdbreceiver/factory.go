// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

// NewFactory creates the couchdbreceiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.TLS = configtls.ClientConfig{}
	clientConfig.Endpoint = defaultEndpoint
	clientConfig.Timeout = 1 * time.Minute
	return &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		ClientConfig:         clientConfig,
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	ns := newCouchdbScraper(params, cfg)
	s, err := scraper.NewMetrics(ns.scrape, scraper.WithStart(ns.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, params, consumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}
