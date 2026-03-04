// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"context"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
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
		ControllerConfig:     cfg,
		AllowNativePasswords: true,
		Username:             "root",
		AddrConfig: confignet.AddrConfig{
			Endpoint:  "localhost:3306",
			Transport: confignet.TransportTypeTCP,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		StatementEvents: StatementEventsConfig{
			DigestTextLimit: defaultStatementEventsDigestTextLimit,
			Limit:           defaultStatementEventsLimit,
			TimeLimit:       defaultStatementEventsTimeLimit,
		},
		TopQueryCollection: TopQueryCollection{
			LookbackTime:        60,
			MaxQuerySampleCount: 1000,
			TopQueryCount:       200,
			CollectionInterval:  60 * time.Second,
			QueryPlanCacheSize:  1000,
			QueryPlanCacheTTL:   time.Hour,
		},
		QuerySampleCollection: QuerySampleCollection{
			MaxRowsPerQuery: 100,
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

	ns := newMySQLScraper(params, cfg, newCache[int64](1), newTTLCache[string](0, time.Hour*24*365*10))
	s, err := scraper.NewMetrics(ns.scrape, scraper.WithStart(ns.start),
		scraper.WithShutdown(ns.shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig, params, consumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}

func createLogsReceiver(
	_ context.Context,
	params receiver.Settings,
	rConf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := rConf.(*Config)

	opts := make([]scraperhelper.ControllerOption, 0)

	if cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled {
		// we have 2 updated only attributes. so we set the cache size accordingly.
		// TODO: parameterize this cache size.
		ns := newMySQLScraper(params, cfg, newCache[int64](int(cfg.TopQueryCollection.MaxQuerySampleCount*2*2)), newTTLCache[string](cfg.TopQueryCollection.QueryPlanCacheSize, cfg.TopQueryCollection.QueryPlanCacheTTL))
		s, err := scraper.NewLogs(
			ns.scrapeTopQueryFunc,
			scraper.WithStart(ns.start),
			scraper.WithShutdown(ns.shutdown),
		)
		if err != nil {
			return nil, err
		}
		opt := scraperhelper.AddFactoryWithConfig(
			scraper.NewFactory(metadata.Type, nil,
				scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
					return s, nil
				}, component.StabilityLevelDevelopment)), nil)
		opts = append(opts, opt)
	}

	if cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled {
		// query sample collection does not need cache, but we do not want to make it
		// nil, so create one size 1 cache as a placeholder.
		ns := newMySQLScraper(params, cfg, newCache[int64](1), newTTLCache[string](0, time.Hour*24*365*10))
		s, err := scraper.NewLogs(
			ns.scrapeQuerySampleFunc,
			scraper.WithStart(ns.start),
			scraper.WithShutdown(ns.shutdown),
		)
		if err != nil {
			return nil, err
		}
		opt := scraperhelper.AddFactoryWithConfig(
			scraper.NewFactory(metadata.Type, nil,
				scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
					return s, nil
				}, component.StabilityLevelDevelopment)), nil)
		opts = append(opts, opt)
	}

	return scraperhelper.NewLogsController(
		&cfg.ControllerConfig, params, consumer,
		opts...,
	)
}

// newCache creates a new cache with the given size.
// If the size is less or equal to 0, it will be set to 1.
// It will never return an error.
func newCache[v any](size int) *lru.Cache[string, v] {
	if size <= 0 {
		size = 1
	}
	// Ignore returned error as lru will only return an error when the size is less than 0
	cache, _ := lru.New[string, v](size)
	return cache
}

func newTTLCache[v any](size int, ttl time.Duration) *expirable.LRU[string, v] {
	if size <= 0 {
		size = 1
	}
	cache := expirable.NewLRU[string, v](size, nil, ttl)
	return cache
}
