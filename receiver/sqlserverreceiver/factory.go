// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

var errConfigNotSQLServer = errors.New("config was not a sqlserver receiver config")

// NewFactory creates a factory for SQL Server receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 10 * time.Second
	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Granularity:          10,
		MaxQuerySampleCount:  10000,
		TopQueryCount:        200,
	}
}

func setupQueries(cfg *Config) []string {
	var queries []string

	if isDatabaseIOQueryEnabled(&cfg.MetricsBuilderConfig.Metrics) {
		queries = append(queries, getSQLServerDatabaseIOQuery(cfg.InstanceName))
	}

	if cfg.MetricsBuilderConfig.Metrics.SqlserverBatchRequestRate.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverPageBufferCacheHitRatio.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverResourcePoolDiskThrottledReadRate.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverResourcePoolDiskThrottledWriteRate.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverLockWaitRate.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverProcessesBlocked.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverBatchSQLRecompilationRate.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverBatchSQLCompilationRate.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverUserConnectionCount.Enabled {
		queries = append(queries, getSQLServerPerformanceCounterQuery(cfg.InstanceName))
	}

	if cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseCount.Enabled {
		queries = append(queries, getSQLServerPropertiesQuery(cfg.InstanceName))
	}

	if cfg.MetricsBuilderConfig.Metrics.SqlserverQueryExecutionCount.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalElapsedTime.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalGrantKb.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalLogicalReads.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalLogicalWrites.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalPhysicalReads.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalRows.Enabled ||
		cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalWorkerTime.Enabled {
		queries = append(queries, getSQLServerQueryMetricsQuery(cfg.InstanceName, cfg.MaxQuerySampleCount, cfg.Granularity))
	}
	return queries
}

func setupLogQueries(cfg *Config) []string {
	var queries []string

	queries = append(queries, getQueryTextQuery(cfg.InstanceName, cfg.MaxQuerySampleCount, cfg.Granularity))
	queries = append(queries, getSQLServerQuerySamplesQuery())
	return queries
}

func directDBConnectionEnabled(config *Config) bool {
	return config.Server != "" &&
		config.Username != "" &&
		string(config.Password) != ""
}

// Assumes config has all information necessary to directly connect to the database
func getDBConnectionString(config *Config) string {
	return fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d", config.Server, config.Username, string(config.Password), config.Port)
}

// SQL Server scraper creation is split out into a separate method for the sake of testing.
func setupSQLServerScrapers(params receiver.Settings, cfg *Config) []*sqlServerScraperHelper {
	if !directDBConnectionEnabled(cfg) {
		params.Logger.Info("No direct connection will be made to the SQL Server: Configuration doesn't include some options.")
		return nil
	}

	queries := setupQueries(cfg)
	if len(queries) == 0 {
		params.Logger.Info("No direct connection will be made to the SQL Server: No metrics are enabled requiring it.")
		return nil
	}

	// TODO: Test if this needs to be re-defined for each scraper
	// This should be tested when there is more than one query being made.
	dbProviderFunc := func() (*sql.DB, error) {
		return sql.Open("sqlserver", getDBConnectionString(cfg))
	}

	var scrapers []*sqlServerScraperHelper
	for i, query := range queries {
		id := component.NewIDWithName(metadata.Type, fmt.Sprintf("query-%d: %s", i, query))

		var cache *lru.Cache[string, float64]
		var err error

		if query == getSQLServerQueryMetricsQuery(cfg.InstanceName, cfg.MaxQuerySampleCount, cfg.Granularity) {
			cache, err = lru.New[string, float64](10000 * 10)
			if err != nil {
				params.Logger.Error("Failed to create LRU cache, skipping the current scraper", zap.Error(err))
				continue
			}
		}

		sqlServerScraper := newSQLServerScraper(id, query,
			cfg.MaxQuerySampleCount,
			cfg.Granularity,
			cfg.TopQueryCount,
			cfg.InstanceName,
			cfg.ControllerConfig,
			params.Logger,
			sqlquery.TelemetryConfig{},
			dbProviderFunc,
			sqlquery.NewDbClient,
			metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
			cache)

		scrapers = append(scrapers, sqlServerScraper)
	}

	return scrapers
}

// SQL Server scraper creation is split out into a separate method for the sake of testing.
func setupSQLServerLogsScrapers(params receiver.Settings, cfg *Config) []*sqlServerScraperHelper {
	if !directDBConnectionEnabled(cfg) {
		params.Logger.Info("No direct connection will be made to the SQL Server: Configuration doesn't include some options.")
		return nil
	}

	queries := setupLogQueries(cfg)
	if len(queries) == 0 {
		params.Logger.Info("No direct connection will be made to the SQL Server: No metrics are enabled requiring it.")
		return nil
	}

	// TODO: Test if this needs to be re-defined for each scraper
	// This should be tested when there is more than one query being made.
	dbProviderFunc := func() (*sql.DB, error) {
		return sql.Open("sqlserver", getDBConnectionString(cfg))
	}

	var scrapers []*sqlServerScraperHelper
	for i, query := range queries {
		id := component.NewIDWithName(metadata.Type, fmt.Sprintf("logs-query-%d: %s", i, query))

		var cache *lru.Cache[string, float64]
		var err error

		if query == getQueryTextQuery(cfg.InstanceName, cfg.MaxQuerySampleCount, cfg.Granularity) {
			cache, err = lru.New[string, float64](10000 * 10)
			if err != nil {
				params.Logger.Error("Failed to create LRU cache, skipping the current scraper", zap.Error(err))
				continue
			}
		}
		if query == getSQLServerQuerySamplesQuery() {
			cache, err = lru.New[string, float64](10000 * 10)
			if err != nil {
				params.Logger.Error("Failed to create LRU cache, skipping the current scraper", zap.Error(err))
				continue
			}
		}

		sqlServerScraper := newSQLServerScraper(id, query,
			cfg.MaxQuerySampleCount,
			cfg.Granularity,
			cfg.TopQueryCount,
			cfg.InstanceName,
			cfg.ControllerConfig,
			params.Logger,
			sqlquery.TelemetryConfig{},
			dbProviderFunc,
			sqlquery.NewDbClient,
			metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
			cache)

		scrapers = append(scrapers, sqlServerScraper)
	}

	return scrapers
}

// Note: This method will fail silently if there is no work to do. This is an acceptable use case
// as this receiver can still get information on Windows from performance counters without a direct
// connection. Messages will be logged at the INFO level in such cases.
func setupScrapers(params receiver.Settings, cfg *Config) ([]scraperhelper.ScraperControllerOption, error) {
	sqlServerScrapers := setupSQLServerScrapers(params, cfg)

	var opts []scraperhelper.ScraperControllerOption
	for _, sqlScraper := range sqlServerScrapers {
		s, err := scraper.NewMetrics(sqlScraper.ScrapeMetrics,
			scraper.WithStart(sqlScraper.Start),
			scraper.WithShutdown(sqlScraper.Shutdown))
		if err != nil {
			return nil, err
		}

		opt := scraperhelper.AddScraper(metadata.Type, s)
		opts = append(opts, opt)
	}

	return opts, nil
}

// Note: This method will fail silently if there is no work to do. This is an acceptable use case
// as this receiver can still get information on Windows from performance counters without a direct
// connection. Messages will be logged at the INFO level in such cases.
func setupLogsScrapers(params receiver.Settings, cfg *Config) ([]scraperhelper.ScraperControllerOption, error) {
	sqlServerScrapers := setupSQLServerLogsScrapers(params, cfg)

	var opts []scraperhelper.ScraperControllerOption
	for _, sqlScraper := range sqlServerScrapers {
		s, err := scraper.NewLogs(sqlScraper.ScrapeLogs,
			scraper.WithStart(sqlScraper.Start),
			scraper.WithShutdown(sqlScraper.Shutdown))
		if err != nil {
			return nil, err
		}

		opt := scraperhelper.AddLogsScraper(metadata.Type, s)
		opts = append(opts, opt)
	}

	return opts, nil
}

func isDatabaseIOQueryEnabled(metrics *metadata.MetricsConfig) bool {
	if metrics.SqlserverDatabaseLatency.Enabled ||
		metrics.SqlserverDatabaseOperations.Enabled ||
		metrics.SqlserverDatabaseIo.Enabled {
		return true
	}
	return false
}
