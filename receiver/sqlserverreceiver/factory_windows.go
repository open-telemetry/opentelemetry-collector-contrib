// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

// createMetricsReceiver creates a metrics receiver based on provided config.
func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	receiverCfg component.Config,
	metricsConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg, ok := receiverCfg.(*Config)
	if !ok {
		return nil, errConfigNotSQLServer
	}
	sqlServerScraper := newSQLServerPCScraper(params, cfg)

	scraper, err := scraper.NewMetrics(sqlServerScraper.scrape,
		scraper.WithStart(sqlServerScraper.start),
		scraper.WithShutdown(sqlServerScraper.shutdown))
	if err != nil {
		return nil, err
	}

	var opts []scraperhelper.ScraperControllerOption
	opts, err = setupScrapers(params, cfg)
	if err != nil {
		return nil, err
	}
	opts = append(opts, scraperhelper.AddScraper(metadata.Type, scraper))

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ControllerConfig,
		params,
		metricsConsumer,
		opts...,
	)
}
