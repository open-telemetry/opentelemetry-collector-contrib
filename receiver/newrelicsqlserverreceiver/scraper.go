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
	connection               *SQLConnection
	config                   *Config
	logger                   *zap.Logger
	startTime                pcommon.Timestamp
	settings                 receiver.Settings
	instanceScraper          *scrapers.InstanceScraper
	queryPerformanceScraper  *scrapers.QueryPerformanceScraper
	//slowQueryScraper  *scrapers.SlowQueryScraper
	engineEdition            int // SQL Server engine edition (0=Unknown, 5=Azure DB, 8=Azure MI)
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
	s.instanceScraper = scrapers.NewInstanceScraper(s.connection, s.logger, s.engineEdition)
	
	// Initialize query performance scraper for blocking sessions and performance monitoring
	s.queryPerformanceScraper = scrapers.NewQueryPerformanceScraper(s.connection, s.logger, s.engineEdition)
	//s.slowQueryScraper = scrapers.NewSlowQueryScraper(s.logger, s.connection)
	

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
	if s.config.EnableBufferMetrics {
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

	// Scrape blocking session metrics if query monitoring is enabled
	if s.config.EnableQueryMonitoring {
		scrapeCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
		defer cancel()

		if err := s.queryPerformanceScraper.ScrapeBlockingSessionMetrics(scrapeCtx, scopeMetrics); err != nil {
			s.logger.Error("Failed to scrape blocking session metrics",
				zap.Error(err),
				zap.Duration("timeout", s.config.Timeout))
			scrapeErrors = append(scrapeErrors, err)
			// Don't return here - continue with other metrics if enabled
		} else {
			s.logger.Debug("Successfully scraped blocking session metrics")
		}
	}

// 	if s.config.EnableQueryMonitoring {
//     scrapeCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
//     defer cancel()

//     if err := s.slowQueryScraper.ScrapeSlowQueryMetrics(scrapeCtx, scopeMetrics); err != nil {
//         s.logger.Error("Failed to scrape slow query metrics",
//             zap.Error(err),
//             zap.Duration("timeout", s.config.Timeout))
//         scrapeErrors = append(scrapeErrors, err)
//     } else {
//         s.logger.Debug("Successfully scraped slow query metrics")
//     }
// }

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
