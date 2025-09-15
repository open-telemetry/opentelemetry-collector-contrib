// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// coreMetricsScraper handles collection of core SQL Server instance metrics
// Based on nri-mssql instance metrics collection patterns
type coreMetricsScraper struct {
	connection     *SQLConnection
	instanceHelper *InstanceHelper
	databaseHelper *DatabaseHelper
	logger         *zap.Logger
	config         *Config
	startTime      pcommon.Timestamp
}

// SQL Server performance counters query for user connections metric
// Based on nri-mssql metrics/instance_metric_definitions.go user_connections query
const userConnectionsQuery = `
	SELECT cntr_value AS user_connections 
	FROM sys.dm_os_performance_counters WITH (nolock) 
	WHERE counter_name = 'User Connections'
`

// UserConnectionsResult represents the result structure for user connections metric
// Based on nri-mssql instance metric data models
type UserConnectionsResult struct {
	UserConnections *int64 `db:"user_connections"`
}

// newCoreMetricsScraper creates a new core metrics scraper instance
func newCoreMetricsScraper(connection *SQLConnection, config *Config, logger *zap.Logger) *coreMetricsScraper {
	return &coreMetricsScraper{
		connection:     connection,
		instanceHelper: NewInstanceHelper(connection, logger),
		databaseHelper: NewDatabaseHelper(connection, logger),
		logger:         logger,
		config:         config,
		startTime:      pcommon.NewTimestampFromTime(time.Now()),
	}
}

// scrape collects core SQL Server instance metrics and returns them as pmetric.Metrics
// This implements one complete end-to-end metric collection (user connections)
func (cms *coreMetricsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	cms.logger.Debug("Starting core metrics scraping")

	// Create metrics container
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()

	// Set resource attributes
	resource := resourceMetrics.Resource()
	if err := cms.setResourceAttributes(ctx, resource); err != nil {
		return metrics, fmt.Errorf("failed to set resource attributes: %w", err)
	}

	// Create scope metrics
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("newrelicsqlserverreceiver")

	// Collect user connections metric (end-to-end implementation)
	if err := cms.collectUserConnectionsMetric(ctx, scopeMetrics); err != nil {
		cms.logger.Error("Failed to collect user connections metric", zap.Error(err))
		// Continue collecting other metrics even if one fails
	}

	cms.logger.Debug("Completed core metrics scraping")
	return metrics, nil
}

// collectUserConnectionsMetric collects the user connections metric end-to-end
// Based on nri-mssql instance metrics pattern for user_connections
func (cms *coreMetricsScraper) collectUserConnectionsMetric(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	cms.logger.Debug("Collecting user connections metric")

	// Execute the SQL query
	var results []UserConnectionsResult
	if err := cms.connection.Query(ctx, &results, userConnectionsQuery); err != nil {
		return fmt.Errorf("failed to execute user connections query: %w", err)
	}

	if len(results) == 0 {
		cms.logger.Warn("No results returned for user connections query")
		return nil
	}

	result := results[0]
	if result.UserConnections == nil {
		cms.logger.Warn("User connections value is null")
		return nil
	}

	// Create the metric
	metric := scopeMetrics.Metrics().AppendEmpty()
	metric.SetName("sqlserver.stats.connections")
	metric.SetDescription("Number of user connections to the SQL Server")
	metric.SetUnit("{connections}")

	// Set as gauge metric (current snapshot value)
	gauge := metric.SetEmptyGauge()
	dataPoint := gauge.DataPoints().AppendEmpty()

	// Set the metric value
	dataPoint.SetIntValue(*result.UserConnections)

	// Set timestamp
	now := pcommon.NewTimestampFromTime(time.Now())
	dataPoint.SetTimestamp(now)
	dataPoint.SetStartTimestamp(cms.startTime)

	// Add metric-specific attributes if needed
	attrs := dataPoint.Attributes()
	attrs.PutStr("metric.type", "gauge")
	attrs.PutStr("metric.source", "sys.dm_os_performance_counters")

	cms.logger.Debug("Successfully collected user connections metric",
		zap.Int64("value", *result.UserConnections))

	return nil
}

// setResourceAttributes sets resource attributes for the metrics
// Based on OpenTelemetry resource semantics and nri-mssql entity attributes
func (cms *coreMetricsScraper) setResourceAttributes(ctx context.Context, resource pcommon.Resource) error {
	attrs := resource.Attributes()

	// Get instance information
	instanceInfo, err := cms.instanceHelper.GetInstanceInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get instance info: %w", err)
	}

	// Set standard resource attributes
	attrs.PutStr("server.address", cms.config.Hostname)
	if cms.config.Port != "" {
		attrs.PutStr("server.port", cms.config.Port)
	}
	attrs.PutStr("sqlserver.instance.name", instanceInfo.Name)

	// Add SQL Server specific attributes
	attrs.PutStr("db.system", "mssql")
	attrs.PutStr("service.name", "sqlserver")
	attrs.PutStr("service.instance.id", instanceInfo.Name)

	cms.logger.Debug("Set resource attributes",
		zap.String("instance_name", instanceInfo.Name),
		zap.String("hostname", cms.config.Hostname))

	return nil
}

// validateConnection validates that the scraper can connect to SQL Server
func (cms *coreMetricsScraper) validateConnection(ctx context.Context) error {
	// Test basic connection
	if err := cms.connection.Ping(ctx); err != nil {
		return fmt.Errorf("connection ping failed: %w", err)
	}

	// Validate instance access
	if err := cms.instanceHelper.ValidateInstanceAccess(ctx); err != nil {
		return fmt.Errorf("instance access validation failed: %w", err)
	}

	// Validate database access
	if err := cms.databaseHelper.ValidateDatabaseAccess(ctx); err != nil {
		return fmt.Errorf("database access validation failed: %w", err)
	}

	cms.logger.Debug("Connection validation passed")
	return nil
}
