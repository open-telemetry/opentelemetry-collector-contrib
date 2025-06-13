// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

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

	var opts []scraperhelper.ControllerOption
	opts, err = setupScrapers(params, cfg)
	if err != nil {
		return nil, err
	}
	opts = append(opts, scraperhelper.AddScraper(metadata.Type, scraper))

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig,
		params,
		metricsConsumer,
		opts...,
	)
}

// createLogsReceiver create a logs receiver based on provided config.
func createLogsReceiver(
	_ context.Context,
	params receiver.Settings,
	receiverCfg component.Config,
	logsConsumer consumer.Logs,
) (receiver.Logs, error) {
	cfg, ok := receiverCfg.(*Config)
	if !ok {
		return nil, errConfigNotSQLServer
	}

	// Disable logs receiver on Windows as the only supported logs query (Top Query) is not tested on Windows yet.
	opts := []scraperhelper.ControllerOption{}

	return scraperhelper.NewLogsController(
		&cfg.ControllerConfig,
		params,
		logsConsumer,
		opts...,
	)
}
