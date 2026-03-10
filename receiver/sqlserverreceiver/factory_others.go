// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
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

	opts, err := setupScrapers(params, cfg)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig,
		params,
		metricsConsumer,
		opts...,
	)
}

// createLogsReceiver create a logs receiver based on provided config.
// When events (db.server.top_query, db.server.query_sample) are enabled, each is collected
// at its own collection_interval via WithLogsSchedules.
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

	opts, err := setupLogsScrapers(params, cfg)
	if err != nil {
		return nil, err
	}

	// Use per-event collection intervals when any event is enabled.
	if cfg.Events.DbServerQuerySample.Enabled || cfg.Events.DbServerTopQuery.Enabled {
		schedules := buildLogsSchedules(cfg, logsConsumer)
		opts = append(opts, scraperhelper.WithLogsSchedules(schedules))
	}

	return scraperhelper.NewLogsController(
		&cfg.ControllerConfig,
		params,
		logsConsumer,
		opts...,
	)
}
