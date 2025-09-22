// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/scrapers"
)

// sqlServerScraper handles SQL Server metrics collection following nri-mssql patterns
type sqlServerScraper struct {
	connection      *SQLConnection
	config          *Config
	logger          *zap.Logger
	startTime       pcommon.Timestamp
	settings        receiver.Settings
	instanceScraper *scrapers.InstanceScraper
	databaseScraper *scrapers.DatabaseScraper
	engineEdition   int // SQL Server engine edition (0=Unknown, 5=Azure DB, 8=Azure MI)
}

// newSqlServerScraper creates a new SQL Server scraper with structured approach
func newSqlServerScraper(settings receiver.Settings, cfg *Config) *sqlServerScraper {
	return &sqlServerScraper{
		config:   cfg,
		logger:   settings.Logger,
		settings: settings,
	}
}

// start initializes the scraper and establishes database connection
func (s *sqlServerScraper) start(ctx context.Context, _ component.Host) error {
	s.logger.Info("Starting New Relic SQL Server receiver")

	connection, err := NewSQLConnection(ctx, s.config, s.logger)
	if err != nil {
		s.logger.Error("Failed to connect to SQL Server", zap.Error(err))
		return err
	}
	s.connection = connection
	s.startTime = pcommon.NewTimestampFromTime(time.Now())

	if err := s.connection.Ping(ctx); err != nil {
		s.logger.Error("Failed to ping SQL Server", zap.Error(err))
		return err
	}

	// Get EngineEdition (following nri-mssql pattern)
	s.engineEdition = 0 // Default to 0 (Unknown)
	s.engineEdition, err = s.detectEngineEdition(ctx)
	if err != nil {
		s.logger.Debug("Failed to get engine edition, using default", zap.Error(err))
		s.engineEdition = queries.StandardSQLServerEngineEdition
	} else {
		s.logger.Info("Detected SQL Server engine edition",
			zap.Int("engine_edition", s.engineEdition),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
	}

	// Initialize instance scraper with engine edition for engine-specific queries
	// Create instance scraper for instance-level metrics
	s.instanceScraper = scrapers.NewInstanceScraper(s.connection, s.logger, s.engineEdition)
	
	// Create database scraper for database-level metrics
	s.databaseScraper = scrapers.NewDatabaseScraper(s.connection, s.logger, s.engineEdition)

	s.logger.Info("Successfully connected to SQL Server",
		zap.String("hostname", s.config.Hostname),
		zap.String("port", s.config.Port),
		zap.Int("engine_edition", s.engineEdition),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	return nil
}

// shutdown closes the database connection
func (s *sqlServerScraper) shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down New Relic SQL Server receiver")
	if s.connection != nil {
		s.connection.Close()
	}
	return nil
}

// detectEngineEdition detects the SQL Server engine edition following nri-mssql pattern
func (s *sqlServerScraper) detectEngineEdition(ctx context.Context) (int, error) {
	queryFunc := func(query string) (int, error) {
		var results []struct {
			EngineEdition int `db:"EngineEdition"`
		}

		err := s.connection.Query(ctx, &results, query)
		if err != nil {
			return 0, err
		}

		if len(results) == 0 {
			s.logger.Debug("EngineEdition query returned empty output.")
			return 0, nil
		}

		s.logger.Debug("Detected EngineEdition", zap.Int("engine_edition", results[0].EngineEdition))
		return results[0].EngineEdition, nil
	}

	return queries.DetectEngineEdition(queryFunc)
}

// scrape collects SQL Server instance metrics using structured approach
func (s *sqlServerScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Starting SQL Server metrics collection",
		zap.String("hostname", s.config.Hostname),
		zap.String("port", s.config.Port))

	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()

	// Set resource attributes with error handling
	attrs := resourceMetrics.Resource().Attributes()
	attrs.PutStr("server.address", s.config.Hostname)
	attrs.PutStr("server.port", s.config.Port)
	attrs.PutStr("db.system", "mssql")
	attrs.PutStr("service.name", "sql-server-monitoring")

	// Add instance name if configured
	if s.config.Instance != "" {
		attrs.PutStr("db.instance", s.config.Instance)
	}

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("newrelicsqlserverreceiver")

	// Track scraping errors but continue with partial results
	var scrapeErrors []error

	// Check if connection is still valid before scraping
	if s.connection != nil {
		if err := s.connection.Ping(ctx); err != nil {
			s.logger.Error("Connection health check failed before scraping", zap.Error(err))
			scrapeErrors = append(scrapeErrors, fmt.Errorf("connection health check failed: %w", err))
			// Continue with scraping attempt - connection might recover
		}
	} else {
		s.logger.Error("No database connection available for scraping")
		return metrics, fmt.Errorf("no database connection available")
	}

	// Use structured approach to scrape instance buffer metrics with timeout
	if s.config.IsBufferMetricsEnabled() {
		scrapeCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()

		if err := s.instanceScraper.ScrapeInstanceBufferMetrics(scrapeCtx, scopeMetrics); err != nil {
			s.logger.Error("Failed to scrape instance buffer metrics",
				zap.Error(err),
				zap.Duration("timeout", s.config.Timeout))
			scrapeErrors = append(scrapeErrors, err)
			// Don't return here - continue with other metrics if enabled
		} else {
			s.logger.Debug("Successfully scraped instance buffer metrics")
		}
	}

	// Scrape database-level buffer pool metrics (bufferpool.sizePerDatabaseInBytes)
	if s.config.IsBufferMetricsEnabled() {
		scrapeCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()

		if err := s.databaseScraper.ScrapeDatabaseBufferMetrics(scrapeCtx, scopeMetrics); err != nil {
			s.logger.Error("Failed to scrape database buffer metrics",
				zap.Error(err),
				zap.Duration("timeout", s.config.Timeout))
			scrapeErrors = append(scrapeErrors, err)
			// Don't return here - continue with other metrics
		} else {
			s.logger.Debug("Successfully scraped database buffer metrics")
		}
	}

	// Scrape database-level disk metrics (maxDiskSizeInBytes)
	s.logger.Debug("Checking disk metrics configuration",
		zap.Bool("enable_disk_metrics_in_bytes", s.config.EnableDiskMetricsInBytes))
	
	if s.config.IsDiskMetricsInBytesEnabled() {
		s.logger.Debug("Starting database disk metrics scraping")
		scrapeCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()

		if err := s.databaseScraper.ScrapeDatabaseDiskMetrics(scrapeCtx, scopeMetrics); err != nil {
			s.logger.Error("Failed to scrape database disk metrics",
				zap.Error(err),
				zap.Duration("timeout", s.config.Timeout))
			scrapeErrors = append(scrapeErrors, err)
			// Don't return here - continue with other metrics
		} else {
			s.logger.Debug("Successfully scraped database disk metrics")
		}
	} else {
		s.logger.Debug("Database disk metrics disabled in configuration")
	}

	// Scrape database-level IO metrics (io.stallInMilliseconds)
	if s.config.IsIOMetricsEnabled() {
		s.logger.Debug("Starting database IO metrics scraping")
		scrapeCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()

		if err := s.databaseScraper.ScrapeDatabaseIOMetrics(scrapeCtx, scopeMetrics); err != nil {
			s.logger.Error("Failed to scrape database IO metrics",
				zap.Error(err),
				zap.Duration("timeout", s.config.Timeout))
			scrapeErrors = append(scrapeErrors, err)
			// Don't return here - continue with other metrics
		} else {
			s.logger.Debug("Successfully scraped database IO metrics")
		}
	} else {
		s.logger.Debug("Database IO metrics disabled in configuration")
	}

	// Scrape database-level log growth metrics (log.transactionGrowth)
	if s.config.IsLogGrowthMetricsEnabled() {
		s.logger.Debug("Starting database log growth metrics scraping")
		scrapeCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()

		if err := s.databaseScraper.ScrapeDatabaseLogGrowthMetrics(scrapeCtx, scopeMetrics); err != nil {
			s.logger.Error("Failed to scrape database log growth metrics",
				zap.Error(err),
				zap.Duration("timeout", s.config.Timeout))
			scrapeErrors = append(scrapeErrors, err)
			// Don't return here - continue with other metrics
		} else {
			s.logger.Debug("Successfully scraped database log growth metrics")
		}
	} else {
		s.logger.Debug("Database log growth metrics disabled in configuration")
	}

	// Scrape database-level page file metrics (pageFileAvailable)
	if s.config.IsPageFileMetricsEnabled() {
		s.logger.Debug("Starting database page file metrics scraping")
		scrapeCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()

		if err := s.databaseScraper.ScrapeDatabasePageFileMetrics(scrapeCtx, scopeMetrics); err != nil {
			s.logger.Error("Failed to scrape database page file metrics",
				zap.Error(err),
				zap.Duration("timeout", s.config.Timeout))
			scrapeErrors = append(scrapeErrors, err)
			// Don't return here - continue with other metrics
		} else {
			s.logger.Debug("Successfully scraped database page file metrics")
		}
	} else {
		s.logger.Debug("Database page file metrics disabled in configuration")
	}

	// Scrape database-level page file total metrics (pageFileTotal)
	if s.config.IsPageFileTotalMetricsEnabled() {
		s.logger.Debug("Starting database page file total metrics scraping")
		scrapeCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()

		if err := s.databaseScraper.ScrapeDatabasePageFileTotalMetrics(scrapeCtx, scopeMetrics); err != nil {
			s.logger.Error("Failed to scrape database page file total metrics",
				zap.Error(err),
				zap.Duration("timeout", s.config.Timeout))
			scrapeErrors = append(scrapeErrors, err)
			// Don't return here - continue with other metrics
		} else {
			s.logger.Debug("Successfully scraped database page file total metrics")
		}
	} else {
		s.logger.Debug("Database page file total metrics disabled in configuration")
	}

	// Scrape instance-level memory metrics (memoryTotal, memoryAvailable, memoryUtilization)
	if s.config.IsMemoryMetricsEnabled() || s.config.IsMemoryTotalMetricsEnabled() || s.config.IsMemoryAvailableMetricsEnabled() || s.config.IsMemoryUtilizationMetricsEnabled() {
		s.logger.Debug("Starting database memory metrics scraping")
		scrapeCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()

		if err := s.databaseScraper.ScrapeDatabaseMemoryMetrics(scrapeCtx, scopeMetrics); err != nil {
			s.logger.Error("Failed to scrape database memory metrics",
				zap.Error(err),
				zap.Duration("timeout", s.config.Timeout))
			scrapeErrors = append(scrapeErrors, err)
			// Don't return here - continue with other metrics
		} else {
			s.logger.Debug("Successfully scraped database memory metrics")
		}
	} else {
		s.logger.Debug("Database memory metrics disabled in configuration")
	}

	// Log summary of scraping results
	if len(scrapeErrors) > 0 {
		s.logger.Warn("Completed scraping with errors",
			zap.Int("error_count", len(scrapeErrors)),
			zap.Int("metrics_collected", scopeMetrics.Metrics().Len()))

		// Return the first error but with partial metrics
		return metrics, scrapeErrors[0]
	}

	s.logger.Debug("Successfully completed SQL Server metrics collection",
		zap.Int("metrics_collected", scopeMetrics.Metrics().Len()))

	return metrics, nil
}
