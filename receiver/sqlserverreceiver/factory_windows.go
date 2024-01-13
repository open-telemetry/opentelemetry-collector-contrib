// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

var errConfigNotSQLServer = errors.New("config was not a sqlserver receiver config")

// createMetricsReceiver creates a metrics receiver based on provided config.
func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	receiverCfg component.Config,
	metricsConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg, ok := receiverCfg.(*Config)
	if !ok {
		return nil, errConfigNotSQLServer
	}
	sqlServerScraper := newSQLServerScraper(params, cfg)

	scraper, err := scraperhelper.NewScraper(metadata.Type, sqlServerScraper.scrape,
		scraperhelper.WithStart(sqlServerScraper.start),
		scraperhelper.WithShutdown(sqlServerScraper.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings, params, metricsConsumer, scraperhelper.AddScraper(scraper),
	)
}
