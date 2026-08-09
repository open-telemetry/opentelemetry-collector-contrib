// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"
	"errors"

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

	opts, provider, err := setupScrapers(params, cfg)
	if err != nil {
		return nil, err
	}

	controller, err := scraperhelper.NewMetricsController(
		&cfg.ControllerConfig,
		params,
		metricsConsumer,
		opts...,
	)
	if err != nil {
		return nil, errors.Join(err, provider.close())
	}

	return &sqlServerMetricsReceiver{Metrics: controller, provider: provider}, nil
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

	opts, provider, err := setupLogsScrapers(params, cfg)
	if err != nil {
		return nil, err
	}

	controller, err := scraperhelper.NewLogsController(
		&cfg.ControllerConfig,
		params,
		logsConsumer,
		opts...,
	)
	if err != nil {
		return nil, errors.Join(err, provider.close())
	}

	return &sqlServerLogsReceiver{Logs: controller, provider: provider}, nil
}
