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
	_ "github.com/microsoft/go-mssqldb"                     // register Db driver
	_ "github.com/microsoft/go-mssqldb/integratedauth/krb5" // register Db driver
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

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
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 10 * time.Second
	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		LogsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
		QuerySample: QuerySample{
			MaxRowsPerQuery: 100,
		},
		TopQueryCollection: TopQueryCollection{
			MaxQuerySampleCount: 1000,
			TopQueryCount:       250,
			CollectionInterval:  time.Minute,
		},
	}
}

func setupQueries(cfg *Config) []string {
	var queries []string

	if isAvailabilityGroupQueryEnabled(&cfg.MetricsBuilderConfig.Metrics) {
		queries = append(queries, getSQLServerAvailabilityGroupQuery(cfg.InstanceName))
	}

	if isDatabaseIOQueryEnabled(&cfg.MetricsBuilderConfig.Metrics) {
		queries = append(queries, getSQLServerDatabaseIOQuery(cfg.InstanceName))
	}

	if isPerfCounterQueryEnabled(&cfg.MetricsBuilderConfig.Metrics) {
		queries = append(queries, getSQLServerPerformanceCounterQuery(cfg.InstanceName))
	}

	if cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseCount.Enabled || cfg.MetricsBuilderConfig.Metrics.SqlserverCPUCount.Enabled || cfg.MetricsBuilderConfig.Metrics.SqlserverComputerUptime.Enabled {
		queries = append(queries, getSQLServerPropertiesQuery(cfg.InstanceName))
	}

	if isWaitStatsQueryEnabled(&cfg.MetricsBuilderConfig.Metrics) {
		queries = append(queries, getSQLServerWaitStatsQuery(cfg.InstanceName))
	}

	if isWorkerThreadsQueryEnabled(&cfg.MetricsBuilderConfig.Metrics) {
		queries = append(queries, getSQLServerWorkerThreadsQuery(cfg.InstanceName))
	}

	if isIndexPhysicalStatsQueryEnabled(&cfg.MetricsBuilderConfig.Metrics) {
		queries = append(queries, getSQLServerIndexPhysicalStatsQuery(cfg.InstanceName))
	}

	if isCPUMemoryQueryEnabled(&cfg.MetricsBuilderConfig.Metrics) {
		queries = append(queries, getSQLServerCPUMemoryQuery(cfg.InstanceName))
	}

	if isDiskIOQueryEnabled(&cfg.MetricsBuilderConfig.Metrics) {
		queries = append(queries, getSQLServerDiskIOQuery(cfg.InstanceName))
	}

	return queries
}

func setupLogQueries(cfg *Config) []string {
	var queries []string

	if cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled {
		queries = append(queries, getSQLServerQuerySamplesQuery())
	}

	if cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled {
		queries = append(queries, getSQLServerQueryTextAndPlanQuery())
	}

	return queries
}

// Assumes config has all information necessary to directly connect to the database
func getDBConnectionString(config *Config) string {
	if config.DataSource != "" {
		return config.DataSource
	}
	return fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d", config.Server, config.Username, string(config.Password), config.Port)
}

// SQL Server scraper creation is split out into a separate method for the sake of testing.
func setupSQLServerScrapers(params receiver.Settings, cfg *Config) []*sqlServerScraperHelper {
	if !cfg.isDirectDBConnectionEnabled {
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
	if !cfg.isDirectDBConnectionEnabled {
		params.Logger.Info("No direct connection will be made to the SQL Server: Configuration doesn't include some options.")
		return nil
	}

	queries := setupLogQueries(cfg)

	if len(queries) == 0 {
		params.Logger.Info("No direct connection will be made to the SQL Server: No logs are enabled requiring it.")
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

		if query == getSQLServerQueryTextAndPlanQuery() {
			// we have 8 metrics in this query and multiple 2 to allow to cache more queries.
			cache = newCache(int(cfg.TopQueryCollection.MaxQuerySampleCount * 8 * 2))
		}

		if query == getSQLServerQuerySamplesQuery() {
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
func setupScrapers(params receiver.Settings, cfg *Config) []scraperhelper.ControllerOption {
	// Every scraper this receiver runs already implements scraper.Metrics, so
	// they can be handed to AddMetricsScraper directly.
	var scrapers []scraper.Metrics
	for _, sqlScraper := range setupSQLServerScrapers(params, cfg) {
		scrapers = append(scrapers, sqlScraper)
	}
	if healthScraper := setupConnectionHealthScraper(params, cfg); healthScraper != nil {
		scrapers = append(scrapers, healthScraper)
	}

	var opts []scraperhelper.ControllerOption
	for _, s := range scrapers {
		opts = append(opts, scraperhelper.AddMetricsScraper(metadata.Type, s))
	}

	return opts
}

// setupConnectionHealthScraper creates the scraper backing the sqlserver.health
// metric. It requires a direct DB connection (server/port/credentials or a
// datasource); it returns nil when the receiver runs in performance-counter-only
// mode or when the health metric is disabled.
func setupConnectionHealthScraper(params receiver.Settings, cfg *Config) *connectionHealthScraper {
	if !cfg.isDirectDBConnectionEnabled {
		return nil
	}

	if !cfg.MetricsBuilderConfig.Metrics.SqlserverHealth.Enabled {
		return nil
	}

	dbProviderFunc := func() (*sql.DB, error) {
		return sql.Open("sqlserver", getDBConnectionString(cfg))
	}

	id := component.NewIDWithName(metadata.Type, "connection-health")
	return newConnectionHealthScraper(id, sqlquery.TelemetryConfig{}, dbProviderFunc, sqlquery.NewDbClient, params, cfg)
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
				}, component.StabilityLevelAlpha)), nil,
		)
		opts = append(opts, opt)
	}

	return opts, nil
}

func isAvailabilityGroupQueryEnabled(metrics *metadata.MetricsConfig) bool {
	if metrics == nil {
		return false
	}

	return metrics.SqlserverAvailabilityGroupDatabaseReplicaSecondaryLag.Enabled ||
		metrics.SqlserverAvailabilityGroupDatabaseReplicaQueueSize.Enabled ||
		metrics.SqlserverAvailabilityGroupDatabaseReplicaQueueRate.Enabled
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

	return metrics.SqlserverAccessScanRate.Enabled ||
		metrics.SqlserverAttentionRate.Enabled ||
		metrics.SqlserverBatchRequestRate.Enabled ||
		metrics.SqlserverBatchSQLCompilationRate.Enabled ||
		metrics.SqlserverBatchSQLRecompilationRate.Enabled ||
		metrics.SqlserverClrExecutionTime.Enabled ||
		metrics.SqlserverConnectionResetRate.Enabled ||
		metrics.SqlserverCursorCount.Enabled ||
		metrics.SqlserverCursorMemoryUsage.Enabled ||
		metrics.SqlserverCursorPlanCount.Enabled ||
		metrics.SqlserverCursorRequestRate.Enabled ||
		metrics.SqlserverDatabaseBackupOrRestoreRate.Enabled ||
		metrics.SqlserverDatabaseExecutionErrors.Enabled ||
		metrics.SqlserverDatabaseFullScanRate.Enabled ||
		metrics.SqlserverDatabaseTempdbSpace.Enabled ||
		metrics.SqlserverDatabaseTempdbVersionStoreSize.Enabled ||
		metrics.SqlserverDeadlockRate.Enabled ||
		metrics.SqlserverErrorRate.Enabled ||
		metrics.SqlserverExtentOperationRate.Enabled ||
		metrics.SqlserverGhostRecordSkippedRate.Enabled ||
		metrics.SqlserverIndexSearchRate.Enabled ||
		metrics.SqlserverLatchSuperlatchCount.Enabled ||
		metrics.SqlserverLatchSuperlatchTransitionRate.Enabled ||
		metrics.SqlserverLatchWaitRate.Enabled ||
		metrics.SqlserverLatchWaitTimeAvg.Enabled ||
		metrics.SqlserverLatchWaitTimeTotal.Enabled ||
		metrics.SqlserverLockBlockCount.Enabled ||
		metrics.SqlserverLockEscalationRate.Enabled ||
		metrics.SqlserverLockMemory.Enabled ||
		metrics.SqlserverLockRequestRate.Enabled ||
		metrics.SqlserverLockTimeoutRate.Enabled ||
		metrics.SqlserverLockWaitCount.Enabled ||
		metrics.SqlserverLockWaitRate.Enabled ||
		metrics.SqlserverLockWaitTimeTotal.Enabled ||
		metrics.SqlserverLoginRate.Enabled ||
		metrics.SqlserverLogoutRate.Enabled ||
		metrics.SqlserverMemoryArea.Enabled ||
		metrics.SqlserverMemoryCacheObjectCount.Enabled ||
		metrics.SqlserverMemoryGrantsPendingCount.Enabled ||
		metrics.SqlserverMemoryPageCount.Enabled ||
		metrics.SqlserverMemoryUsage.Enabled ||
		metrics.SqlserverPageAllocationRate.Enabled ||
		metrics.SqlserverPageBufferCacheFreeListStallsRate.Enabled ||
		metrics.SqlserverPageBufferCacheHitRatio.Enabled ||
		metrics.SqlserverPageCompressionRate.Enabled ||
		metrics.SqlserverPageLookupRate.Enabled ||
		metrics.SqlserverPageReadAheadRate.Enabled ||
		metrics.SqlserverParameterizationRate.Enabled ||
		metrics.SqlserverPlanExecutionRate.Enabled ||
		metrics.SqlserverProcessesBlocked.Enabled ||
		metrics.SqlserverRecompilationRatio.Enabled ||
		metrics.SqlserverReplicaDataRate.Enabled ||
		metrics.SqlserverResourcePoolDiskOperations.Enabled ||
		metrics.SqlserverResourcePoolDiskThrottledReadRate.Enabled ||
		metrics.SqlserverResourcePoolDiskThrottledWriteRate.Enabled ||
		metrics.SqlserverScanPointRevalidationRate.Enabled ||
		metrics.SqlserverStoredProcedureInvocationRate.Enabled ||
		metrics.SqlserverTableCount.Enabled ||
		metrics.SqlserverTaskCount.Enabled ||
		metrics.SqlserverTaskRate.Enabled ||
		metrics.SqlserverTransactionDelay.Enabled ||
		metrics.SqlserverTransactionMirrorWriteRate.Enabled ||
		metrics.SqlserverUserConnectionCount.Enabled ||
		metrics.SqlserverWorktableCacheHitRatio.Enabled
}

func isWaitStatsQueryEnabled(metrics *metadata.MetricsConfig) bool {
	if metrics == nil {
		return false
	}

	return metrics.SqlserverOsWaitDuration.Enabled
}

func isWorkerThreadsQueryEnabled(metrics *metadata.MetricsConfig) bool {
	if metrics == nil {
		return false
	}

	return metrics.SqlserverWorkerRequestCount.Enabled ||
		metrics.SqlserverWorkerThreadCount.Enabled
}

func isIndexPhysicalStatsQueryEnabled(metrics *metadata.MetricsConfig) bool {
	if metrics == nil {
		return false
	}

	return metrics.SqlserverIndexFragmentation.Enabled ||
		metrics.SqlserverIndexPageCount.Enabled ||
		metrics.SqlserverIndexPageUtilization.Enabled ||
		metrics.SqlserverIndexRecordCount.Enabled ||
		metrics.SqlserverIndexSize.Enabled
}

func isCPUMemoryQueryEnabled(metrics *metadata.MetricsConfig) bool {
	if metrics == nil {
		return false
	}

	return metrics.SqlserverCPUUtilization.Enabled ||
		metrics.SqlserverHostMemoryLimit.Enabled ||
		metrics.SqlserverHostMemoryUsage.Enabled
}

func isDiskIOQueryEnabled(metrics *metadata.MetricsConfig) bool {
	if metrics == nil {
		return false
	}

	return metrics.SqlserverDiskOperations.Enabled ||
		metrics.SqlserverDiskIo.Enabled
}
