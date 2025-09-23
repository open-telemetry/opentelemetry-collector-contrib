// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package scrapers provides the performance monitoring scraper for SQL Server.
// This file implements comprehensive query performance monitoring including
// slow query analysis, wait time statistics, blocking session detection, and execution plan analysis.
//
// Performance Monitoring Features:
//
// 1. Top N Slow Queries:
//   - Query dm_exec_query_stats for execution statistics
//   - Rank by total_elapsed_time, avg_elapsed_time, execution_count
//   - Include query text from dm_exec_sql_text()
//   - Track CPU time, logical reads, physical reads, writes
//   - Monitor plan generation time and recompiles
//
// 2. Wait Time Analysis:
//   - Query dm_os_wait_stats for system-wide wait statistics
//   - Track wait categories: CPU, I/O, Network, Memory, Locking
//   - Calculate percentage distribution of wait types
//   - Monitor signal wait time vs resource wait time
//   - Include wait time per second calculations
//
// 3. Blocking Session Detection:
//   - Query dm_exec_requests for blocked sessions
//   - Identify blocking session hierarchy (head blockers)
//   - Track blocked session duration and wait types
//   - Include session details (login, program, host)
//   - Monitor lock resource information
//
// 4. Execution Plan Analysis:
//   - Query dm_exec_cached_plans for plan cache statistics
//   - Track plan reuse ratio and cache hit rates
//   - Monitor plan cache memory consumption
//   - Identify ad-hoc vs prepared statement ratios
//   - Include parameterized vs non-parameterized queries
//
// Scraper Structure:
//
//	type PerformanceScraper struct {
//	    config   *Config
//	    mb       *metadata.MetricsBuilder
//	    queries  *queries.PerformanceQueries
//	    logger   *zap.Logger
//	}
//
// Metrics Generated:
// - mssql.performance.slow_queries.count
// - mssql.performance.slow_queries.duration
// - mssql.performance.wait_time.duration (by category)
// - mssql.performance.blocking_sessions.count
// - mssql.performance.execution_plans.cache_hit_ratio
// - mssql.performance.execution_plans.recompiles_per_sec
//
// Engine-Specific Considerations:
// - Azure SQL Database: Limited access to some DMVs, use Azure-specific alternatives
// - Azure SQL Managed Instance: Most performance DMVs available
// - Standard SQL Server: Full access to all performance monitoring DMVs
package scrapers

import (
    "context"
    "fmt"
    "time"

    "go.opentelemetry.io/collector/pdata/pcommon"
    "go.opentelemetry.io/collector/pdata/pmetric"
    "go.uber.org/zap"

    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/models"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver/queries"
)

// QueryPerformanceScraper handles SQL Server query performance monitoring metrics collection
type QueryPerformanceScraper struct {
    connection    SQLConnectionInterface
    logger        *zap.Logger
    startTime     pcommon.Timestamp
    engineEdition int
}

// NewQueryPerformanceScraper creates a new query performance scraper
func NewQueryPerformanceScraper(conn SQLConnectionInterface, logger *zap.Logger, engineEdition int) *QueryPerformanceScraper {
    return &QueryPerformanceScraper{
        connection:    conn,
        logger:        logger,
        startTime:     pcommon.NewTimestampFromTime(time.Now()),
        engineEdition: engineEdition,
    }
}

// ScrapeBlockingSessionMetrics collects blocking session metrics using engine-specific queries
func (s *QueryPerformanceScraper) ScrapeBlockingSessionMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
    s.logger.Debug("Scraping SQL Server blocking session metrics")

    // Format the blocking sessions query with parameters
    limit := 50          // Default limit for blocking sessions
    textTruncateLimit := 1000 // Default text truncate limit
    formattedQuery := fmt.Sprintf(queries.BlockingSessionsQuery, limit, textTruncateLimit)

    // Execute blocking sessions query
    s.logger.Debug("Executing blocking sessions query",
        zap.String("query", queries.TruncateQuery(formattedQuery, 100)),
        zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)),
        zap.Int("limit", limit),
        zap.Int("text_truncate_limit", textTruncateLimit))

    var results []models.BlockingSession
    if err := s.connection.Query(ctx, &results, formattedQuery); err != nil {
        s.logger.Error("Failed to execute blocking sessions query",
            zap.Error(err),
            zap.String("query", queries.TruncateQuery(formattedQuery, 100)),
            zap.Int("engine_edition", s.engineEdition))
        return fmt.Errorf("failed to execute blocking sessions query: %w", err)
    }

    s.logger.Debug("Query executed successfully", zap.Int("result_count", len(results)))

    // Process each blocking session result
    for i, result := range results {
        if err := s.processBlockingSessionMetrics(result, scopeMetrics, i); err != nil {
            s.logger.Error("Failed to process blocking session metrics", 
                zap.Error(err), 
                zap.Int("result_index", i))
            continue // Continue processing other results
        }
    }

    s.logger.Debug("Successfully scraped blocking session metrics",
        zap.Int("blocking_session_count", len(results)))

    return nil
}

// ScrapeSlowQueryMetrics collects slow query metrics using engine-specific queries
func (s *QueryPerformanceScraper) ScrapeSlowQueryMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics, intervalSeconds, topN, elapsedTimeThreshold, textTruncateLimit int) error {
    s.logger.Debug("Scraping SQL Server slow query metrics")

    // Format the slow query with parameters
    formattedQuery := fmt.Sprintf(queries.SlowQuery, intervalSeconds, topN, elapsedTimeThreshold, textTruncateLimit)

    // Execute slow query
    s.logger.Debug("Executing slow query",
        zap.String("query", queries.TruncateQuery(formattedQuery, 100)),
        zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)),
        zap.Int("interval_seconds", intervalSeconds),
        zap.Int("top_n", topN),
        zap.Int("elapsed_time_threshold", elapsedTimeThreshold),
        zap.Int("text_truncate_limit", textTruncateLimit))

    var results []models.SlowQuery
    if err := s.connection.Query(ctx, &results, formattedQuery); err != nil {
        s.logger.Error("Failed to execute slow query",
            zap.Error(err),
            zap.String("query", queries.TruncateQuery(formattedQuery, 100)),
            zap.Int("engine_edition", s.engineEdition))
        return fmt.Errorf("failed to execute slow query: %w", err)
    }

    s.logger.Debug("Query executed successfully", zap.Int("result_count", len(results)))

    // Process each slow query result
    for i, result := range results {
        if err := s.processSlowQueryMetrics(result, scopeMetrics, i); err != nil {
            s.logger.Error("Failed to process slow query metrics", 
                zap.Error(err), 
                zap.Int("result_index", i))
            continue // Continue processing other results
        }
    }

    s.logger.Debug("Successfully scraped slow query metrics",
        zap.Int("slow_query_count", len(results)))

    return nil
}

// processSlowQueryMetrics processes slow query metrics and creates OpenTelemetry metrics
func (s *QueryPerformanceScraper) processSlowQueryMetrics(result models.SlowQuery, scopeMetrics pmetric.ScopeMetrics, index int) error {
    // Create a single gauge metric for the slow query event
    metric := scopeMetrics.Metrics().AppendEmpty()
    metric.SetName("MSSQLTopSlowQueries")
    metric.SetDescription("SQL Server top slow query details")
    metric.SetUnit("1")

    // Create gauge metric
    gauge := metric.SetEmptyGauge()
    dataPoint := gauge.DataPoints().AppendEmpty()
    dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
    dataPoint.SetStartTimestamp(s.startTime)
    
    // Set a dummy value for the gauge (1 = presence of slow query)
    dataPoint.SetIntValue(1)

    // Set all slow query attributes in nri-mssql format
    attrs := dataPoint.Attributes()
    
    // Core slow query data
    if result.QueryID != nil {
        attrs.PutStr("query_id", fmt.Sprintf("%x", *result.QueryID))
    }
    if result.QueryText != nil {
        attrs.PutStr("query_text", *result.QueryText)
    }
    if result.DatabaseName != nil {
        attrs.PutStr("database_name", *result.DatabaseName)
    }
    if result.LastExecutionTimestamp != nil {
        attrs.PutStr("last_execution_timestamp", *result.LastExecutionTimestamp)
    }
    if result.ExecutionCount != nil {
        attrs.PutInt("execution_count", *result.ExecutionCount)
    }
    if result.AvgCPUTimeMS != nil {
        attrs.PutDouble("avg_cpu_time_ms", *result.AvgCPUTimeMS)
    }
    if result.AvgElapsedTimeMS != nil {
        attrs.PutDouble("avg_elapsed_time_ms", *result.AvgElapsedTimeMS)
    }
    if result.AvgDiskReads != nil {
        attrs.PutDouble("avg_disk_reads", *result.AvgDiskReads)
    }
    if result.AvgDiskWrites != nil {
        attrs.PutDouble("avg_disk_writes", *result.AvgDiskWrites)
    }
    if result.StatementType != nil {
        attrs.PutStr("statement_type", *result.StatementType)
    }
    if result.CollectionTimestamp != nil {
        attrs.PutStr("collection_timestamp", *result.CollectionTimestamp)
    }

    // Set the event type to match nri-mssql format
    attrs.PutStr("event_type", "MSSQLTopSlowQueries")
    

    s.logger.Debug("Processed slow query event",
        zap.String("event_type", "MSSQLTopSlowQueries"),
        zap.Any("query_id", result.QueryID),
        zap.Any("database_name", result.DatabaseName),
        zap.Any("avg_elapsed_time_ms", result.AvgElapsedTimeMS),
        zap.Any("execution_count", result.ExecutionCount))

    return nil
}

// processBlockingSessionMetrics processes blocking session metrics and creates OpenTelemetry metrics
func (s *QueryPerformanceScraper) processBlockingSessionMetrics(result models.BlockingSession, scopeMetrics pmetric.ScopeMetrics, index int) error {
    // Create a single gauge metric for the blocking session event
    metric := scopeMetrics.Metrics().AppendEmpty()
    metric.SetName("MSSQLBlockingSessionQueries")
    metric.SetDescription("SQL Server blocking session query details")
    metric.SetUnit("1")

    // Create gauge metric
    gauge := metric.SetEmptyGauge()
    dataPoint := gauge.DataPoints().AppendEmpty()
    dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
    dataPoint.SetStartTimestamp(s.startTime)
    
    // Set a dummy value for the gauge (1 = presence of blocking session)
    dataPoint.SetIntValue(1)

    // Set all blocking session attributes in nri-mssql format
    attrs := dataPoint.Attributes()
    
    // Core blocking session data
    if result.BlockingSPID != nil {
        attrs.PutInt("BlockingSPID", *result.BlockingSPID)
    }
    if result.BlockingStatus != nil {
        attrs.PutStr("BlockingStatus", *result.BlockingStatus)
    }
    if result.BlockedSPID != nil {
        attrs.PutInt("BlockedSPID", *result.BlockedSPID)
    }
    if result.BlockedStatus != nil {
        attrs.PutStr("BlockedStatus", *result.BlockedStatus)
    }
    if result.WaitType != nil {
        attrs.PutStr("WaitType", *result.WaitType)
    }
    if result.WaitTimeInSeconds != nil {
        attrs.PutDouble("WaitTimeInSeconds", *result.WaitTimeInSeconds)
    }
    if result.CommandType != nil {
        attrs.PutStr("CommandType", *result.CommandType)
    }
    if result.DatabaseName != nil {
        attrs.PutStr("DatabaseName", *result.DatabaseName)
    }
    if result.BlockingQueryText != nil {
        attrs.PutStr("BlockingQueryText", *result.BlockingQueryText)
    }
    if result.BlockedQueryText != nil {
        attrs.PutStr("BlockedQueryText", *result.BlockedQueryText)
    }
    if result.BlockedQueryStartTime != nil {
        attrs.PutStr("BlockedQueryStartTime", *result.BlockedQueryStartTime)
    }

    // Set the event type to match nri-mssql format
    attrs.PutStr("event_type", "MSSQLBlockingSessionQueries")
    

    s.logger.Debug("Processed blocking session event",
        zap.String("event_type", "MSSQLBlockingSessionQueries"),
        zap.Any("blocking_spid", result.BlockingSPID),
        zap.Any("blocked_spid", result.BlockedSPID),
        zap.Any("wait_type", result.WaitType),
        zap.Any("wait_time_seconds", result.WaitTimeInSeconds))

    return nil
}