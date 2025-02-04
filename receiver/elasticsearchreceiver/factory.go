// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
)

const (
	defaultCollectionInterval = 10 * time.Second
	defaultHTTPClientTimeout  = 10 * time.Second
)

// NewFactory creates a factory for elasticsearch receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

// createDefaultConfig creates the default elasticsearchreceiver config.
func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = defaultCollectionInterval

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint
	clientConfig.Timeout = defaultHTTPClientTimeout

	return &Config{
		ControllerConfig:     cfg,
		ClientConfig:         clientConfig,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Nodes:                []string{"_all"},
		Indices:              []string{"_all"},
	}
}

var errConfigNotES = errors.New("config was not an elasticsearch receiver config")

// createMetricsReceiver creates a metrics receiver for scraping elasticsearch metrics.
func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	c, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotES
	}
	es := newElasticSearchScraper(params, c)

	s, err := scraper.NewMetrics(es.scrape, scraper.WithStart(es.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&c.ControllerConfig,
		params,
		consumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}
