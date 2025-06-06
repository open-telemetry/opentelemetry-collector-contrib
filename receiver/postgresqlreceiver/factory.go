// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"context"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
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

// newCache creates a new cache with the given size.
// If the size is less or equal to 0, it will be set to 1.
// It will never return an error.
func newCache(size int) *lru.Cache[string, float64] {
	if size <= 0 {
		size = 1
	}
	// lru will only return error when the size is less than 0
	cache, _ := lru.New[string, float64](size)
	return cache
}

func newTTLCache[v any](size int, ttl time.Duration) *expirable.LRU[string, v] {
	if size <= 0 {
		size = 1
	}
	cache := expirable.NewLRU[string, v](size, nil, ttl)
	return cache
}

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
		TopQueryCollection: TopQueryCollection{
			Enabled:                false,
			TopNQuery:              1000,
			MaxRowsPerQuery:        1000,
			MaxExplainEachInterval: 1000,
			QueryPlanCacheSize:     1000,
			QueryPlanCacheTTL:      time.Hour,
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

	ns := newPostgreSQLScraper(params, cfg, clientFactory, newCache(1), newTTLCache[string](1, time.Second))
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

	opts := make([]scraperhelper.ControllerOption, 0)

	if cfg.QuerySampleCollection.Enabled {
		// query sample collection does not need cache, but we do not want to make it
		// nil, so create one size 1 cache as a placeholder.
		ns := newPostgreSQLScraper(params, cfg, clientFactory, newCache(1), newTTLCache[string](1, time.Second))
		s, err := scraper.NewLogs(func(ctx context.Context) (plog.Logs, error) {
			return ns.scrapeQuerySamples(ctx, cfg.QuerySampleCollection.MaxRowsPerQuery)
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

	if cfg.TopQueryCollection.Enabled {
		// we have 10 updated only attributes. so we set the cache size accordingly.
		ns := newPostgreSQLScraper(params, cfg, clientFactory, newCache(int(cfg.TopNQuery*10*2)), newTTLCache[string](cfg.QueryPlanCacheSize, cfg.QueryPlanCacheTTL))
		s, err := scraper.NewLogs(func(ctx context.Context) (plog.Logs, error) {
			return ns.scrapeTopQuery(ctx, cfg.TopQueryCollection.MaxRowsPerQuery, cfg.TopNQuery, cfg.MaxExplainEachInterval)
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
