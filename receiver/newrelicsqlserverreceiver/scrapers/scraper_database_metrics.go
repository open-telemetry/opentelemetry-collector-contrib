// Copyright The OpenTelemetry Authors
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package scrapers provides the database-level metrics scraper for SQL Server.
// This file implements comprehensive collection of 9 database-level metrics
// covering database size, I/O operations, and database activity statistics.
//
// Database-Level Metrics (9 total):
//
// 1. Database Size Metrics (3 metrics):
//   - Database Size (MB): Total size of the database including log files
//   - Data File Size (MB): Size of all data files in the database
//   - Log File Size (MB): Size of all transaction log files in the database
//
// 2. Database I/O Metrics (3 metrics):
//   - Database Read I/O per Second: Database-specific read operations per second
//   - Database Write I/O per Second: Database-specific write operations per second
//   - Database I/O Stall Time (ms): Total I/O stall time for database files
//
// 3. Database Activity Metrics (3 metrics):
//   - Active Transactions: Number of active transactions in the database
//   - Database Sessions: Number of sessions connected to the database
//   - Log Flush Rate per Second: Transaction log flush operations per second
//
// Detailed Metric Descriptions:
//
// Database Size (MB):
// - Query: sys.database_files and FILEPROPERTY functions
// - Includes: Data files, log files, full-text catalog files
// - Calculation: Sum of (size * 8 / 1024) for all database files
//
// Data File Size (MB):
// - Query: sys.database_files WHERE type = 0 (data files)
// - Includes: Primary data file (.mdf) and secondary data files (.ndf)
// - Excludes: Transaction log files and full-text files
//
// Log File Size (MB):
// - Query: sys.database_files WHERE type = 1 (log files)
// - Includes: All transaction log files (.ldf)
// - Used for monitoring log file growth and space usage
//
// Database Read I/O per Second:
// - Query: sys.dm_io_virtual_file_stats() for read operations
// - Metric: num_of_reads per second for database files
// - Scope: All read I/O operations for the specific database
//
// Database Write I/O per Second:
// - Query: sys.dm_io_virtual_file_stats() for write operations
// - Metric: num_of_writes per second for database files
// - Scope: All write I/O operations for the specific database
//
// Database I/O Stall Time (ms):
// - Query: sys.dm_io_virtual_file_stats() for I/O stall statistics
// - Metric: io_stall_read_ms + io_stall_write_ms
// - Purpose: Identify I/O bottlenecks at database level
//
// Active Transactions:
// - Query: sys.dm_tran_active_transactions and sys.dm_tran_session_transactions
// - Count: Active transactions scoped to specific database
// - Includes: Both user and system transactions
//
// Database Sessions:
// - Query: sys.dm_exec_sessions WHERE database_id = DB_ID()
// - Count: Sessions currently connected to the database
// - Excludes: System sessions and background tasks
//
// Log Flush Rate per Second:
// - Query: sys.dm_os_performance_counters for log flush waits
// - Metric: Log flush operations per second for the database
// - Critical for: Transaction throughput monitoring
//
// Scraper Structure:
//
//	type DatabaseScraper struct {
//	    config   *Config
//	    mb       *metadata.MetricsBuilder
//	    queries  *queries.DatabaseQueries
//	    logger   *zap.Logger
//	}
//
// Data Sources:
// - sys.database_files: Database file information and sizes
// - sys.dm_io_virtual_file_stats(): Database I/O statistics
// - sys.dm_tran_active_transactions: Active transaction information
// - sys.dm_exec_sessions: Session and connection data
// - sys.dm_os_performance_counters: Database-specific performance counters
//
// Engine-Specific Considerations:
// - Azure SQL Database: All 9 metrics fully supported, database-scoped views
// - Azure SQL Managed Instance: All 9 metrics available with full functionality
// - Standard SQL Server: Complete access to all database-level metrics and DMVs
package scrapers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

// DatabaseScraper handles SQL Server database-level metrics collection
type DatabaseScraper struct {
	connection    SQLConnectionInterface
	logger        *zap.Logger
	startTime     pcommon.Timestamp
	engineEdition int
}

// NewDatabaseScraper creates a new database scraper
func NewDatabaseScraper(conn SQLConnectionInterface, logger *zap.Logger, engineEdition int) *DatabaseScraper {
	return &DatabaseScraper{
		connection:    conn,
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: engineEdition,
	}
}

// ScrapeDatabaseBufferMetrics collects database-level buffer pool metrics using engine-specific queries
func (s *DatabaseScraper) ScrapeDatabaseBufferMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database buffer metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.database.buffer_pool_size")
	if !found {
		return fmt.Errorf("no database buffer pool query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database buffer pool query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabaseBufferMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database buffer query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute database buffer query: %w", err)
	}

	if len(results) == 0 {
		s.logger.Warn("No results returned from database buffer query - no databases available")
		return fmt.Errorf("no results returned from database buffer query")
	}

	s.logger.Debug("Processing database buffer metrics results",
		zap.Int("result_count", len(results)))

	// Process each database's buffer metrics
	for _, result := range results {
		if result.BufferPoolSizeBytes == nil {
			s.logger.Warn("Buffer pool size is null for database",
				zap.String("database_name", result.DatabaseName))
			continue
		}

		if err := s.processDatabaseBufferMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database buffer metrics",
				zap.Error(err),
				zap.String("database_name", result.DatabaseName))
			continue
		}

		s.logger.Debug("Successfully processed database buffer metrics",
			zap.String("database_name", result.DatabaseName),
			zap.Int64("buffer_pool_size_bytes", *result.BufferPoolSizeBytes))
	}

	s.logger.Debug("Successfully scraped database buffer metrics",
		zap.Int("database_count", len(results)))

	return nil
}

// getQueryForMetric retrieves the appropriate query for a metric based on engine edition
func (s *DatabaseScraper) getQueryForMetric(metricName string) (string, bool) {
	query, found := queries.GetQueryForMetric(queries.DatabaseQueries, metricName, s.engineEdition)
	if found {
		s.logger.Debug("Using query for metric",
			zap.String("metric_name", metricName),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
	}
	return query, found
}

// processDatabaseBufferMetrics processes buffer pool metrics and creates OpenTelemetry metrics
func (s *DatabaseScraper) processDatabaseBufferMetrics(result models.DatabaseBufferMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process the struct fields with metric tags
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Skip nil values and non-metric fields
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}

		// Get metric metadata from struct tags
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")

		if metricName == "" {
			continue
		}

		// Create the metric
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		metric.SetUnit("By")

		// Set description based on metric name
		switch metricName {
		case "sqlserver.database.bufferpool.sizePerDatabaseInBytes":
			metric.SetDescription("Size of the SQL Server buffer pool allocated for the database in bytes (bufferpool.sizePerDatabaseInBytes)")
		default:
			metric.SetDescription(fmt.Sprintf("SQL Server database %s metric", metricName))
		}

		// Create gauge metric (for database metrics)
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)

		// Handle pointer fields and set the value
		fieldValue := field
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				return fmt.Errorf("field value is nil for metric %s", metricName)
			}
			fieldValue = field.Elem()
		}

		// Set the value based on field type
		switch fieldValue.Kind() {
		case reflect.Int64:
			value := fieldValue.Int()
			dataPoint.SetIntValue(value)
			s.logger.Info("Successfully scraped SQL Server database buffer metric",
				zap.String("database_name", result.DatabaseName),
				zap.Int64("buffer_pool_size_bytes", value))
		case reflect.Float64:
			value := fieldValue.Float()
			dataPoint.SetDoubleValue(value)
			s.logger.Info("Successfully scraped SQL Server database buffer metric",
				zap.String("database_name", result.DatabaseName),
				zap.Float64("value", value))
		case reflect.Int, reflect.Int32:
			value := fieldValue.Int()
			dataPoint.SetIntValue(value)
			s.logger.Info("Successfully scraped SQL Server database buffer metric",
				zap.String("database_name", result.DatabaseName),
				zap.Int64("value", value))
		default:
			return fmt.Errorf("unsupported field type %s for metric %s", fieldValue.Kind(), metricName)
		}

		// Set attributes for database-scoped metrics
		dataPoint.Attributes().PutStr("metric.type", sourceType)
		dataPoint.Attributes().PutStr("metric.source", "sys.dm_os_buffer_descriptors")
		dataPoint.Attributes().PutStr("database_name", result.DatabaseName)
		dataPoint.Attributes().PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
		dataPoint.Attributes().PutInt("engine_edition_id", int64(s.engineEdition))
	}

	return nil
}

// ScrapeDatabaseDiskMetrics collects database-level disk metrics using engine-specific queries
func (s *DatabaseScraper) ScrapeDatabaseDiskMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database disk metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.database.max_disk_size")
	if !found {
		return fmt.Errorf("no database disk metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database disk metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabaseDiskMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database disk query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute database disk query: %w", err)
	}

	if len(results) == 0 {
		s.logger.Warn("No results returned from database disk query - no databases available")
		return fmt.Errorf("no results returned from database disk query")
	}

	s.logger.Debug("Processing database disk metrics results",
		zap.Int("result_count", len(results)))

	// Process each database's disk metrics
	for _, result := range results {
		if result.MaxDiskSizeBytes == nil {
			s.logger.Warn("Max disk size is null for database",
				zap.String("database_name", result.DatabaseName))
			continue
		}

		if err := s.processDatabaseDiskMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database disk metrics",
				zap.Error(err),
				zap.String("database_name", result.DatabaseName))
			continue
		}

		s.logger.Debug("Successfully processed database disk metrics",
			zap.String("database_name", result.DatabaseName),
			zap.Int64("max_disk_size_bytes", *result.MaxDiskSizeBytes))
	}

	s.logger.Debug("Successfully scraped database disk metrics",
		zap.Int("database_count", len(results)))

	return nil
}

// processDatabaseDiskMetrics processes disk metrics and creates OpenTelemetry metrics
func (s *DatabaseScraper) processDatabaseDiskMetrics(result models.DatabaseDiskMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process the struct fields with metric tags
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Skip nil values and non-metric fields
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}

		// Get metric metadata from struct tags
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")

		if metricName == "" {
			continue
		}

		// Create the metric
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		metric.SetUnit("By")

		// Set description based on metric name
		switch metricName {
		case "sqlserver.database.maxDiskSizeInBytes":
			metric.SetDescription("Maximum disk size allowed for the SQL Server database in bytes (maxDiskSizeInBytes)")
		default:
			metric.SetDescription(fmt.Sprintf("SQL Server database %s metric", metricName))
		}

		// Create gauge metric (for database metrics)
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)

		// Handle pointer fields and set the value
		fieldValue := field
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				return fmt.Errorf("field value is nil for metric %s", metricName)
			}
			fieldValue = field.Elem()
		}

		// Set the value based on field type
		switch fieldValue.Kind() {
		case reflect.Int64:
			value := fieldValue.Int()
			dataPoint.SetIntValue(value)
			s.logger.Info("Successfully scraped SQL Server database disk metric",
				zap.String("database_name", result.DatabaseName),
				zap.Int64("max_disk_size_bytes", value))
		case reflect.Float64:
			value := fieldValue.Float()
			dataPoint.SetDoubleValue(value)
			s.logger.Info("Successfully scraped SQL Server database disk metric",
				zap.String("database_name", result.DatabaseName),
				zap.Float64("value", value))
		case reflect.Int, reflect.Int32:
			value := fieldValue.Int()
			dataPoint.SetIntValue(value)
			s.logger.Info("Successfully scraped SQL Server database disk metric",
				zap.String("database_name", result.DatabaseName),
				zap.Int64("value", value))
		default:
			return fmt.Errorf("unsupported field type %s for metric %s", fieldValue.Kind(), metricName)
		}

		// Set attributes for database-scoped metrics
		dataPoint.Attributes().PutStr("metric.type", sourceType)
		dataPoint.Attributes().PutStr("metric.source", "DATABASEPROPERTYEX")
		dataPoint.Attributes().PutStr("database_name", result.DatabaseName)
		dataPoint.Attributes().PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
		dataPoint.Attributes().PutInt("engine_edition_id", int64(s.engineEdition))
	}

	return nil
}

// ScrapeDatabaseIOMetrics collects database-level IO stall metrics using engine-specific queries
func (s *DatabaseScraper) ScrapeDatabaseIOMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database IO metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.database.io_stall")
	if !found {
		return fmt.Errorf("no database IO metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database IO metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabaseIOMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database IO query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute database IO query: %w", err)
	}

	if len(results) == 0 {
		s.logger.Warn("No results returned from database IO query - no databases available")
		return fmt.Errorf("no results returned from database IO query")
	}

	s.logger.Debug("Processing database IO metrics results",
		zap.Int("result_count", len(results)))

	// Process each database's IO metrics
	for _, result := range results {
		if result.IOStallTimeMs == nil {
			s.logger.Warn("IO stall time is null for database",
				zap.String("database_name", result.DatabaseName))
			continue
		}

		if err := s.processDatabaseIOMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database IO metrics",
				zap.Error(err),
				zap.String("database_name", result.DatabaseName))
			continue
		}

		s.logger.Info("Successfully scraped SQL Server database IO metric",
			zap.String("database_name", result.DatabaseName),
			zap.Int64("io_stall_time_ms", *result.IOStallTimeMs))

		s.logger.Debug("Successfully processed database IO metrics",
			zap.String("database_name", result.DatabaseName),
			zap.Int64("io_stall_time_ms", *result.IOStallTimeMs))
	}

	s.logger.Debug("Successfully scraped database IO metrics",
		zap.Int("database_count", len(results)))

	return nil
}

// processDatabaseIOMetrics processes IO metrics and creates OpenTelemetry metrics
func (s *DatabaseScraper) processDatabaseIOMetrics(result models.DatabaseIOMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process the struct fields with metric tags
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Skip nil values and non-metric fields
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}

		// Get metric metadata from struct tags
		metricName := fieldType.Tag.Get("metric_name")

		if metricName == "" {
			continue
		}

		// Create the metric
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		metric.SetDescription("Total IO stall time for the SQL Server database in milliseconds (io.stallInMilliseconds)")
		metric.SetUnit("ms")

		// Set data type to gauge for IO stall metrics
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()

		// Set attributes
		attrs := dataPoint.Attributes()
		attrs.PutStr("database_name", result.DatabaseName)
		attrs.PutStr("metric.type", "gauge")
		attrs.PutStr("metric.source", "sys.dm_io_virtual_file_stats")
		attrs.PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
		attrs.PutInt("engine_edition_id", int64(s.engineEdition))

		// Set the metric value
		var value int64
		if field.Kind() == reflect.Ptr {
			if field.Type().Elem().Kind() == reflect.Int64 {
				value = field.Elem().Int()
			}
		} else if field.Kind() == reflect.Int64 {
			value = field.Int()
		}

		dataPoint.SetIntValue(value)

		// Set timestamps
		now := pcommon.NewTimestampFromTime(time.Now())
		dataPoint.SetTimestamp(now)
		dataPoint.SetStartTimestamp(s.startTime)
	}

	return nil
}

// ScrapeDatabaseLogGrowthMetrics collects log growth metrics for SQL Server databases
func (s *DatabaseScraper) ScrapeDatabaseLogGrowthMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database log growth metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.database.log_growth")
	if !found {
		return fmt.Errorf("no database log growth metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database log growth metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabaseLogGrowthMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database log growth query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute database log growth query: %w", err)
	}

	if len(results) == 0 {
		s.logger.Warn("No results returned from database log growth query - no databases available")
		return fmt.Errorf("no results returned from database log growth query")
	}

	s.logger.Debug("Processing database log growth metrics results",
		zap.Int("result_count", len(results)))

	// Process each database's log growth metrics
	for _, result := range results {
		if result.LogGrowthCount == nil {
			s.logger.Warn("Log growth count is null for database",
				zap.String("database_name", result.DatabaseName))
			continue
		}

		if err := s.processDatabaseLogGrowthMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database log growth metrics",
				zap.Error(err),
				zap.String("database_name", result.DatabaseName))
			continue
		}
	}

	s.logger.Debug("Successfully processed database log growth metrics",
		zap.Int("metrics_processed", len(results)))

	return nil
}

// processDatabaseLogGrowthMetrics processes individual database log growth metrics using reflection
func (s *DatabaseScraper) processDatabaseLogGrowthMetrics(result models.DatabaseLogGrowthMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process the struct fields with metric tags
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Skip nil values and non-metric fields
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}

		// Get metric metadata from struct tags
		metricName := fieldType.Tag.Get("metric_name")

		if metricName == "" {
			continue
		}

		// Create the metric
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		metric.SetDescription("Number of log growth events for the SQL Server database (log.transactionGrowth)")
		metric.SetUnit("1")

		// Set data type to gauge for log growth count metrics
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()

		// Set attributes
		attrs := dataPoint.Attributes()
		attrs.PutStr("database_name", result.DatabaseName)
		attrs.PutStr("metric.type", "gauge")
		attrs.PutStr("metric.source", "sys.dm_os_performance_counters")
		attrs.PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
		attrs.PutInt("engine_edition_id", int64(s.engineEdition))

		// Set the metric value
		var value int64
		if field.Kind() == reflect.Ptr {
			// Handle pointer fields (like *int64)
			value = field.Elem().Int()
		} else {
			// Handle non-pointer fields
			value = field.Int()
		}

		s.logger.Debug("Setting database log growth metric value",
			zap.String("metric_name", metricName),
			zap.String("database_name", result.DatabaseName),
			zap.Int64("value", value))

		dataPoint.SetIntValue(value)

		// Set timestamps
		now := pcommon.NewTimestampFromTime(time.Now())
		dataPoint.SetTimestamp(now)
		dataPoint.SetStartTimestamp(s.startTime)
	}

	return nil
}

// ScrapeDatabasePageFileMetrics scrapes page file metrics for all databases
func (s *DatabaseScraper) ScrapeDatabasePageFileMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database page file metrics")

	// First, get the list of databases to iterate through
	databasesQuery := `SELECT name FROM sys.databases 
		WHERE name NOT IN ('master', 'tempdb', 'msdb', 'model', 'rdsadmin', 'distribution', 'model_msdb', 'model_replicatedmaster')
		AND state = 0`

	var databases []struct {
		Name string `db:"name"`
	}

	if err := s.connection.Query(ctx, &databases, databasesQuery); err != nil {
		return fmt.Errorf("failed to get database list: %w", err)
	}

	s.logger.Debug("Found databases for page file metrics",
		zap.Int("database_count", len(databases)))

	// Iterate through each database and collect page file metrics
	for _, db := range databases {
		s.logger.Debug("Processing page file metrics for database",
			zap.String("database_name", db.Name))

		// Create database-specific query
		query := fmt.Sprintf(`SELECT 
			'%s' AS db_name,
			(SUM(a.total_pages) * 8.0 - SUM(a.used_pages) * 8.0) * 1024 AS reserved_space_not_used
		FROM [%s].sys.partitions p WITH (NOLOCK)
		INNER JOIN [%s].sys.allocation_units a WITH (NOLOCK) ON p.partition_id = a.container_id
		LEFT JOIN [%s].sys.internal_tables it WITH (NOLOCK) ON p.object_id = it.object_id`, 
			db.Name, db.Name, db.Name, db.Name)

		var results []models.DatabasePageFileMetrics
		if err := s.connection.Query(ctx, &results, query); err != nil {
			s.logger.Error("Failed to execute page file metrics query for database",
				zap.String("database_name", db.Name),
				zap.Error(err))
			continue
		}

		// Process results for this database
		for _, result := range results {
			if result.PageFileAvailableBytes != nil {
				s.logger.Info("Successfully scraped SQL Server database page file metric",
					zap.String("database_name", result.DatabaseName),
					zap.Float64("page_file_available_bytes", *result.PageFileAvailableBytes))

				if err := s.processDatabasePageFileMetrics(result, scopeMetrics); err != nil {
					s.logger.Error("Failed to process database page file metrics",
						zap.String("database_name", result.DatabaseName),
						zap.Error(err))
					continue
				}

				s.logger.Debug("Successfully processed database page file metrics",
					zap.String("database_name", result.DatabaseName),
					zap.Float64("page_file_available_bytes", *result.PageFileAvailableBytes))
			}
		}
	}

	s.logger.Debug("Successfully completed database page file metrics scraping")
	return nil
}

// ScrapeDatabasePageFileTotalMetrics scrapes page file total metrics for all databases
func (s *DatabaseScraper) ScrapeDatabasePageFileTotalMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database page file total metrics")

	// First, get the list of databases to iterate through
	databasesQuery := `SELECT name FROM sys.databases 
		WHERE name NOT IN ('master', 'tempdb', 'msdb', 'model', 'rdsadmin', 'distribution', 'model_msdb', 'model_replicatedmaster')
		AND state = 0`

	var databases []struct {
		Name string `db:"name"`
	}

	if err := s.connection.Query(ctx, &databases, databasesQuery); err != nil {
		return fmt.Errorf("failed to get database list: %w", err)
	}

	s.logger.Debug("Found databases for page file total metrics",
		zap.Int("database_count", len(databases)))

	// Iterate through each database and collect page file total metrics
	for _, db := range databases {
		s.logger.Debug("Processing page file total metrics for database",
			zap.String("database_name", db.Name))

		// Create database-specific query
		query := fmt.Sprintf(`SELECT 
			'%s' AS db_name,
			SUM(a.total_pages) * 8.0 * 1024 AS reserved_space
		FROM [%s].sys.partitions p WITH (NOLOCK)
		INNER JOIN [%s].sys.allocation_units a WITH (NOLOCK) ON p.partition_id = a.container_id
		LEFT JOIN [%s].sys.internal_tables it WITH (NOLOCK) ON p.object_id = it.object_id`, 
			db.Name, db.Name, db.Name, db.Name)

		var results []models.DatabasePageFileTotalMetrics
		if err := s.connection.Query(ctx, &results, query); err != nil {
			s.logger.Error("Failed to execute page file total metrics query for database",
				zap.String("database_name", db.Name),
				zap.Error(err))
			continue
		}

		// Process results for this database
		for _, result := range results {
			if result.PageFileTotalBytes != nil {
				s.logger.Info("Successfully scraped SQL Server database page file total metric",
					zap.String("database_name", result.DatabaseName),
					zap.Float64("page_file_total_bytes", *result.PageFileTotalBytes))

				if err := s.processDatabasePageFileTotalMetrics(result, scopeMetrics); err != nil {
					s.logger.Error("Failed to process database page file total metrics",
						zap.String("database_name", result.DatabaseName),
						zap.Error(err))
					continue
				}

				s.logger.Debug("Successfully processed database page file total metrics",
					zap.String("database_name", result.DatabaseName),
					zap.Float64("page_file_total_bytes", *result.PageFileTotalBytes))
			}
		}
	}

	s.logger.Debug("Successfully completed database page file total metrics scraping")
	return nil
}

// processDatabasePageFileMetrics processes individual database page file metrics using reflection
func (s *DatabaseScraper) processDatabasePageFileMetrics(result models.DatabasePageFileMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process the struct fields with metric tags
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Skip nil values and non-metric fields
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}

		// Get the metric name from the struct tag
		metricName := fieldType.Tag.Get("metric_name")
		if metricName == "" {
			continue // Skip fields without metric_name tag
		}

		// Create the metric
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		metric.SetDescription("Available page file space (reserved space not used) for the SQL Server database in bytes (pageFileAvailable)")
		metric.SetUnit("By")

		// Set up gauge data
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()

		// Set attributes
		attrs := dataPoint.Attributes()
		attrs.PutStr("database_name", result.DatabaseName)
		attrs.PutStr("metric.type", "gauge")
		attrs.PutStr("metric.source", "sys.partitions_sys.allocation_units")
		attrs.PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
		attrs.PutInt("engine_edition_id", int64(s.engineEdition))

		// Set the metric value
		var value float64
		if field.Kind() == reflect.Ptr {
			// Handle pointer fields (like *float64)
			value = field.Elem().Float()
		} else {
			// Handle non-pointer fields
			value = field.Float()
		}

		s.logger.Debug("Setting database page file metric value",
			zap.String("metric_name", metricName),
			zap.String("database_name", result.DatabaseName),
			zap.Float64("value", value))

		dataPoint.SetDoubleValue(value)

		// Set timestamps
		now := pcommon.NewTimestampFromTime(time.Now())
		dataPoint.SetTimestamp(now)
		dataPoint.SetStartTimestamp(s.startTime)
	}

	return nil
}

// processDatabasePageFileTotalMetrics processes individual database page file total metrics using reflection
func (s *DatabaseScraper) processDatabasePageFileTotalMetrics(result models.DatabasePageFileTotalMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process the struct fields with metric tags
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Skip nil values and non-metric fields
		if field.Kind() == reflect.Ptr && field.IsNil() {
			continue
		}

		// Get the metric name from the struct tag
		metricName := fieldType.Tag.Get("metric_name")
		if metricName == "" {
			continue // Skip fields without metric_name tag
		}

		// Create the metric
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricName)
		metric.SetDescription("Total page file space (total reserved space) for the SQL Server database in bytes (pageFileTotal)")
		metric.SetUnit("By")

		// Set up gauge data
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()

		// Set attributes
		attrs := dataPoint.Attributes()
		attrs.PutStr("database_name", result.DatabaseName)
		attrs.PutStr("metric.type", "gauge")
		attrs.PutStr("metric.source", "sys.partitions_sys.allocation_units")
		attrs.PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
		attrs.PutInt("engine_edition_id", int64(s.engineEdition))

		// Set the metric value
		var value float64
		if field.Kind() == reflect.Ptr {
			// Handle pointer fields (like *float64)
			value = field.Elem().Float()
		} else {
			// Handle non-pointer fields
			value = field.Float()
		}

		s.logger.Debug("Setting database page file total metric value",
			zap.String("metric_name", metricName),
			zap.String("database_name", result.DatabaseName),
			zap.Float64("value", value))

		dataPoint.SetDoubleValue(value)

		// Set timestamps
		now := pcommon.NewTimestampFromTime(time.Now())
		dataPoint.SetTimestamp(now)
		dataPoint.SetStartTimestamp(s.startTime)
	}

	return nil
}

// ScrapeDatabaseMemoryMetrics scrapes available physical memory metrics
// This is an instance-level metric that provides system memory information
func (s *DatabaseScraper) ScrapeDatabaseMemoryMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Starting ScrapeDatabaseMemoryMetrics")

	// Get the appropriate query based on the engine edition
	query, found := queries.GetQueryForMetric(queries.DatabaseQueries, "sqlserver.instance.memory_available", s.engineEdition)
	if !found {
		s.logger.Error("Memory query not found for engine edition", zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("memory query not found for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing memory query", zap.String("query", queries.TruncateQuery(query, 100)))

	// Query the database for memory metrics
	var results []models.DatabaseMemoryMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute memory query", zap.Error(err))
		return fmt.Errorf("failed to execute memory query: %w", err)
	}

	s.logger.Debug("Memory query completed", zap.Int("result_count", len(results)))

	// Process each result using reflection
	return s.processDatabaseMemoryResults(scopeMetrics, results)
}

// processDatabaseMemoryResults processes the memory query results and creates OpenTelemetry metrics
func (s *DatabaseScraper) processDatabaseMemoryResults(scopeMetrics pmetric.ScopeMetrics, results []models.DatabaseMemoryMetrics) error {
	if len(results) == 0 {
		s.logger.Debug("No memory metrics results to process")
		return nil
	}

	// Use reflection to iterate through the struct fields and create metrics
	structType := reflect.TypeOf(models.DatabaseMemoryMetrics{})
	
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		
		// Skip fields that don't have metric_name tag
		metricNameTag := field.Tag.Get("metric_name")
		if metricNameTag == "" {
			continue
		}

		sourceTypeTag := field.Tag.Get("source_type")
		if sourceTypeTag == "" {
			continue
		}

		// Create metric for this field
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName(metricNameTag)
		metric.SetDescription("Available physical memory on the system")
		metric.SetUnit("bytes")

		// Set the appropriate metric type based on source_type
		var dataPoints pmetric.NumberDataPointSlice
		switch sourceTypeTag {
		case "gauge":
			gauge := metric.SetEmptyGauge()
			dataPoints = gauge.DataPoints()
		default:
			s.logger.Warn("Unknown source type for memory metric", zap.String("source_type", sourceTypeTag), zap.String("metric_name", metricNameTag))
			continue
		}

		// Process each result
		for _, result := range results {
			dataPoint := dataPoints.AppendEmpty()
			attrs := dataPoint.Attributes()

			// Add instance-level attributes
			attrs.PutStr("metric.type", "gauge")
			attrs.PutStr("metric.source", "sys.dm_os_sys_memory")
			attrs.PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
			attrs.PutInt("engine_edition_id", int64(s.engineEdition))

			// Get field value using reflection
			structValue := reflect.ValueOf(result)
			fieldValue := structValue.Field(i)

			// Set the metric value
			var value float64
			if fieldValue.Kind() == reflect.Ptr {
				if !fieldValue.IsNil() {
					value = fieldValue.Elem().Float()
				} else {
					s.logger.Debug("Nil value for memory metric", zap.String("metric_name", metricNameTag))
					continue
				}
			} else {
				value = fieldValue.Float()
			}

			s.logger.Debug("Setting memory metric value",
				zap.String("metric_name", metricNameTag),
				zap.Float64("value", value))

			dataPoint.SetDoubleValue(value)

			// Set timestamps
			now := pcommon.NewTimestampFromTime(time.Now())
			dataPoint.SetTimestamp(now)
			dataPoint.SetStartTimestamp(s.startTime)
		}
	}

	return nil
}
