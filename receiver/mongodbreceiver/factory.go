// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

const (
	defaultMongoDBPort                = 27017
	defaultMaxRowsPerQuery            = 100
	defaultMaxQuerySampleCount        = int64(1000)
	defaultMaxExplainEachInterval     = int64(250)
	defaultTopQueryCount              = int64(500)
	defaultQueryPlanCacheSize         = 500
	defaultQueryPlanCacheTTL          = 10 * time.Minute
	defaultTopQueryCollectionInterval = 60 * time.Second
)

var defaultEndpoint = "localhost:" + strconv.Itoa(defaultMongoDBPort)

// NewFactory creates a factory for mongodb receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
		Timeout:          time.Minute,
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: defaultEndpoint,
			},
		},
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		LogsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
		QuerySampleCollection: QuerySampleCollection{
			MaxRowsPerQuery: defaultMaxRowsPerQuery,
		},
		TopQueryCollection: TopQueryCollection{
			CollectionInterval:     defaultTopQueryCollectionInterval,
			MaxQuerySampleCount:    defaultMaxQuerySampleCount,
			MaxExplainEachInterval: defaultMaxExplainEachInterval,
			TopQueryCount:          defaultTopQueryCount,
			QueryPlanCacheSize:     defaultQueryPlanCacheSize,
			QueryPlanCacheTTL:      defaultQueryPlanCacheTTL,
		},
		ClientConfig: configtls.ClientConfig{},
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := rConf.(*Config)
	ms := newMongodbScraper(params, cfg)

	s, err := scraper.NewMetrics(
		ms.scrape,
		scraper.WithStart(ms.start),
		scraper.WithShutdown(ms.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, params, consumer,
		scraperhelper.AddMetricsScraper(metadata.Type, s),
	)
}

func createLogsReceiver(_ context.Context, params receiver.Settings, rConf component.Config, consumer consumer.Logs) (receiver.Logs, error) {
	cfg := rConf.(*Config)
	ms := newMongodbScraper(params, cfg)

	opts := make([]scraperhelper.ControllerOption, 0)

	if cfg.Events.DbServerQuerySample.Enabled {
		s, err := scraper.NewLogs(
			ms.scrapeLogs,
			scraper.WithStart(ms.start),
			scraper.WithShutdown(ms.shutdown),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create log scraper: %w", err)
		}
		opts = append(
			opts,
			scraperhelper.AddFactoryWithConfig(
				scraper.NewFactory(
					metadata.Type, nil, scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
						return s, nil
					}, metadata.LogsStability)), nil))
	}

	if cfg.Events.DbServerTopQuery.Enabled {
		tqms := newMongodbScraper(params, cfg)
		tqms.planCache = buildPlanCache(cfg, params.Logger)
		tqs, err := scraper.NewLogs(
			tqms.scrapeTopQueryLogs,
			scraper.WithStart(tqms.start),
			scraper.WithShutdown(tqms.shutdown),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create top_query log scraper: %w", err)
		}
		opts = append(
			opts,
			scraperhelper.AddFactoryWithConfig(
				scraper.NewFactory(
					metadata.Type, nil, scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
						return tqs, nil
					}, metadata.LogsStability)), nil))
	}

	return scraperhelper.NewLogsController(
		&cfg.ControllerConfig, params, consumer, opts...)
}
