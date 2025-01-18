// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/metadata"
)

var errConfigNotRabbit = errors.New("config was not a RabbitMQ receiver config")

// NewFactory creates a new receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

// createDefaultConfig creates the default configuration for the RabbitMQ receiver.
func createDefaultConfig() component.Config {
	defaultControllerConfig := scraperhelper.NewDefaultControllerConfig()
	defaultControllerConfig.CollectionInterval = 10 * time.Second

	defaultClientConfig := confighttp.NewDefaultClientConfig()
	defaultClientConfig.Endpoint = defaultEndpoint
	defaultClientConfig.Timeout = 10 * time.Second

	return &Config{
		ControllerConfig:     defaultControllerConfig,
		ClientConfig:         defaultClientConfig,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		EnableNodeMetrics:    true, // Default to enabling node metrics.
	}
}

// createMetricsReceiver creates the metrics receiver for RabbitMQ.
func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotRabbit
	}

	rabbitScraper := newScraper(params.Logger, cfg, params)

	s, err := scraper.NewMetrics(
		rabbitScraper.scrape,
		scraper.WithStart(rabbitScraper.start),
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig,
		params,
		consumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}
