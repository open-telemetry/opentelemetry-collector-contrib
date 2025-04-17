// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 10 * time.Second

	return &Config{
		ControllerConfig: cfg,
		AddrConfig: confignet.AddrConfig{
			Endpoint:  "localhost:5432",
			Transport: confignet.TransportTypeTCP,
		},
		ClientConfig: configtls.ClientConfig{
			Insecure:           false,
			InsecureSkipVerify: true,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		QuerySampleCollection: QuerySampleCollection{
			Enabled:         false,
			MaxRowsPerQuery: 1000,
		},
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)

	var clientFactory postgreSQLClientFactory
	if connectionPoolGate.IsEnabled() {
		clientFactory = newPoolClientFactory(cfg)
	} else {
		clientFactory = newDefaultClientFactory(cfg)
	}

	ns := newPostgreSQLScraper(params, cfg, clientFactory)
	s, err := scraper.NewMetrics(ns.scrape, scraper.WithShutdown(ns.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, params, consumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}

// createLogsReceiver create a logs receiver based on provided config.
func createLogsReceiver(
	_ context.Context,
	params receiver.Settings,
	receiverCfg component.Config,
	logsConsumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := receiverCfg.(*Config)

	var clientFactory postgreSQLClientFactory
	if connectionPoolGate.IsEnabled() {
		clientFactory = newPoolClientFactory(cfg)
	} else {
		clientFactory = newDefaultClientFactory(cfg)
	}

	ns := newPostgreSQLScraper(params, cfg, clientFactory)

	opts := make([]scraperhelper.ControllerOption, 0)

	//nolint:staticcheck
	if cfg.QuerySampleCollection.Enabled {
		s, err := scraper.NewLogs(func(ctx context.Context) (plog.Logs, error) {
			return ns.scrapeQuerySamples(ctx, cfg.MaxRowsPerQuery)
		}, scraper.WithShutdown(ns.shutdown))
		if err != nil {
			return nil, err
		}
		opt := scraperhelper.AddFactoryWithConfig(
			scraper.NewFactory(metadata.Type, nil,
				scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
					return s, nil
				}, component.StabilityLevelAlpha)), nil)
		opts = append(opts, opt)
	}

	return scraperhelper.NewLogsController(
		&cfg.ControllerConfig, params, logsConsumer, opts...,
	)
}
