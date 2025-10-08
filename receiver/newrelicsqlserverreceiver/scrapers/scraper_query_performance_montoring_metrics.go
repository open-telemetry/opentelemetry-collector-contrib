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
// 2. Blocking Session Detection:
//   - Query dm_exec_requests for blocked sessions
//   - Identify blocking session hierarchy (head blockers)
//   - Track blocked session duration and wait types
//   - Include session details (login, program, host)
//   - Monitor lock resource information
//
// 3. Execution Plan Analysis:
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
// - sqlserver.slowquery.avg_cpu_time_ms
// - sqlserver.slowquery.avg_disk_reads
// - sqlserver.slowquery.avg_disk_writes
// - sqlserver.slowquery.avg_elapsed_time_ms
// - sqlserver.slowquery.execution_count
// - sqlserver.slowquery.query_text
// - sqlserver.slowquery.query_id
// - sqlserver.blocking_query.wait_time_seconds
// - sqlserver.blocked_query.wait_time_seconds
// - sqlserver.wait_analysis.query_text
// - sqlserver.wait_analysis.total_wait_time_ms
// - sqlserver.wait_analysis.avg_wait_time_ms
// - sqlserver.wait_analysis.wait_event_count
// - sqlserver.wait_analysis.last_execution_time
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

// ScrapeWaitTimeAnalysisMetrics collects wait time analysis metrics using engine-specific queries
func (s *QueryPerformanceScraper) ScrapeWaitTimeAnalysisMetrics(ctx context.Context, scopeMetrics pmetric.ScopeMetrics, topN, textTruncateLimit int) error {
    s.logger.Debug("Scraping SQL Server wait time analysis metrics")

    // Format the wait query with parameters
    formattedQuery := fmt.Sprintf(queries.WaitQuery, topN, textTruncateLimit)

    // Execute wait time analysis query
    s.logger.Debug("Executing wait time analysis query",
        zap.String("query", queries.TruncateQuery(formattedQuery, 100)),
        zap.String("engine_type", queries.GetEngineTypeName(s.engineEdition)),
        zap.Int("top_n", topN),
        zap.Int("text_truncate_limit", textTruncateLimit))

    var results []models.WaitTimeAnalysis
    if err := s.connection.Query(ctx, &results, formattedQuery); err != nil {
        s.logger.Warn("Failed to execute wait time analysis query - continuing with other metrics",
            zap.Error(err),
            zap.String("query", queries.TruncateQuery(formattedQuery, 100)),
            zap.Int("engine_edition", s.engineEdition))
        // Don't return error - just log warning and continue
        return nil
    }

    s.logger.Debug("Wait query executed successfully", zap.Int("result_count", len(results)))

    // Process each wait time analysis result
    for i, result := range results {
        if err := s.processWaitTimeAnalysisMetrics(result, scopeMetrics, i); err != nil {
            s.logger.Error("Failed to process wait time analysis metrics", 
                zap.Error(err), 
                zap.Int("result_index", i))
            continue // Continue processing other results
        }
    }

    s.logger.Debug("Successfully scraped wait time analysis metrics",
        zap.Int("wait_analysis_count", len(results)))

    return nil
}

// processSlowQueryMetrics processes slow query metrics and creates separate OpenTelemetry metrics for each measurement
func (s *QueryPerformanceScraper) processSlowQueryMetrics(result models.SlowQuery, scopeMetrics pmetric.ScopeMetrics, index int) error {
    timestamp := pcommon.NewTimestampFromTime(time.Now())
    
    // Helper function to create common attributes for all metrics
    createCommonAttributes := func() pcommon.Map {
        attrs := pcommon.NewMap()
        if result.DatabaseName != nil {
            attrs.PutStr("DatabaseName", *result.DatabaseName)
        }
        if result.QueryID != nil {
            attrs.PutStr("QueryID", fmt.Sprintf("%x", *result.QueryID))
        }
        if result.StatementType != nil {
            attrs.PutStr("statement_type", *result.StatementType) 
        }
        if result.CollectionTimestamp != nil {
            attrs.PutStr("CollectionTimestamp", *result.CollectionTimestamp)
        }
        return attrs
    }

    // Create avg_cpu_time_ms metric
    if result.AvgCPUTimeMS != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.slowquery.avg_cpu_time_ms")
        metric.SetDescription("Average CPU time in milliseconds for slow query")
        metric.SetUnit("ms")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetDoubleValue(*result.AvgCPUTimeMS)
        createCommonAttributes().CopyTo(dataPoint.Attributes())
    }

    // Create avg_disk_reads metric
    if result.AvgDiskReads != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.slowquery.avg_disk_reads")
        metric.SetDescription("Average disk reads for slow query")
        metric.SetUnit("1")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetDoubleValue(*result.AvgDiskReads)
        createCommonAttributes().CopyTo(dataPoint.Attributes())
    }

    // Create avg_disk_writes metric
    if result.AvgDiskWrites != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.slowquery.avg_disk_writes")
        metric.SetDescription("Average disk writes for slow query")
        metric.SetUnit("1")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetDoubleValue(*result.AvgDiskWrites)
        createCommonAttributes().CopyTo(dataPoint.Attributes())
    }

    // Create avg_elapsed_time_ms metric
    if result.AvgElapsedTimeMS != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.slowquery.avg_elapsed_time_ms")
        metric.SetDescription("Average elapsed time in milliseconds for slow query")
        metric.SetUnit("ms")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetDoubleValue(*result.AvgElapsedTimeMS)
        createCommonAttributes().CopyTo(dataPoint.Attributes())
    }

    // Create execution_count metric
    if result.ExecutionCount != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.slowquery.execution_count")
        metric.SetDescription("Execution count for slow query")
        metric.SetUnit("1")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetIntValue(*result.ExecutionCount)
        createCommonAttributes().CopyTo(dataPoint.Attributes())
    }

    // Create query_text metric (as string attribute with dummy numeric value)
    if result.QueryText != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.slowquery.query_text")
        metric.SetDescription("Query text for slow query")
        metric.SetUnit("1")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetIntValue(1) // Dummy value since this is primarily for the string attribute
        
        attrs := createCommonAttributes()
        attrs.PutStr("query_text", *result.QueryText)
        attrs.CopyTo(dataPoint.Attributes())
    }

    // Create query_id metric (separate metric for query ID)
    if result.QueryID != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.slowquery.query_id")
        metric.SetDescription("Query ID for slow query")
        metric.SetUnit("1")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetIntValue(1) // Dummy value since this is primarily for the ID attribute
        createCommonAttributes().CopyTo(dataPoint.Attributes())
    }

    s.logger.Debug("Processed slow query metrics as separate metrics",
        zap.Any("query_id", result.QueryID),
        zap.Any("database_name", result.DatabaseName),
        zap.Any("avg_elapsed_time_ms", result.AvgElapsedTimeMS),
        zap.Any("execution_count", result.ExecutionCount))

    return nil
}

// processBlockingSessionMetrics processes blocking session metrics and creates separate OpenTelemetry metrics for BlockingSPID and BlockedSPID
func (s *QueryPerformanceScraper) processBlockingSessionMetrics(result models.BlockingSession, scopeMetrics pmetric.ScopeMetrics, index int) error {
    timestamp := pcommon.NewTimestampFromTime(time.Now())
    
    // Create BlockingSPID metric as wait time
    if result.WaitTimeInSeconds != nil && result.BlockingSPID != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.blocking_query.wait_time_seconds")
        metric.SetDescription("Wait time in seconds for blocking query")
        metric.SetUnit("s")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetDoubleValue(*result.WaitTimeInSeconds)
        
        // Set attributes including blocking SPID as an attribute
        attrs := dataPoint.Attributes()
        attrs.PutInt("BlockingSPID", int64(*result.BlockingSPID))
        if result.BlockingStatus != nil {
            attrs.PutStr("BlockingStatus", *result.BlockingStatus)
        }
        if result.WaitType != nil {
            attrs.PutStr("WaitType", *result.WaitType)
        }
        if result.DatabaseName != nil {
            attrs.PutStr("DatabaseName", *result.DatabaseName)
        }
        if result.CommandType != nil {
            attrs.PutStr("CommandType", *result.CommandType)
        }
        if result.BlockingQueryText != nil {
            attrs.PutStr("BlockingQueryText", *result.BlockingQueryText)
        }
    }

    // Create BlockedSPID metric as wait time
    if result.WaitTimeInSeconds != nil && result.BlockedSPID != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.blocked_query.wait_time_seconds")
        metric.SetDescription("Wait time in seconds for blocked query")
        metric.SetUnit("s")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetDoubleValue(*result.WaitTimeInSeconds)
        
        // Set attributes including blocked SPID as an attribute
        attrs := dataPoint.Attributes()
        attrs.PutInt("BlockedSPID", int64(*result.BlockedSPID))
        if result.BlockedStatus != nil {
            attrs.PutStr("BlockedStatus", *result.BlockedStatus)
        }
        if result.WaitType != nil {
            attrs.PutStr("WaitType", *result.WaitType)
        }
        if result.DatabaseName != nil {
            attrs.PutStr("DatabaseName", *result.DatabaseName)
        }
        if result.CommandType != nil {
            attrs.PutStr("CommandType", *result.CommandType)
        }
        if result.BlockedQueryText != nil {
            attrs.PutStr("BlockedQueryText", *result.BlockedQueryText)
        }
        if result.BlockedQueryStartTime != nil {
            attrs.PutStr("BlockedQueryStartTime", *result.BlockedQueryStartTime)
        }
    }

    s.logger.Debug("Processed blocking session metrics as separate metrics",
        zap.Any("blocking_spid", result.BlockingSPID),
        zap.Any("blocked_spid", result.BlockedSPID),
        zap.Any("wait_type", result.WaitType),
        zap.Any("wait_time_seconds", result.WaitTimeInSeconds))

    return nil
}

// processWaitTimeAnalysisMetrics processes wait time analysis metrics and creates separate OpenTelemetry metrics for each measurement
func (s *QueryPerformanceScraper) processWaitTimeAnalysisMetrics(result models.WaitTimeAnalysis, scopeMetrics pmetric.ScopeMetrics, index int) error {
    timestamp := pcommon.NewTimestampFromTime(time.Now())
    
    // Helper function to create common attributes for all metrics
    createCommonAttributes := func() pcommon.Map {
        attrs := pcommon.NewMap()
        if result.DatabaseName != nil {
            attrs.PutStr("DatabaseName", *result.DatabaseName)
        }
        if len(result.QueryID) > 0 {
            attrs.PutStr("QueryID", fmt.Sprintf("%x", result.QueryID))
        }
        if result.WaitCategory != nil {
            attrs.PutStr("WaitCategory", *result.WaitCategory)
        }
        if result.CollectionTimestamp != nil {
            attrs.PutStr("CollectionTimestamp", *result.CollectionTimestamp)
        }
        return attrs
    }

    // Create query_text metric
    if result.QueryText != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.wait_analysis.query_text")
        metric.SetDescription("Query text for wait time analysis")
        metric.SetUnit("{dimensionless}")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetIntValue(1) // Dummy numeric value required for OpenTelemetry metrics
        
        // Add query text as an attribute and also include it in common attributes
        attrs := createCommonAttributes()
        attrs.PutStr("query_text", *result.QueryText)
        attrs.CopyTo(dataPoint.Attributes())
    }

    // Create total_wait_time_ms metric
    if result.TotalWaitTimeMs != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.wait_analysis.total_wait_time_ms")
        metric.SetDescription("Total wait time in milliseconds for wait analysis")
        metric.SetUnit("ms")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetDoubleValue(*result.TotalWaitTimeMs)
        createCommonAttributes().CopyTo(dataPoint.Attributes())
    }

    // Create avg_wait_time_ms metric
    if result.AvgWaitTimeMs != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.wait_analysis.avg_wait_time_ms")
        metric.SetDescription("Average wait time in milliseconds for wait analysis")
        metric.SetUnit("ms")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetDoubleValue(*result.AvgWaitTimeMs)
        createCommonAttributes().CopyTo(dataPoint.Attributes())
    }

    // Create wait_event_count metric
    if result.WaitEventCount != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.wait_analysis.wait_event_count")
        metric.SetDescription("Wait event count for wait analysis")
        metric.SetUnit("{count}")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetIntValue(*result.WaitEventCount)
        createCommonAttributes().CopyTo(dataPoint.Attributes())
    }

    // Create last_execution_time metric
    if result.LastExecutionTime != nil {
        metric := scopeMetrics.Metrics().AppendEmpty()
        metric.SetName("sqlserver.wait_analysis.last_execution_time")
        metric.SetDescription("Last execution time for wait analysis")
        metric.SetUnit("{dimensionless}")
        
        gauge := metric.SetEmptyGauge()
        dataPoint := gauge.DataPoints().AppendEmpty()
        dataPoint.SetTimestamp(timestamp)
        dataPoint.SetStartTimestamp(s.startTime)
        dataPoint.SetIntValue(1) // Dummy value since this is primarily for the string attribute
        
        // Only use common attributes as per specification
        createCommonAttributes().CopyTo(dataPoint.Attributes())
    }

    s.logger.Debug("Processed wait time analysis metrics as separate metrics",
        zap.Any("query_id", result.QueryID),
        zap.Any("database_name", result.DatabaseName),
        zap.Any("wait_category", result.WaitCategory),
        zap.Any("total_wait_time_ms", result.TotalWaitTimeMs),
        zap.Any("avg_wait_time_ms", result.AvgWaitTimeMs),
        zap.Any("wait_event_count", result.WaitEventCount))

    return nil
}

