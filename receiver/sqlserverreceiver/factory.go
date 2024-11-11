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
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

var errConfigNotSQLServer = errors.New("config was not a sqlserver receiver config")

// NewFactory creates a factory for SQL Server receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = 10 * time.Second
	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
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

		sqlServerScraper := newSQLServerScraper(id, query,
			cfg.InstanceName,
			cfg.ControllerConfig,
			params.Logger,
			sqlquery.TelemetryConfig{},
			dbProviderFunc,
			sqlquery.NewDbClient,
			metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params))

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
		scraper, err := scraperhelper.NewScraper(metadata.Type, sqlScraper.Scrape,
			scraperhelper.WithStart(sqlScraper.Start),
			scraperhelper.WithShutdown(sqlScraper.Shutdown))

		if err != nil {
			return nil, err
		}

		opt := scraperhelper.AddScraper(scraper)
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
