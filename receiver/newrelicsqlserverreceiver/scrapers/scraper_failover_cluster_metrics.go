// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package scrapers provides the failover cluster-level metrics scraper for SQL Server.
// This file implements collection of SQL Server Always On Availability Group replica
// performance metrics for high availability failover cluster deployments.
//
// Failover Cluster-Level Metrics:
//
// 1. Database Replica Performance Metrics:
//   - Log Bytes Received/sec: Rate of log records received by secondary replica from primary
//   - Transaction Delay: Average delay for transactions on the secondary replica
//   - Flow Control Time (ms/sec): Time spent in flow control by log records from primary
//
// Detailed Metric Descriptions:
//
// Log Bytes Received/sec:
// - Source: sys.dm_os_performance_counters for Database Replica counters
// - Purpose: Measures replication throughput from primary to secondary replica
// - Unit: Bytes per second
// - Critical for: Monitoring replication performance and network throughput
//
// Transaction Delay:
// - Source: sys.dm_os_performance_counters for Database Replica counters
// - Purpose: Indicates lag in transaction processing on secondary replica
// - Unit: Milliseconds
// - Critical for: Identifying replication delays and performance bottlenecks
//
// Flow Control Time (ms/sec):
// - Source: sys.dm_os_performance_counters for Database Replica counters
// - Purpose: Measures time spent waiting for flow control from primary replica
// - Unit: Milliseconds per second
// - Critical for: Understanding log send/receive throttling behavior
//
// Scraper Structure:
//
//	type FailoverClusterScraper struct {
//	    connection    SQLConnectionInterface
//	    logger        *zap.Logger
//	    startTime     pcommon.Timestamp
//	    engineEdition int
//	}
//
// Data Sources:
// - sys.dm_os_performance_counters: Always On Database Replica performance counters
//
// Engine-Specific Considerations:
// - Standard SQL Server: Full failover cluster metrics support for Always On AG
// - Azure SQL Database: Not applicable (no Always On AG support)
// - Azure SQL Managed Instance: Limited support (managed HA service)
//
// Availability:
// - Only available on SQL Server instances with Always On Availability Groups enabled
// - Returns empty result set on instances without Always On AG configuration
// - Requires appropriate permissions to query performance counter DMVs
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

// FailoverClusterScraper handles SQL Server Always On Availability Group replica metrics collection
type FailoverClusterScraper struct {
	connection    SQLConnectionInterface
	logger        *zap.Logger
	startTime     pcommon.Timestamp
	engineEdition int
}

// NewFailoverClusterScraper creates a new failover cluster scraper
func NewFailoverClusterScraper(conn SQLConnectionInterface, logger *zap.Logger, engineEdition int) *FailoverClusterScraper {
	return &FailoverClusterScraper{
		connection:    conn,
		logger:        logger,
		startTime:     pcommon.NewTimestampFromTime(time.Now()),
		engineEdition: engineEdition,
	}
}

// ScrapeFailoverClusterMetrics collects Always On Availability Group replica performance metrics
// This method is only applicable to SQL Server deployments with Always On AG enabled
func (s *FailoverClusterScraper) ScrapeFailoverClusterMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server Always On failover cluster replica metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.failover_cluster.replica_metrics")
	if !found {
		return fmt.Errorf("no failover cluster replica metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing failover cluster replica metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.FailoverClusterReplicaMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute failover cluster replica query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute failover cluster replica query: %w", err)
	}

	// If no results, this SQL Server instance does not have Always On AG enabled or configured
	if len(results) == 0 {
		s.logger.Debug("No Always On replica metrics found - SQL Server may not have Always On Availability Groups enabled")
		return nil
	}

	s.logger.Debug("Processing failover cluster replica metrics results",
		zap.Int("result_count", len(results)))

	// Process each replica's metrics
	for _, result := range results {
		if err := s.processFailoverClusterReplicaMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process failover cluster replica metrics",
				zap.Error(err))
			continue
		}

		s.logger.Info("Successfully scraped SQL Server Always On replica metrics",
			zap.Int64p("log_bytes_received_per_sec", result.LogBytesReceivedPerSec),
			zap.Int64p("transaction_delay_ms", result.TransactionDelayMs),
			zap.Int64p("flow_control_time_ms", result.FlowControlTimeMs))
	}

	s.logger.Debug("Successfully scraped failover cluster replica metrics",
		zap.Int("result_count", len(results)))

	return nil
}

// ScrapeFailoverClusterReplicaStateMetrics collects Always On Availability Group database replica state metrics
// This method provides detailed log synchronization metrics for each database in the availability group
func (s *FailoverClusterScraper) ScrapeFailoverClusterReplicaStateMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server Always On failover cluster replica state metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.failover_cluster.replica_state_metrics")
	if !found {
		return fmt.Errorf("no failover cluster replica state metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing failover cluster replica state metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.FailoverClusterReplicaStateMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute failover cluster replica state query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute failover cluster replica state query: %w", err)
	}

	// If no results, this SQL Server instance does not have Always On AG enabled or configured
	if len(results) == 0 {
		s.logger.Debug("No Always On replica state metrics found - SQL Server may not have Always On Availability Groups enabled")
		return nil
	}

	s.logger.Debug("Processing failover cluster replica state metrics results",
		zap.Int("result_count", len(results)))

	// Process each replica state's metrics
	for _, result := range results {
		if err := s.processFailoverClusterReplicaStateMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process failover cluster replica state metrics",
				zap.Error(err))
			continue
		}

		s.logger.Info("Successfully scraped SQL Server Always On replica state metrics",
			zap.String("replica_server_name", result.ReplicaServerName),
			zap.String("database_name", result.DatabaseName),
			zap.Int64p("log_send_queue_kb", result.LogSendQueueKB),
			zap.Int64p("redo_queue_kb", result.RedoQueueKB),
			zap.Int64p("redo_rate_kb_sec", result.RedoRateKBSec))
	}

	s.logger.Debug("Successfully scraped failover cluster replica state metrics",
		zap.Int("result_count", len(results)))

	return nil
}

// getQueryForMetric retrieves the appropriate query for a metric based on engine edition
func (s *FailoverClusterScraper) getQueryForMetric(metricName string) (string, bool) {
	// For now, use direct query constants since we need to support replica state metrics
	switch metricName {
	case "sqlserver.failover_cluster.replica_metrics":
		switch s.engineEdition {
		case queries.AzureSQLDatabaseEngineEdition:
			return queries.FailoverClusterReplicaQueryAzureSQL, true
		case queries.AzureSQLManagedInstanceEngineEdition:
			return queries.FailoverClusterReplicaQueryAzureMI, true
		default:
			return queries.FailoverClusterReplicaQuery, true
		}
	case "sqlserver.failover_cluster.replica_state_metrics":
		switch s.engineEdition {
		case queries.AzureSQLDatabaseEngineEdition:
			return queries.FailoverClusterReplicaStateQueryAzureSQL, true
		case queries.AzureSQLManagedInstanceEngineEdition:
			return queries.FailoverClusterReplicaStateQueryAzureMI, true
		default:
			return queries.FailoverClusterReplicaStateQuery, true
		}
	default:
		return "", false
	}
}

// processFailoverClusterReplicaMetrics processes replica metrics and creates OpenTelemetry metrics
func (s *FailoverClusterScraper) processFailoverClusterReplicaMetrics(result models.FailoverClusterReplicaMetrics, scopeMetrics pmetric.ScopeMetrics) error {
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
		metric.SetUnit(s.getMetricUnit(metricName))
		metric.SetDescription(s.getMetricDescription(metricName))

		// Create gauge metric for failover cluster metrics
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)

		// Set the value
		var value int64
		if field.Kind() == reflect.Ptr {
			if field.Type().Elem().Kind() == reflect.Int64 {
				value = field.Elem().Int()
			}
		} else if field.Kind() == reflect.Int64 {
			value = field.Int()
		}

		dataPoint.SetIntValue(value)

		// Set attributes for failover cluster metrics
		dataPoint.Attributes().PutStr("metric.type", sourceType)
		dataPoint.Attributes().PutStr("metric.source", "sys.dm_os_performance_counters")
		dataPoint.Attributes().PutStr("metric.category", "always_on_availability_group")
		dataPoint.Attributes().PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
		dataPoint.Attributes().PutInt("engine_edition_id", int64(s.engineEdition))
	}

	return nil
}

// processFailoverClusterReplicaStateMetrics processes replica state metrics and creates OpenTelemetry metrics
func (s *FailoverClusterScraper) processFailoverClusterReplicaStateMetrics(result models.FailoverClusterReplicaStateMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Use reflection to process the struct fields with metric tags
	resultValue := reflect.ValueOf(result)
	resultType := reflect.TypeOf(result)

	for i := 0; i < resultValue.NumField(); i++ {
		field := resultValue.Field(i)
		fieldType := resultType.Field(i)

		// Skip non-metric fields (string fields like replica_server_name, database_name)
		if field.Kind() == reflect.String {
			continue
		}

		// Skip nil values
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
		metric.SetUnit(s.getMetricUnit(metricName))
		metric.SetDescription(s.getMetricDescription(metricName))

		// Create gauge metric for failover cluster metrics
		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)

		// Set the value
		var value int64
		if field.Kind() == reflect.Ptr {
			if field.Type().Elem().Kind() == reflect.Int64 {
				value = field.Elem().Int()
			}
		} else if field.Kind() == reflect.Int64 {
			value = field.Int()
		}

		dataPoint.SetIntValue(value)

		// Set attributes for failover cluster replica state metrics
		dataPoint.Attributes().PutStr("replica_server_name", result.ReplicaServerName)
		dataPoint.Attributes().PutStr("database_name", result.DatabaseName)
		dataPoint.Attributes().PutStr("metric.type", sourceType)
		dataPoint.Attributes().PutStr("metric.source", "sys.dm_hadr_database_replica_states")
		dataPoint.Attributes().PutStr("metric.category", "always_on_availability_group")
		dataPoint.Attributes().PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
		dataPoint.Attributes().PutInt("engine_edition_id", int64(s.engineEdition))
	}

	return nil
}

// getMetricUnit returns the appropriate unit for each metric
func (s *FailoverClusterScraper) getMetricUnit(metricName string) string {
	switch metricName {
	case "sqlserver.failover_cluster.log_bytes_received_per_sec":
		return "By/s"
	case "sqlserver.failover_cluster.transaction_delay_ms":
		return "ms"
	case "sqlserver.failover_cluster.flow_control_time_ms":
		return "ms/s"
	case "sqlserver.failover_cluster.log_send_queue_kb":
		return "KBy"
	case "sqlserver.failover_cluster.redo_queue_kb":
		return "KBy"
	case "sqlserver.failover_cluster.redo_rate_kb_sec":
		return "KBy/s"
	default:
		return "1"
	}
}

// getMetricDescription returns the appropriate description for each metric
func (s *FailoverClusterScraper) getMetricDescription(metricName string) string {
	switch metricName {
	case "sqlserver.failover_cluster.log_bytes_received_per_sec":
		return "Rate of log records received by secondary replica from primary replica in bytes per second"
	case "sqlserver.failover_cluster.transaction_delay_ms":
		return "Average delay for transactions on the secondary replica in milliseconds"
	case "sqlserver.failover_cluster.flow_control_time_ms":
		return "Time spent in flow control by log records from primary replica in milliseconds per second"
	case "sqlserver.failover_cluster.log_send_queue_kb":
		return "Amount of log records in the log send queue waiting to be sent to the secondary replica in kilobytes"
	case "sqlserver.failover_cluster.redo_queue_kb":
		return "Amount of log records in the redo queue waiting to be redone on the secondary replica in kilobytes"
	case "sqlserver.failover_cluster.redo_rate_kb_sec":
		return "Rate at which log records are being redone on the secondary replica in kilobytes per second"
	default:
		return fmt.Sprintf("SQL Server Always On failover cluster %s metric", metricName)
	}
}
