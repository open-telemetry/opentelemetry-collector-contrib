// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipvsreceiver // import "github.com/sergeysedoy97/opentelemetry-collector-contrib/receiver/ipvsreceiver"

import (
	"context"
	"errors"

	"github.com/sergeysedoy97/opentelemetry-collector-contrib/receiver/ipvsreceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetrics, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetrics(
	ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("configuration parsing error")
	}

	s := newScraper(config.MetricsBuilderConfig, set)
	sc, err := scraper.NewMetrics(
		s.scrape,
		scraper.WithStart(s.start),
		scraper.WithShutdown(s.shutdown),
	)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&config.ControllerConfig,
		set,
		nextConsumer,
		scraperhelper.AddScraper(metadata.Type, sc),
	)
}
