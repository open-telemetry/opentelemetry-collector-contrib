// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package scrapers provides the database principals metrics scraper for SQL Server.
// This file implements comprehensive collection of database security principals metrics
// including users, roles, and security-related statistics.
//
// Database Principals Metrics Overview:
//
// Database principals represent security entities within a SQL Server database that
// can be granted permissions to access database objects. This scraper collects:
//
// 1. Individual Principal Metrics:
//   - Principal Name: The name of each user, role, or security entity
//   - Principal Type: The type of principal (SQL_USER, WINDOWS_USER, DATABASE_ROLE, etc.)
//   - Creation Date: When each principal was created for lifecycle tracking
//
// 2. Aggregate Principal Statistics:
//   - Total Principals: Count of all non-system principals in the database
//   - User Count: Count of all database users (various authentication types)
//   - Role Count: Count of custom database roles and application roles
//   - Type-Specific Counts: Detailed breakdown by authentication type
//
// 3. Principal Activity Metrics:
//   - Recently Created: Principals created in the last 30 days
//   - Old Principals: Principals created more than 1 year ago
//   - Orphaned Users: SQL users without corresponding server logins
//
// Security and Compliance Value:
// - Audit database access control configuration
// - Monitor user and role lifecycle changes
// - Identify potential security risks (orphaned accounts)
// - Support compliance reporting for database access
// - Track application-specific role usage
//
// Query Optimization:
// - Excludes system fixed roles and internal principals
// - Uses efficient sys.database_principals catalog view
// - Minimizes data transfer with targeted column selection
// - Supports batching for multi-database environments
//
// Engine Compatibility:
// - Standard SQL Server: Full access to all principal types and metadata
// - Azure SQL Database: Database-scoped principals with full feature support
// - Azure SQL Managed Instance: Complete functionality with enterprise features
//
// Based on User Requirements:
// This implementation extends the provided query for comprehensive monitoring:
// "SELECT name AS principal_name, type_desc, create_date FROM sys.database_principals
//
//	WHERE is_fixed_role = 0 AND type <> 'X' ORDER BY type_desc, principal_name"
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

// DatabasePrincipalsScraper handles SQL Server database principals metrics collection
// This scraper provides comprehensive monitoring of database security principals
type DatabasePrincipalsScraper struct {
	connection    SQLConnectionInterface
	logger        *zap.Logger
	startTime     pcommon.Timestamp
	engineEdition int
}

// NewDatabasePrincipalsScraper creates a new database principals scraper
func NewDatabasePrincipalsScraper(conn SQLConnectionInterface, logger *zap.Logger, engineEdition int) *DatabasePrincipalsScraper {
	return &DatabasePrincipalsScraper{
		connection:    conn,
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: engineEdition,
	}
}

// ScrapeDatabasePrincipalsMetrics collects individual database principals metrics using engine-specific queries
func (s *DatabasePrincipalsScraper) ScrapeDatabasePrincipalsMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database principals metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("database_principals")
	if !found {
		return fmt.Errorf("no database principals query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database principals query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabasePrincipalsMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database principals query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
		return fmt.Errorf("failed to query database principals: %w", err)
	}

	s.logger.Debug("Database principals query completed",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each principal result
	for _, result := range results {
		if err := s.processDatabasePrincipalsMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database principals metrics",
				zap.Error(err),
				zap.String("principal_name", result.PrincipalName),
				zap.String("database_name", result.DatabaseName))
			return err
		}
	}

	return nil
}

// ScrapeDatabasePrincipalsSummaryMetrics collects aggregated database principals statistics
func (s *DatabasePrincipalsScraper) ScrapeDatabasePrincipalsSummaryMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database principals summary metrics")

	// Get the appropriate summary query for this engine edition
	query, found := s.getQueryForMetric("database_principals_summary")
	if !found {
		return fmt.Errorf("no database principals summary query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database principals summary query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabasePrincipalsSummary
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database principals summary query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
		return fmt.Errorf("failed to query database principals summary: %w", err)
	}

	s.logger.Debug("Database principals summary query completed",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each summary result
	for _, result := range results {
		if err := s.processDatabasePrincipalsSummaryMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database principals summary metrics",
				zap.Error(err),
				zap.String("database_name", result.DatabaseName))
			return err
		}
	}

	return nil
}

// ScrapeDatabasePrincipalActivityMetrics collects database principals activity and lifecycle metrics
func (s *DatabasePrincipalsScraper) ScrapeDatabasePrincipalActivityMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server database principals activity metrics")

	// Get the appropriate activity query for this engine edition
	query, found := s.getQueryForMetric("database_principals_activity")
	if !found {
		return fmt.Errorf("no database principals activity query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing database principals activity query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.DatabasePrincipalActivity
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute database principals activity query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))
		return fmt.Errorf("failed to query database principals activity: %w", err)
	}

	s.logger.Debug("Database principals activity query completed",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each activity result
	for _, result := range results {
		if err := s.processDatabasePrincipalActivityMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process database principals activity metrics",
				zap.Error(err),
				zap.String("database_name", result.DatabaseName))
			return err
		}
	}

	return nil
}

// getQueryForMetric retrieves the appropriate query for a metric based on engine edition
func (s *DatabasePrincipalsScraper) getQueryForMetric(metricName string) (string, bool) {
	queryMap := map[string]map[int]string{
		"database_principals": {
			queries.StandardSQLServerEngineEdition:       queries.DatabasePrincipalsQuery,
			queries.AzureSQLDatabaseEngineEdition:        queries.DatabasePrincipalsQueryAzureSQL,
			queries.AzureSQLManagedInstanceEngineEdition: queries.DatabasePrincipalsQueryAzureMI,
			// Add support for other SQL Server editions (Enterprise, Standard, etc.)
			1: queries.DatabasePrincipalsQuery, // Personal/Desktop
			2: queries.DatabasePrincipalsQuery, // Standard
			3: queries.DatabasePrincipalsQuery, // Enterprise
			4: queries.DatabasePrincipalsQuery, // Express
			6: queries.DatabasePrincipalsQuery, // Azure Synapse Analytics
		},
		"database_principals_summary": {
			queries.StandardSQLServerEngineEdition:       queries.DatabasePrincipalsSummaryQuery,
			queries.AzureSQLDatabaseEngineEdition:        queries.DatabasePrincipalsSummaryQueryAzureSQL,
			queries.AzureSQLManagedInstanceEngineEdition: queries.DatabasePrincipalsSummaryQueryAzureMI,
			// Add support for other SQL Server editions
			1: queries.DatabasePrincipalsSummaryQuery, // Personal/Desktop
			2: queries.DatabasePrincipalsSummaryQuery, // Standard
			3: queries.DatabasePrincipalsSummaryQuery, // Enterprise
			4: queries.DatabasePrincipalsSummaryQuery, // Express
			6: queries.DatabasePrincipalsSummaryQuery, // Azure Synapse Analytics
		},
		"database_principals_activity": {
			queries.StandardSQLServerEngineEdition:       queries.DatabasePrincipalActivityQuery,
			queries.AzureSQLDatabaseEngineEdition:        queries.DatabasePrincipalActivityQueryAzureSQL,
			queries.AzureSQLManagedInstanceEngineEdition: queries.DatabasePrincipalActivityQueryAzureMI,
			// Add support for other SQL Server editions
			1: queries.DatabasePrincipalActivityQuery, // Personal/Desktop
			2: queries.DatabasePrincipalActivityQuery, // Standard
			3: queries.DatabasePrincipalActivityQuery, // Enterprise
			4: queries.DatabasePrincipalActivityQuery, // Express
			6: queries.DatabasePrincipalActivityQuery, // Azure Synapse Analytics
		},
	}

	if engineQueries, exists := queryMap[metricName]; exists {
		if query, found := engineQueries[s.engineEdition]; found {
			return query, true
		}
	}

	return "", false
}

// processDatabasePrincipalsMetrics processes individual database principals metrics and creates OpenTelemetry metrics
func (s *DatabasePrincipalsScraper) processDatabasePrincipalsMetrics(result models.DatabasePrincipalsMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process metrics following the existing pattern
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Get metric metadata from struct tags
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")

		if metricName == "" || sourceType == "attribute" {
			continue // Skip fields without metric names or attribute fields
		}

		// Create metric based on field type and value
		if err := s.createMetricFromField(field, fieldType, metricName, result.DatabaseName, result.PrincipalName, result.TypeDesc, scopeMetrics); err != nil {
			return fmt.Errorf("failed to create metric %s: %w", metricName, err)
		}
	}

	return nil
}

// processDatabasePrincipalsSummaryMetrics processes summary metrics and creates OpenTelemetry metrics
func (s *DatabasePrincipalsScraper) processDatabasePrincipalsSummaryMetrics(result models.DatabasePrincipalsSummary, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process summary metrics
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Get metric metadata from struct tags
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")

		if metricName == "" || sourceType != "gauge" {
			continue // Skip non-metric fields
		}

		// Create gauge metric
		if err := s.createGaugeMetric(field, metricName, result.DatabaseName, scopeMetrics); err != nil {
			return fmt.Errorf("failed to create summary metric %s: %w", metricName, err)
		}
	}

	return nil
}

// processDatabasePrincipalActivityMetrics processes activity metrics and creates OpenTelemetry metrics
func (s *DatabasePrincipalsScraper) processDatabasePrincipalActivityMetrics(result models.DatabasePrincipalActivity, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process activity metrics
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Get metric metadata from struct tags
		metricName := fieldType.Tag.Get("metric_name")
		sourceType := fieldType.Tag.Get("source_type")

		if metricName == "" || sourceType != "gauge" {
			continue // Skip non-metric fields
		}

		// Create gauge metric
		if err := s.createGaugeMetric(field, metricName, result.DatabaseName, scopeMetrics); err != nil {
			return fmt.Errorf("failed to create activity metric %s: %w", metricName, err)
		}
	}

	return nil
}

// createMetricFromField creates an OpenTelemetry metric from a struct field
func (s *DatabasePrincipalsScraper) createMetricFromField(field reflect.Value, fieldType reflect.StructField, metricName, databaseName, principalName, principalType string, scopeMetrics pmetric.ScopeMetrics) error {
	// Handle different field types
	switch field.Kind() {
	case reflect.Ptr:
		if field.IsNil() {
			return nil // Skip nil pointer values
		}

		// Handle time.Time pointer for creation date
		if field.Type().Elem().String() == "time.Time" {
			timeValue := field.Elem().Interface().(time.Time)
			return s.createTimestampMetric(timeValue, metricName, databaseName, principalName, principalType, scopeMetrics)
		}

	case reflect.String:
		// Strings are handled as attributes, not metrics
		return nil
	}

	return fmt.Errorf("unsupported field type for metric %s: %s", metricName, field.Type())
}

// createGaugeMetric creates a gauge metric from an int64 pointer field
func (s *DatabasePrincipalsScraper) createGaugeMetric(field reflect.Value, metricName, databaseName string, scopeMetrics pmetric.ScopeMetrics) error {
	if field.Kind() != reflect.Ptr || field.IsNil() {
		return nil // Skip nil values
	}

	if field.Type().Elem().Kind() != reflect.Int64 {
		return fmt.Errorf("expected *int64 for gauge metric %s, got %s", metricName, field.Type())
	}

	value := field.Elem().Int()

	// Create gauge metric
	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(metricName)
	metric.SetDescription(fmt.Sprintf("Database principals metric: %s", metricName))

	gauge := metric.SetEmptyGauge()
	dataPoint := gauge.DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(s.startTime)
	dataPoint.SetIntValue(value)

	// Add database name as attribute
	dataPoint.Attributes().PutStr("database_name", databaseName)

	return nil
}

// createTimestampMetric creates a timestamp-based metric from a time.Time value
func (s *DatabasePrincipalsScraper) createTimestampMetric(timeValue time.Time, metricName, databaseName, principalName, principalType string, scopeMetrics pmetric.ScopeMetrics) error {
	// Convert time to Unix timestamp
	timestamp := timeValue.Unix()

	// Create gauge metric with timestamp as value
	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName(metricName)
	metric.SetDescription(fmt.Sprintf("Database principal creation date: %s", metricName))

	gauge := metric.SetEmptyGauge()
	dataPoint := gauge.DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(s.startTime)
	dataPoint.SetIntValue(timestamp)

	// Add attributes
	dataPoint.Attributes().PutStr("database_name", databaseName)
	dataPoint.Attributes().PutStr("principal_name", principalName)
	dataPoint.Attributes().PutStr("principal_type", principalType)

	return nil
}
