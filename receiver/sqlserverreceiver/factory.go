// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
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
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
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

	if isDatabaseIOQueryEnabled(&cfg.Metrics) {
		queries = append(queries, getSQLServerDatabaseIOQuery(cfg.InstanceName))
	}

	if isPerfCounterQueryEnabled(&cfg.Metrics) {
		queries = append(queries, getSQLServerPerformanceCounterQuery(cfg.InstanceName))
	}

	if cfg.Metrics.SqlserverDatabaseCount.Enabled || cfg.Metrics.SqlserverCPUCount.Enabled || cfg.Metrics.SqlserverComputerUptime.Enabled {
		queries = append(queries, getSQLServerPropertiesQuery(cfg.InstanceName))
	}

	if isWaitStatsQueryEnabled(&cfg.Metrics) {
		queries = append(queries, getSQLServerWaitStatsQuery(cfg.InstanceName))
	}

	if isIndexPhysicalStatsQueryEnabled(&cfg.Metrics) {
		queries = append(queries, getSQLServerIndexPhysicalStatsQuery(cfg.InstanceName))
	}

	return queries
}

func setupLogQueries(cfg *Config) []string {
	var queries []string

	if cfg.Events.DbServerQuerySample.Enabled {
		queries = append(queries, getSQLServerQuerySamplesQuery())
	}

	if cfg.Events.DbServerTopQuery.Enabled {
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

// sqlServerMetricsReceiver wraps the scraper controller so that the shared
// connection pool is closed when the receiver shuts down.
type sqlServerMetricsReceiver struct {
	receiver.Metrics
	provider *dbProvider
}

func (r *sqlServerMetricsReceiver) Shutdown(ctx context.Context) error {
	err := r.Metrics.Shutdown(ctx)
	if r.provider != nil {
		err = errors.Join(err, r.provider.close())
	}
	return err
}

// sqlServerLogsReceiver wraps the scraper controller so that the shared
// connection pool is closed when the receiver shuts down.
type sqlServerLogsReceiver struct {
	receiver.Logs
	provider *dbProvider
}

func (r *sqlServerLogsReceiver) Shutdown(ctx context.Context) error {
	err := r.Logs.Shutdown(ctx)
	if r.provider != nil {
		err = errors.Join(err, r.provider.close())
	}
	return err
}

// dbProvider owns the single connection pool shared by all scrapers of a
// receiver. It is created in the factory so that the pool's ownership and
// lifecycle are tied to the receiver rather than to any individual scraper:
// the pool is opened lazily the first time a scraper starts and is closed once
// by the receiver on shutdown. A *sql.DB is safe for concurrent use and already
// maintains its own connection pool, so sharing one pool across all scrapers
// avoids creating a redundant, independently-managed pool per query.
type dbProvider struct {
	dsn         string
	pool        ConnectionPool
	numScrapers int

	mu       sync.Mutex
	db       *sql.DB
	openErr  error
	opened   bool
	closed   bool
	closeErr error
}

var errDBProviderClosed = errors.New("connection pool is closed")

func newDBProvider(cfg *Config, numScrapers int) *dbProvider {
	return &dbProvider{
		dsn:         getDBConnectionString(cfg),
		pool:        cfg.ConnectionPool,
		numScrapers: numScrapers,
	}
}

// getDB lazily opens and configures the shared pool, returning the same
// *sql.DB on every call. It satisfies sqlquery.DbProviderFunc. Once the
// provider has been closed it refuses to open a new pool, so a pool can never
// be created after close and leaked.
func (p *dbProvider) getDB() (*sql.DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, errDBProviderClosed
	}
	if !p.opened {
		p.opened = true
		p.db, p.openErr = sql.Open("sqlserver", p.dsn)
		if p.openErr == nil {
			setConnectionPoolSettings(p.db, p.pool, p.numScrapers)
		}
	}
	return p.db, p.openErr
}

// close closes the shared pool. It is idempotent and safe to call on a nil
// provider or when the pool was never opened. After close, getDB will not open
// a new pool.
func (p *dbProvider) close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return p.closeErr
	}
	p.closed = true
	if p.db != nil {
		p.closeErr = p.db.Close()
	}
	return p.closeErr
}

// setConnectionPoolSettings applies the configured pool settings, falling back
// to defaults derived from the number of scrapers that share the pool. The Go
// driver defaults (unlimited open connections, two idle connections) are
// sub-optimal when several scrapers query the same instance on every collection
// interval, so by default we size both limits to the number of scrapers: this
// lets every scraper run concurrently while bounding the total connections and
// avoiding idle-connection churn between intervals.
func setConnectionPoolSettings(db *sql.DB, pool ConnectionPool, numScrapers int) {
	if numScrapers < 1 {
		numScrapers = 1
	}

	maxOpen := numScrapers
	if pool.MaxOpen != nil {
		maxOpen = *pool.MaxOpen
	}
	db.SetMaxOpenConns(maxOpen)

	maxIdle := numScrapers
	if pool.MaxIdle != nil {
		maxIdle = *pool.MaxIdle
	}
	db.SetMaxIdleConns(maxIdle)

	if pool.MaxLifetime != nil {
		db.SetConnMaxLifetime(*pool.MaxLifetime)
	}
	if pool.MaxIdleTime != nil {
		db.SetConnMaxIdleTime(*pool.MaxIdleTime)
	}
}

// SQL Server scraper creation is split out into a separate method for the sake of testing.
// It returns the scrapers along with the shared connection pool provider, whose
// lifecycle is owned by the receiver. The provider is nil when no direct
// connection is made.
func setupSQLServerScrapers(params receiver.Settings, cfg *Config) ([]*sqlServerScraperHelper, *dbProvider) {
	if !cfg.isDirectDBConnectionEnabled {
		params.Logger.Info("No direct connection will be made to the SQL Server: Configuration doesn't include some options.")
		return nil, nil
	}

	queries := setupQueries(cfg)
	if len(queries) == 0 {
		params.Logger.Info("No direct connection will be made to the SQL Server: No metrics are enabled requiring it.")
		return nil, nil
	}

	// All scrapers of this receiver share a single connection pool so that the
	// number of pools does not grow with the number of enabled queries.
	provider := newDBProvider(cfg, len(queries))

	var scrapers []*sqlServerScraperHelper
	for i, query := range queries {
		id := component.NewIDWithName(metadata.Type, fmt.Sprintf("query-%d: %s", i, query))

		// lru only returns error when the size is less than 0
		cache := newCache(1)

		sqlServerScraper := newSQLServerScraper(id, query,
			sqlquery.TelemetryConfig{},
			provider.getDB,
			sqlquery.NewDbClient,
			params,
			cfg,
			cache)

		scrapers = append(scrapers, sqlServerScraper)
	}

	return scrapers, provider
}

// SQL Server scraper creation is split out into a separate method for the sake of testing.
// It returns the scrapers along with the shared connection pool provider, whose
// lifecycle is owned by the receiver. The provider is nil when no direct
// connection is made.
func setupSQLServerLogsScrapers(params receiver.Settings, cfg *Config) ([]*sqlServerScraperHelper, *dbProvider) {
	if !cfg.isDirectDBConnectionEnabled {
		params.Logger.Info("No direct connection will be made to the SQL Server: Configuration doesn't include some options.")
		return nil, nil
	}

	queries := setupLogQueries(cfg)

	if len(queries) == 0 {
		params.Logger.Info("No direct connection will be made to the SQL Server: No logs are enabled requiring it.")
		return nil, nil
	}

	// All scrapers of this receiver share a single connection pool so that the
	// number of pools does not grow with the number of enabled queries.
	provider := newDBProvider(cfg, len(queries))

	var scrapers []*sqlServerScraperHelper
	for i, query := range queries {
		id := component.NewIDWithName(metadata.Type, fmt.Sprintf("logs-query-%d: %s", i, query))

		cache := newCache(1)

		if query == getSQLServerQueryTextAndPlanQuery() {
			// we have 8 metrics in this query and multiple 2 to allow to cache more queries.
			cache = newCache(int(cfg.MaxQuerySampleCount * 8 * 2))
		}

		if query == getSQLServerQuerySamplesQuery() {
			cache = newCache(1)
		}

		sqlServerScraper := newSQLServerScraper(id, query,
			sqlquery.TelemetryConfig{},
			provider.getDB,
			sqlquery.NewDbClient,
			params,
			cfg,
			cache)

		scrapers = append(scrapers, sqlServerScraper)
	}

	return scrapers, provider
}

// Note: This method will fail silently if there is no work to do. This is an acceptable use case
// as this receiver can still get information on Windows from performance counters without a direct
// connection. Messages will be logged at the INFO level in such cases.
func setupScrapers(params receiver.Settings, cfg *Config) ([]scraperhelper.ControllerOption, *dbProvider, error) {
	sqlServerScrapers, provider := setupSQLServerScrapers(params, cfg)

	var opts []scraperhelper.ControllerOption
	for _, sqlScraper := range sqlServerScrapers {
		s, err := scraper.NewMetrics(sqlScraper.ScrapeMetrics,
			scraper.WithStart(sqlScraper.Start),
			scraper.WithShutdown(sqlScraper.Shutdown))
		if err != nil {
			// The provider owns the shared pool; close it so it is not leaked
			// when receiver construction fails before Shutdown can run.
			return nil, nil, errors.Join(err, provider.close())
		}

		opt := scraperhelper.AddMetricsScraper(metadata.Type, s)
		opts = append(opts, opt)
	}

	return opts, provider, nil
}

// Note: This method will fail silently if there is no work to do. This is an acceptable use case
// as this receiver can still get information on Windows from performance counters without a direct
// connection. Messages will be logged at the INFO level in such cases.
func setupLogsScrapers(params receiver.Settings, cfg *Config) ([]scraperhelper.ControllerOption, *dbProvider, error) {
	sqlServerScrapers, provider := setupSQLServerLogsScrapers(params, cfg)

	var opts []scraperhelper.ControllerOption
	for _, sqlScraper := range sqlServerScrapers {
		s, err := scraper.NewLogs(sqlScraper.ScrapeLogs,
			scraper.WithStart(sqlScraper.Start),
			scraper.WithShutdown(sqlScraper.Shutdown))
		if err != nil {
			// The provider owns the shared pool; close it so it is not leaked
			// when receiver construction fails before Shutdown can run.
			return nil, nil, errors.Join(err, provider.close())
		}

		opt := scraperhelper.AddFactoryWithConfig(
			scraper.NewFactory(metadata.Type, nil,
				scraper.WithLogs(func(context.Context, scraper.Settings, component.Config) (scraper.Logs, error) {
					return s, nil
				}, component.StabilityLevelAlpha)), nil)
		opts = append(opts, opt)
	}

	return opts, provider, nil
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
		metrics.SqlserverBatchRequestRate.Enabled ||
		metrics.SqlserverBatchSQLCompilationRate.Enabled ||
		metrics.SqlserverBatchSQLRecompilationRate.Enabled ||
		metrics.SqlserverConnectionResetRate.Enabled ||
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
		metrics.SqlserverProcessesBlocked.Enabled ||
		metrics.SqlserverReplicaDataRate.Enabled ||
		metrics.SqlserverResourcePoolDiskThrottledReadRate.Enabled ||
		metrics.SqlserverResourcePoolDiskOperations.Enabled ||
		metrics.SqlserverResourcePoolDiskThrottledWriteRate.Enabled ||
		metrics.SqlserverScanPointRevalidationRate.Enabled ||
		metrics.SqlserverAttentionRate.Enabled ||
		metrics.SqlserverParameterizationRate.Enabled ||
		metrics.SqlserverPlanExecutionRate.Enabled ||
		metrics.SqlserverRecompilationRatio.Enabled ||
		metrics.SqlserverTableCount.Enabled ||
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
