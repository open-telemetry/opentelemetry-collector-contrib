// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicsqlserverreceiver

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.uber.org/zap"
)

type queryPerformanceMonitoringScraper struct {
	logger *zap.Logger
	config *Config
	db     *sql.DB
}

// NewQueryPerformanceMonitoringScraper creates a new query performance monitoring scraper
func NewQueryPerformanceMonitoringScraper(cfg *Config, settings receiver.Settings) (scraper.Metrics, error) {
	s := &queryPerformanceMonitoringScraper{
		logger: settings.Logger,
		config: cfg,
	}

	return scraper.NewMetrics(
		s.scrape,
		scraper.WithStart(s.start),
		scraper.WithShutdown(s.shutdown),
	)
}

// start initializes the database connection
func (s *queryPerformanceMonitoringScraper) start(ctx context.Context, _ component.Host) error {
	db, err := NewConnection(s.config)
	if err != nil {
		return fmt.Errorf("failed to create database connection: %w", err)
	}
	s.db = db
	return nil
}

// shutdown closes the database connection
func (s *queryPerformanceMonitoringScraper) shutdown(ctx context.Context) error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// scrape collects SQL Server query performance monitoring metrics including blocking sessions
func (s *queryPerformanceMonitoringScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()

	// Set resource attributes
	resourceAttrs := resourceMetrics.Resource().Attributes()
	resourceAttrs.PutStr("server.name", s.config.Hostname)
	resourceAttrs.PutStr("database.system", "mssql")

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("newrelicsqlserverreceiver/query_performance_monitoring")

	// Collect blocking sessions metrics
	if err := s.collectBlockingSessions(ctx, scopeMetrics); err != nil {
		s.logger.Error("Failed to collect blocking sessions", zap.Error(err))
	}

	return metrics, nil
}

// collectBlockingSessions collects blocking session metrics based on nri-mssql query performance monitoring
// This mirrors the blocking session analysis from nri-mssql/src/queryanalysis/blocking_session.go
func (s *queryPerformanceMonitoringScraper) collectBlockingSessions(ctx context.Context, scopeMetrics pmetric.ScopeMetrics) error {
	// SQL query for detecting blocking sessions - based on nri-mssql query analysis
	query := `
		SELECT 
			blocking_session_id,
			session_id as blocked_session_id,
			wait_type,
			wait_time,
			wait_resource,
			command,
			blocking_these 
		FROM (
SELECT 
r.blocking_session_id,
r.session_id,
r.wait_type,
r.wait_time,
r.wait_resource,
r.command,
(
SELECT COUNT(*)
FROM sys.dm_exec_requests r2
WHERE r2.blocking_session_id = r.session_id
AND r2.blocking_session_id <> 0
				) as blocking_these
			FROM sys.dm_exec_requests r
			WHERE r.blocking_session_id <> 0
			OR r.session_id IN (
SELECT DISTINCT blocking_session_id 
FROM sys.dm_exec_requests 
WHERE blocking_session_id <> 0
			)
		) blocking_data
		WHERE blocking_session_id <> 0 OR blocking_these > 0
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query blocking sessions: %w", err)
	}
	defer rows.Close()

	var totalBlockingSessions int64
	var totalWaitTime int64
	var maxWaitTime int64

	for rows.Next() {
		var blockingSessionID, blockedSessionID sql.NullInt64
		var waitType, waitResource, command sql.NullString
		var waitTime, blockingThese sql.NullInt64

		err := rows.Scan(
			&blockingSessionID,
			&blockedSessionID,
			&waitType,
			&waitTime,
			&waitResource,
			&command,
			&blockingThese,
		)
		if err != nil {
			s.logger.Warn("Failed to scan blocking session row", zap.Error(err))
			continue
		}

		if waitTime.Valid {
			totalWaitTime += waitTime.Int64
			if waitTime.Int64 > maxWaitTime {
				maxWaitTime = waitTime.Int64
			}
		}

		totalBlockingSessions++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating blocking sessions rows: %w", err)
	}

	// Create blocking sessions count metric
	blockingCountMetric := scopeMetrics.Metrics().AppendEmpty()
	blockingCountMetric.SetName("mssql.blocking_sessions.count")
	blockingCountMetric.SetDescription("Number of blocking sessions")
	blockingCountMetric.SetUnit("sessions")

	gauge := blockingCountMetric.SetEmptyGauge()
	dataPoint := gauge.DataPoints().AppendEmpty()
	dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dataPoint.SetIntValue(totalBlockingSessions)

	attrs := dataPoint.Attributes()
	attrs.PutStr("instance", s.config.Instance)

	// Create total wait time metric
	if totalWaitTime > 0 {
		waitTimeMetric := scopeMetrics.Metrics().AppendEmpty()
		waitTimeMetric.SetName("mssql.blocking_sessions.wait_time_total")
		waitTimeMetric.SetDescription("Total wait time for blocking sessions")
		waitTimeMetric.SetUnit("ms")

		waitGauge := waitTimeMetric.SetEmptyGauge()
		waitDataPoint := waitGauge.DataPoints().AppendEmpty()
		waitDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		waitDataPoint.SetIntValue(totalWaitTime)

		waitAttrs := waitDataPoint.Attributes()
		waitAttrs.PutStr("instance", s.config.Instance)

		// Create max wait time metric
		maxWaitTimeMetric := scopeMetrics.Metrics().AppendEmpty()
		maxWaitTimeMetric.SetName("mssql.blocking_sessions.wait_time_max")
		maxWaitTimeMetric.SetDescription("Maximum wait time for blocking sessions")
		maxWaitTimeMetric.SetUnit("ms")

		maxGauge := maxWaitTimeMetric.SetEmptyGauge()
		maxDataPoint := maxGauge.DataPoints().AppendEmpty()
		maxDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		maxDataPoint.SetIntValue(maxWaitTime)

		maxAttrs := maxDataPoint.Attributes()
		maxAttrs.PutStr("instance", s.config.Instance)
	}

	return nil
}
