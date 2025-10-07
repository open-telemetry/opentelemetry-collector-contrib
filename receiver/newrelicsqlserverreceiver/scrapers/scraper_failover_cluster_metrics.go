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
	case "sqlserver.failover_cluster.node_metrics":
		switch s.engineEdition {
		case queries.AzureSQLDatabaseEngineEdition:
			return queries.FailoverClusterNodeQueryAzureSQL, true
		case queries.AzureSQLManagedInstanceEngineEdition:
			return queries.FailoverClusterNodeQueryAzureMI, true
		default:
			return queries.FailoverClusterNodeQuery, true
		}
	case "sqlserver.failover_cluster.availability_group_health_metrics":
		switch s.engineEdition {
		case queries.AzureSQLDatabaseEngineEdition:
			return queries.FailoverClusterAvailabilityGroupHealthQueryAzureSQL, true
		case queries.AzureSQLManagedInstanceEngineEdition:
			return queries.FailoverClusterAvailabilityGroupHealthQueryAzureMI, true
		default:
			return queries.FailoverClusterAvailabilityGroupHealthQuery, true
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

// ScrapeFailoverClusterNodeMetrics collects cluster node information and status
// This method retrieves information about Windows Server Failover Cluster nodes
func (s *FailoverClusterScraper) ScrapeFailoverClusterNodeMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server failover cluster node metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.failover_cluster.node_metrics")
	if !found {
		return fmt.Errorf("no failover cluster node metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing failover cluster node metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.FailoverClusterNodeMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute failover cluster node query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute failover cluster node query: %w", err)
	}

	// If no results, this SQL Server instance may not be in a cluster
	if len(results) == 0 {
		s.logger.Debug("No cluster node metrics found - SQL Server may not be part of a Windows Server Failover Cluster")
		return nil
	}

	s.logger.Debug("Processing failover cluster node metrics results",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each node result
	for _, result := range results {
		if err := s.processFailoverClusterNodeMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process failover cluster node metrics",
				zap.Error(err),
				zap.String("node_name", result.NodeName))
			return err
		}
	}

	return nil
}

// processFailoverClusterNodeMetrics processes cluster node metrics and creates OpenTelemetry metrics
func (s *FailoverClusterScraper) processFailoverClusterNodeMetrics(result models.FailoverClusterNodeMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Process IsCurrentOwner as a gauge metric
	if result.IsCurrentOwner != nil {
		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("sqlserver.failover_cluster.node_is_current_owner")
		metric.SetUnit("1")
		metric.SetDescription("Indicates if this is the active node currently running the SQL Server instance (1=active, 0=passive)")

		gauge := metric.SetEmptyGauge()
		dataPoint := gauge.DataPoints().AppendEmpty()
		dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dataPoint.SetStartTimestamp(s.startTime)
		dataPoint.SetIntValue(*result.IsCurrentOwner)

		// Add attributes
		dataPoint.Attributes().PutStr("node_name", result.NodeName)
		dataPoint.Attributes().PutStr("status_description", result.StatusDescription)
		dataPoint.Attributes().PutStr("metric.source", "sys.dm_os_cluster_nodes")
		dataPoint.Attributes().PutStr("metric.category", "failover_cluster_node")
		dataPoint.Attributes().PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
		dataPoint.Attributes().PutInt("engine_edition_id", int64(s.engineEdition))
	}

	// Process StatusDescription as an info metric (gauge with value 1)
	statusMetric := scopeMetrics.Metrics().AppendEmpty()
	statusMetric.SetName("sqlserver.failover_cluster.node_status")
	statusMetric.SetUnit("1")
	statusMetric.SetDescription("Health state of the cluster node")

	statusGauge := statusMetric.SetEmptyGauge()
	statusDataPoint := statusGauge.DataPoints().AppendEmpty()
	statusDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	statusDataPoint.SetStartTimestamp(s.startTime)
	statusDataPoint.SetIntValue(1) // Info metric always has value 1

	// Add attributes for status metric
	statusDataPoint.Attributes().PutStr("node_name", result.NodeName)
	statusDataPoint.Attributes().PutStr("status_description", result.StatusDescription)
	statusDataPoint.Attributes().PutStr("metric.source", "sys.dm_os_cluster_nodes")
	statusDataPoint.Attributes().PutStr("metric.category", "failover_cluster_node")
	statusDataPoint.Attributes().PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
	statusDataPoint.Attributes().PutInt("engine_edition_id", int64(s.engineEdition))

	return nil
}

// ScrapeFailoverClusterAvailabilityGroupHealthMetrics collects Availability Group health status
// This method retrieves health and role information for all availability group replicas
func (s *FailoverClusterScraper) ScrapeFailoverClusterAvailabilityGroupHealthMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	s.logger.Debug("Scraping SQL Server Availability Group health metrics")

	// Get the appropriate query for this engine edition
	query, found := s.getQueryForMetric("sqlserver.failover_cluster.availability_group_health_metrics")
	if !found {
		return fmt.Errorf("no availability group health metrics query available for engine edition %d", s.engineEdition)
	}

	s.logger.Debug("Executing availability group health metrics query",
		zap.String("query", queries.TruncateQuery(query, 100)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	var results []models.FailoverClusterAvailabilityGroupHealthMetrics
	if err := s.connection.Query(ctx, &results, query); err != nil {
		s.logger.Error("Failed to execute availability group health query",
			zap.Error(err),
			zap.String("query", queries.TruncateQuery(query, 100)),
			zap.Int("engine_edition", s.engineEdition))
		return fmt.Errorf("failed to execute availability group health query: %w", err)
	}

	// If no results, this SQL Server instance may not have Always On AG configured
	if len(results) == 0 {
		s.logger.Debug("No availability group health metrics found - SQL Server may not have Always On Availability Groups configured")
		return nil
	}

	s.logger.Debug("Processing availability group health metrics results",
		zap.Int("result_count", len(results)),
		zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)))

	// Process each availability group health result
	for _, result := range results {
		if err := s.processFailoverClusterAvailabilityGroupHealthMetrics(result, scopeMetrics); err != nil {
			s.logger.Error("Failed to process availability group health metrics",
				zap.Error(err),
				zap.String("replica_server_name", result.ReplicaServerName))
			return err
		}
	}

	return nil
}

// processFailoverClusterAvailabilityGroupHealthMetrics processes availability group health metrics and creates OpenTelemetry metrics
func (s *FailoverClusterScraper) processFailoverClusterAvailabilityGroupHealthMetrics(result models.FailoverClusterAvailabilityGroupHealthMetrics, scopeMetrics pmetric.ScopeMetrics) error {
	// Process RoleDesc as an info metric (gauge with value 1)
	roleMetric := scopeMetrics.Metrics().AppendEmpty()
	roleMetric.SetName("sqlserver.failover_cluster.ag_replica_role")
	roleMetric.SetUnit("1")
	roleMetric.SetDescription("Current role of the replica within the Availability Group (PRIMARY or SECONDARY)")

	roleGauge := roleMetric.SetEmptyGauge()
	roleDataPoint := roleGauge.DataPoints().AppendEmpty()
	roleDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	roleDataPoint.SetStartTimestamp(s.startTime)
	roleDataPoint.SetIntValue(1) // Info metric always has value 1

	// Add attributes for role metric
	roleDataPoint.Attributes().PutStr("replica_server_name", result.ReplicaServerName)
	roleDataPoint.Attributes().PutStr("role_desc", result.RoleDesc)
	roleDataPoint.Attributes().PutStr("synchronization_health_desc", result.SynchronizationHealthDesc)
	roleDataPoint.Attributes().PutStr("metric.source", "sys.dm_hadr_availability_replica_states")
	roleDataPoint.Attributes().PutStr("metric.category", "availability_group_health")
	roleDataPoint.Attributes().PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
	roleDataPoint.Attributes().PutInt("engine_edition_id", int64(s.engineEdition))

	// Process SynchronizationHealthDesc as an info metric (gauge with value 1)
	healthMetric := scopeMetrics.Metrics().AppendEmpty()
	healthMetric.SetName("sqlserver.failover_cluster.ag_synchronization_health")
	healthMetric.SetUnit("1")
	healthMetric.SetDescription("Health of data synchronization between primary and secondary replica (HEALTHY, PARTIALLY_HEALTHY, NOT_HEALTHY)")

	healthGauge := healthMetric.SetEmptyGauge()
	healthDataPoint := healthGauge.DataPoints().AppendEmpty()
	healthDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	healthDataPoint.SetStartTimestamp(s.startTime)
	healthDataPoint.SetIntValue(1) // Info metric always has value 1

	// Add attributes for health metric
	healthDataPoint.Attributes().PutStr("replica_server_name", result.ReplicaServerName)
	healthDataPoint.Attributes().PutStr("role_desc", result.RoleDesc)
	healthDataPoint.Attributes().PutStr("synchronization_health_desc", result.SynchronizationHealthDesc)
	healthDataPoint.Attributes().PutStr("metric.source", "sys.dm_hadr_availability_replica_states")
	healthDataPoint.Attributes().PutStr("metric.category", "availability_group_health")
	healthDataPoint.Attributes().PutStr("engine_edition", queries.GetEngineTypeName(s.engineEdition))
	healthDataPoint.Attributes().PutInt("engine_edition_id", int64(s.engineEdition))

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
	case "sqlserver.failover_cluster.node_is_current_owner":
		return "1"
	case "sqlserver.failover_cluster.node_status":
		return "1"
	case "sqlserver.failover_cluster.ag_replica_role":
		return "1"
	case "sqlserver.failover_cluster.ag_synchronization_health":
		return "1"
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
	case "sqlserver.failover_cluster.node_is_current_owner":
		return "Indicates if this is the active node currently running the SQL Server instance (1=active, 0=passive)"
	case "sqlserver.failover_cluster.node_status":
		return "Health state of the cluster node"
	case "sqlserver.failover_cluster.ag_replica_role":
		return "Current role of the replica within the Availability Group (PRIMARY or SECONDARY)"
	case "sqlserver.failover_cluster.ag_synchronization_health":
		return "Health of data synchronization between primary and secondary replica (HEALTHY, PARTIALLY_HEALTHY, NOT_HEALTHY)"
	default:
		return fmt.Sprintf("SQL Server Always On failover cluster %s metric", metricName)
	}
}
