// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

var errConfigNotSQLServer = errors.New("config was not a sqlserver receiver config")

// newCache creates a new cache with the given size.
// If the size is less or equal to 0, it will be set to 1.
// It will never return an error.
func newCache(size int) *lru.Cache[string, int64] {
	if size <= 0 {
		size = 1
	}
	// lru will only returns error when the size is less than 0
	cache, _ := lru.New[string, int64](size)
	return cache
}

// NewFactory creates a factory for SQL Server receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 10 * time.Second
	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		QuerySample: QuerySample{
			Enabled:         false,
			MaxRowsPerQuery: 100,
		},
		TopQueryCollection: TopQueryCollection{
			Enabled:             false,
			LookbackTime:        uint(2 * cfg.CollectionInterval / time.Second),
			MaxQuerySampleCount: 1000,
			TopQueryCount:       200,
		},
	}
}

func setupQueries(cfg *Config) []string {
	var queries []string

	if isDatabaseIOQueryEnabled(&cfg.MetricsBuilderConfig.Metrics) {
		queries = append(queries, getSQLServerDatabaseIOQuery(cfg.InstanceName))
	}

	if isPerfCounterQueryEnabled(&cfg.MetricsBuilderConfig.Metrics) {
		queries = append(queries, getSQLServerPerformanceCounterQuery(cfg.InstanceName))
	}

	if cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseCount.Enabled {
		queries = append(queries, getSQLServerPropertiesQuery(cfg.InstanceName))
	}

	return queries
}

func setupLogQueries(cfg *Config) ([]string, []error) {
	var queries []string
	var errs []error

	if cfg.QuerySample.Enabled {
		queries = append(queries, getSQLServerQuerySamplesQuery(cfg.MaxRowsPerQuery))
	}

	if cfg.TopQueryCollection.Enabled {
		q, err := getSQLServerQueryTextAndPlanQuery(cfg.InstanceName, cfg.MaxQuerySampleCount, cfg.LookbackTime)
		if err != nil {
			errs = append(errs, err)
		} else {
			queries = append(queries, q)
		}
	}

	return queries, errs
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

		// lru only returns error when the size is less than 0
		cache := newCache(1)

		sqlServerScraper := newSQLServerScraper(id, query,
			sqlquery.TelemetryConfig{},
			dbProviderFunc,
			sqlquery.NewDbClient,
			params,
			cfg,
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

	queries, errs := setupLogQueries(cfg)
	if len(errs) > 0 {
		params.Logger.Error("Failed to template queries in SQLServer receiver: Configuration might not be correct.", zap.Error(errors.Join(errs...)))
		return nil
	}

	if len(queries) == 0 {
		params.Logger.Info("No direct connection will be made to the SQL Server: No logs are enabled requiring it.")
		return nil
	}

	queryTextAndPlanQuery, err := getSQLServerQueryTextAndPlanQuery(cfg.InstanceName, cfg.MaxQuerySampleCount, cfg.LookbackTime)
	if err != nil {
		params.Logger.Error("Failed to template needed queries in SQLServer receiver: Configuration might not be correct.", zap.Error(err))
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

		cache := newCache(1)

		if query == queryTextAndPlanQuery {
			// we have 8 metrics in this query and multiple 2 to allow to cache more queries.
			cache = newCache(int(cfg.MaxQuerySampleCount * 8 * 2))
		}

		if query == getSQLServerQuerySamplesQuery(cfg.MaxRowsPerQuery) {
			cache = newCache(1)
		}

		sqlServerScraper := newSQLServerScraper(id, query,
			sqlquery.TelemetryConfig{},
			dbProviderFunc,
			sqlquery.NewDbClient,
			params,
			cfg,
			cache)

		scrapers = append(scrapers, sqlServerScraper)
	}

	return scrapers
}

// Note: This method will fail silently if there is no work to do. This is an acceptable use case
// as this receiver can still get information on Windows from performance counters without a direct
// connection. Messages will be logged at the INFO level in such cases.
func setupScrapers(params receiver.Settings, cfg *Config) ([]scraperhelper.ControllerOption, error) {
	sqlServerScrapers := setupSQLServerScrapers(params, cfg)

	var opts []scraperhelper.ControllerOption
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
func setupLogsScrapers(params receiver.Settings, cfg *Config) ([]scraperhelper.ControllerOption, error) {
	sqlServerScrapers := setupSQLServerLogsScrapers(params, cfg)

	var opts []scraperhelper.ControllerOption
	for _, sqlScraper := range sqlServerScrapers {
		s, err := scraper.NewLogs(sqlScraper.ScrapeLogs,
			scraper.WithStart(sqlScraper.Start),
			scraper.WithShutdown(sqlScraper.Shutdown))
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

	return opts, nil
}

func isDatabaseIOQueryEnabled(metrics *metadata.MetricsConfig) bool {
	if metrics == nil {
		return false
	}

	return metrics.SqlserverDatabaseLatency.Enabled ||
		metrics.SqlserverDatabaseOperations.Enabled ||
		metrics.SqlserverDatabaseIo.Enabled
}

func isPerfCounterQueryEnabled(metrics *metadata.MetricsConfig) bool {
	if metrics == nil {
		return false
	}

	return metrics.SqlserverBatchRequestRate.Enabled ||
		metrics.SqlserverBatchSQLCompilationRate.Enabled ||
		metrics.SqlserverBatchSQLRecompilationRate.Enabled ||
		metrics.SqlserverDatabaseBackupOrRestoreRate.Enabled ||
		metrics.SqlserverDatabaseExecutionErrors.Enabled ||
		metrics.SqlserverDatabaseFullScanRate.Enabled ||
		metrics.SqlserverDatabaseTempdbSpace.Enabled ||
		metrics.SqlserverDatabaseTempdbVersionStoreSize.Enabled ||
		metrics.SqlserverDeadlockRate.Enabled ||
		metrics.SqlserverIndexSearchRate.Enabled ||
		metrics.SqlserverLockTimeoutRate.Enabled ||
		metrics.SqlserverLockWaitRate.Enabled ||
		metrics.SqlserverLoginRate.Enabled ||
		metrics.SqlserverLogoutRate.Enabled ||
		metrics.SqlserverMemoryGrantsPendingCount.Enabled ||
		metrics.SqlserverMemoryUsage.Enabled ||
		metrics.SqlserverPageBufferCacheFreeListStallsRate.Enabled ||
		metrics.SqlserverPageBufferCacheHitRatio.Enabled ||
		metrics.SqlserverPageLookupRate.Enabled ||
		metrics.SqlserverProcessesBlocked.Enabled ||
		metrics.SqlserverReplicaDataRate.Enabled ||
		metrics.SqlserverResourcePoolDiskThrottledReadRate.Enabled ||
		metrics.SqlserverResourcePoolDiskThrottledWriteRate.Enabled ||
		metrics.SqlserverTableCount.Enabled ||
		metrics.SqlserverTransactionDelay.Enabled ||
		metrics.SqlserverTransactionMirrorWriteRate.Enabled ||
		metrics.SqlserverUserConnectionCount.Enabled
}
