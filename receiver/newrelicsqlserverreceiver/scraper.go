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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
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
	s.instanceScraper = scrapers.NewInstanceScraper(s.connection, s.logger, s.engineEdition)

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

	// Set basic resource attributes
	attrs := resourceMetrics.Resource().Attributes()
	attrs.PutStr("server.address", s.config.Hostname)
	attrs.PutStr("server.port", s.config.Port)
	attrs.PutStr("db.system", "mssql")
	attrs.PutStr("service.name", "sql-server-monitoring")

	// Add instance name if configured
	if s.config.Instance != "" {
		attrs.PutStr("db.instance", s.config.Instance)
	}

	// Collect and add comprehensive system/host information as resource attributes
	if err := s.addSystemInformationAsResourceAttributes(ctx, attrs); err != nil {
		s.logger.Warn("Failed to collect system information, continuing with basic attributes",
			zap.Error(err))
		// Continue with scraping - system info is supplementary
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

	// ScrapeInstanceMemoryMetrics
	scrapeCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	if err := s.instanceScraper.ScrapeInstanceMemoryMetrics(scrapeCtx, scopeMetrics); err != nil {
		s.logger.Error("Failed to scrape instance memory metrics",
			zap.Error(err),
			zap.Duration("timeout", s.config.Timeout))
		scrapeErrors = append(scrapeErrors, err)
		// Don't return here - continue with other metrics if enabled
	} else {
		s.logger.Debug("Successfully scraped instance memory metrics")
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

// addSystemInformationAsResourceAttributes collects system/host information and adds it as resource attributes
// This ensures that all metrics sent by the scraper include comprehensive host context
func (s *sqlServerScraper) addSystemInformationAsResourceAttributes(ctx context.Context, attrs pcommon.Map) error {
	// Collect system information using the main scraper
	systemInfo, err := s.CollectSystemInformation(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect system information: %w", err)
	}

	// Add SQL Server instance information
	if systemInfo.ServerName != nil && *systemInfo.ServerName != "" {
		attrs.PutStr("sql.instance_name", *systemInfo.ServerName)
	}
	if systemInfo.ComputerName != nil && *systemInfo.ComputerName != "" {
		attrs.PutStr("host.name", *systemInfo.ComputerName)
	}
	if systemInfo.ServiceName != nil && *systemInfo.ServiceName != "" {
		attrs.PutStr("sql.service_name", *systemInfo.ServiceName)
	}

	// Add SQL Server edition and version information
	if systemInfo.Edition != nil && *systemInfo.Edition != "" {
		attrs.PutStr("sql.edition", *systemInfo.Edition)
	}
	if systemInfo.EngineEdition != nil {
		attrs.PutInt("sql.engine_edition", int64(*systemInfo.EngineEdition))
	}
	if systemInfo.ProductVersion != nil && *systemInfo.ProductVersion != "" {
		attrs.PutStr("sql.version", *systemInfo.ProductVersion)
	}
	if systemInfo.VersionDesc != nil && *systemInfo.VersionDesc != "" {
		attrs.PutStr("sql.version_description", *systemInfo.VersionDesc)
	}

	// Add hardware information
	if systemInfo.CPUCount != nil {
		attrs.PutInt("host.cpu.count", int64(*systemInfo.CPUCount))
	}
	if systemInfo.ServerMemoryKB != nil {
		attrs.PutInt("host.memory.total_kb", *systemInfo.ServerMemoryKB)
	}
	if systemInfo.AvailableMemoryKB != nil {
		attrs.PutInt("host.memory.available_kb", *systemInfo.AvailableMemoryKB)
	}

	// Add instance configuration
	if systemInfo.IsClustered != nil {
		attrs.PutBool("sql.is_clustered", *systemInfo.IsClustered)
	}
	if systemInfo.IsHadrEnabled != nil {
		attrs.PutBool("sql.is_hadr_enabled", *systemInfo.IsHadrEnabled)
	}
	if systemInfo.Uptime != nil {
		attrs.PutInt("sql.uptime_minutes", int64(*systemInfo.Uptime))
	}
	if systemInfo.ComputerUptime != nil {
		attrs.PutInt("host.uptime_seconds", int64(*systemInfo.ComputerUptime))
	}

	// Add network configuration
	if systemInfo.Port != nil && *systemInfo.Port != "" {
		attrs.PutStr("sql.port", *systemInfo.Port)
	}
	if systemInfo.PortType != nil && *systemInfo.PortType != "" {
		attrs.PutStr("sql.port_type", *systemInfo.PortType)
	}
	if systemInfo.ForceEncryption != nil {
		attrs.PutBool("sql.force_encryption", *systemInfo.ForceEncryption != 0)
	}

	s.logger.Debug("Successfully added system information as resource attributes",
		zap.String("host_name", getStringValueFromMap(systemInfo.ComputerName)),
		zap.String("sql_instance", getStringValueFromMap(systemInfo.ServerName)),
		zap.String("sql_edition", getStringValueFromMap(systemInfo.Edition)),
		zap.Int("cpu_count", getIntValueFromMap(systemInfo.CPUCount)),
		zap.Bool("is_clustered", getBoolValueFromMap(systemInfo.IsClustered)))

	return nil
}

// Helper functions to safely extract values from pointers for logging
func getStringValueFromMap(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}

func getIntValueFromMap(ptr *int) int {
	if ptr != nil {
		return *ptr
	}
	return 0
}

func getInt64ValueFromMap(ptr *int64) int64 {
	if ptr != nil {
		return *ptr
	}
	return 0
}

func getBoolValueFromMap(ptr *bool) bool {
	if ptr != nil {
		return *ptr
	}
	return false
}

// CollectSystemInformation retrieves comprehensive system and host information
// This information should be included as resource attributes with all metrics
func (s *sqlServerScraper) CollectSystemInformation(ctx context.Context) (*models.SystemInformation, error) {
	s.logger.Debug("Collecting SQL Server system and host information")

	var results []models.SystemInformation
	if err := s.connection.Query(ctx, &results, queries.SystemInformationQuery); err != nil {
		s.logger.Error("Failed to execute system information query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(queries.SystemInformationQuery, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return nil, fmt.Errorf("failed to execute system information query: %w", err)
	}

	if len(results) == 0 {
		s.logger.Warn("No results returned from system information query - SQL Server may not be ready")
		return nil, fmt.Errorf("no results returned from system information query")
	}

	if len(results) > 1 {
		s.logger.Warn("Multiple results returned from system information query",
			zap.Int("result_count", len(results)))
	}

	result := results[0]

	// Log collected system information for debugging
	s.logger.Info("Successfully collected system information",
		zap.String("server_name", getStringValueFromMap(result.ServerName)),
		zap.String("computer_name", getStringValueFromMap(result.ComputerName)),
		zap.String("edition", getStringValueFromMap(result.Edition)),
		zap.Int("engine_edition", getIntValueFromMap(result.EngineEdition)),
		zap.String("product_version", getStringValueFromMap(result.ProductVersion)),
		zap.Int("cpu_count", getIntValueFromMap(result.CPUCount)),
		zap.Int64("server_memory_kb", getInt64ValueFromMap(result.ServerMemoryKB)),
		zap.Bool("is_clustered", getBoolValueFromMap(result.IsClustered)),
		zap.Bool("is_hadr_enabled", getBoolValueFromMap(result.IsHadrEnabled)))

	return &result, nil
}
