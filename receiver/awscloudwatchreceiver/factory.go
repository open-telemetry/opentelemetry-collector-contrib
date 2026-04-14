// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver/internal/metadata"
)

// NewFactory returns the component factory for the awscloudwatchreceiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createLogsReceiver(
	_ context.Context,
	settings receiver.Settings,
	rConf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := rConf.(*Config)
	rcvr := newLogsReceiver(cfg, settings, consumer)
	return rcvr, nil
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	scr := newCloudWatchMetricsScraper(cfg, settings)
	ms, err := scraper.NewMetrics(scr.scrape, scraper.WithStart(scr.start))
	if err != nil {
		return nil, err
	}
	return scraperhelper.NewMetricsController(
		&cfg.Metrics.ControllerConfig,
		settings,
		consumer,
		scraperhelper.AddMetricsScraper(metadata.Type, ms),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Logs: LogsConfig{
			PollInterval:        defaultPollInterval,
			MaxEventsPerRequest: defaultEventLimit,
			Groups: GroupConfig{
				AutodiscoverConfig: &AutodiscoverConfig{
					Limit: defaultLogGroupLimit,
				},
			},
		},
		Metrics: MetricsConfig{
			// CollectionInterval is required to be positive by scraperhelper.ControllerConfig.Validate().
			// Metrics are only collected when a metrics pipeline is configured; this default does not
			// enable metrics collection on its own.
			ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: defaultMetricsCollectionInt},
			Period:           defaultMetricsPeriod,
			Delay:            defaultMetricsDelay,
		},
	}
}
