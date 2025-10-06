// Licensed to The New Relic under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. The New Relic licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package scrapers

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// ConnectionScraper contains the scraper for Oracle connection statistics
type ConnectionScraper struct {
	db                   *sql.DB
	mb                   *metadata.MetricsBuilder
	logger               *zap.Logger
	instanceName         string
	metricsBuilderConfig metadata.MetricsBuilderConfig
}

// NewConnectionScraper creates a new Connection Scraper instance
func NewConnectionScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, metricsBuilderConfig metadata.MetricsBuilderConfig) *ConnectionScraper {
	return &ConnectionScraper{
		db:                   db,
		mb:                   mb,
		logger:               logger,
		instanceName:         instanceName,
		metricsBuilderConfig: metricsBuilderConfig,
	}
}

// ScrapeConnectionMetrics collects Oracle connection statistics
func (s *ConnectionScraper) ScrapeConnectionMetrics(ctx context.Context) []error {
	s.logger.Debug("Begin Oracle connection metrics scrape")

	var scrapeErrors []error
	now := pcommon.NewTimestampFromTime(time.Now())

	// Scrape core connection counts
	if err := s.scrapeCoreConnectionCounts(ctx, now); err != nil {
		scrapeErrors = append(scrapeErrors, err...)
	}

	// Scrape session breakdowns
	if err := s.scrapeSessionBreakdown(ctx, now); err != nil {
		scrapeErrors = append(scrapeErrors, err...)
	}

	// Scrape logon statistics
	if err := s.scrapeLogonStats(ctx, now); err != nil {
		scrapeErrors = append(scrapeErrors, err...)
	}

	// Scrape session resource consumption (top sessions)
	if err := s.scrapeSessionResourceConsumption(ctx, now); err != nil {
		scrapeErrors = append(scrapeErrors, err...)
	}

	// Scrape wait events
	if err := s.scrapeWaitEvents(ctx, now); err != nil {
		scrapeErrors = append(scrapeErrors, err...)
	}

	// Scrape blocking sessions
	if err := s.scrapeBlockingSessions(ctx, now); err != nil {
		scrapeErrors = append(scrapeErrors, err...)
	}

	// Scrape wait event summary
	if err := s.scrapeWaitEventSummary(ctx, now); err != nil {
		scrapeErrors = append(scrapeErrors, err...)
	}

	// Scrape connection pool metrics
	if err := s.scrapeConnectionPoolMetrics(ctx, now); err != nil {
		scrapeErrors = append(scrapeErrors, err...)
	}

	// Scrape session limits
	if err := s.scrapeSessionLimits(ctx, now); err != nil {
		scrapeErrors = append(scrapeErrors, err...)
	}

	// Scrape connection quality metrics
	if err := s.scrapeConnectionQuality(ctx, now); err != nil {
		scrapeErrors = append(scrapeErrors, err...)
	}

	s.logger.Debug("Completed Oracle connection metrics scrape")
	return scrapeErrors
}

// scrapeCoreConnectionCounts scrapes basic connection counts
func (s *ConnectionScraper) scrapeCoreConnectionCounts(ctx context.Context, timestamp pcommon.Timestamp) []error {
	var errors []error

	// Total Sessions
	if err := s.scrapeSingleValue(ctx, queries.TotalSessionsSQL, "total_sessions", timestamp); err != nil {
		errors = append(errors, err)
	}

	// Active Sessions
	if err := s.scrapeSingleValue(ctx, queries.ActiveSessionsSQL, "active_sessions", timestamp); err != nil {
		errors = append(errors, err)
	}

	// Inactive Sessions
	if err := s.scrapeSingleValue(ctx, queries.InactiveSessionsSQL, "inactive_sessions", timestamp); err != nil {
		errors = append(errors, err)
	}

	return errors
}

// scrapeSessionBreakdown scrapes session breakdown by status and type
func (s *ConnectionScraper) scrapeSessionBreakdown(ctx context.Context, timestamp pcommon.Timestamp) []error {
	var errors []error

	// Session Status Breakdown
	rows, err := s.db.QueryContext(ctx, queries.SessionStatusSQL)
	if err != nil {
		s.logger.Error("Failed to execute session status query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var status sql.NullString
		var count sql.NullInt64

		if err := rows.Scan(&status, &count); err != nil {
			s.logger.Error("Failed to scan session status row", zap.Error(err))
			errors = append(errors, err)
			continue
		}

		if status.Valid && count.Valid {
			s.mb.RecordNewrelicoracledbConnectionSessionsByStatusDataPoint(
				timestamp,
				float64(count.Int64),
				s.instanceName,
				status.String,
			)
		}
	}

	// Session Type Breakdown
	rows, err = s.db.QueryContext(ctx, queries.SessionTypeSQL)
	if err != nil {
		s.logger.Error("Failed to execute session type query", zap.Error(err))
		errors = append(errors, err)
		return errors
	}
	defer rows.Close()

	for rows.Next() {
		var sessionType sql.NullString
		var count sql.NullInt64

		if err := rows.Scan(&sessionType, &count); err != nil {
			s.logger.Error("Failed to scan session type row", zap.Error(err))
			errors = append(errors, err)
			continue
		}

		if sessionType.Valid && count.Valid {
			s.mb.RecordNewrelicoracledbConnectionSessionsByTypeDataPoint(
				timestamp,
				float64(count.Int64),
				s.instanceName,
				sessionType.String,
			)
		}
	}

	return errors
}

// scrapeLogonStats scrapes logon statistics
func (s *ConnectionScraper) scrapeLogonStats(ctx context.Context, timestamp pcommon.Timestamp) []error {
	var errors []error

	rows, err := s.db.QueryContext(ctx, queries.LogonsStatsSQL)
	if err != nil {
		s.logger.Error("Failed to execute logons stats query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var name sql.NullString
		var value sql.NullFloat64

		if err := rows.Scan(&name, &value); err != nil {
			s.logger.Error("Failed to scan logons stats row", zap.Error(err))
			errors = append(errors, err)
			continue
		}

		if !name.Valid || !value.Valid {
			continue
		}

		switch name.String {
		case "logons cumulative":
			s.mb.RecordNewrelicoracledbConnectionLogonsCumulativeDataPoint(
				timestamp,
				value.Float64,
				s.instanceName,
			)
		case "logons current":
			s.mb.RecordNewrelicoracledbConnectionLogonsCurrentDataPoint(
				timestamp,
				value.Float64,
				s.instanceName,
			)
		}
	}

	return errors
}

// scrapeSessionResourceConsumption scrapes top resource consuming sessions
func (s *ConnectionScraper) scrapeSessionResourceConsumption(ctx context.Context, timestamp pcommon.Timestamp) []error {
	var errors []error

	rows, err := s.db.QueryContext(ctx, queries.SessionResourceConsumptionSQL)
	if err != nil {
		s.logger.Error("Failed to execute session resource consumption query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var sid sql.NullInt64
		var username sql.NullString
		var status sql.NullString
		var program sql.NullString
		var machine sql.NullString
		var osuser sql.NullString
		var logonTime sql.NullTime
		var lastCallET sql.NullInt64
		var cpuUsageSeconds sql.NullFloat64
		var pgaMemoryBytes sql.NullInt64
		var logicalReads sql.NullInt64

		if err := rows.Scan(&sid, &username, &status, &program, &machine, &osuser,
			&logonTime, &lastCallET, &cpuUsageSeconds, &pgaMemoryBytes, &logicalReads); err != nil {
			s.logger.Error("Failed to scan session resource consumption row", zap.Error(err))
			errors = append(errors, err)
			continue
		}

		// Convert SID to string for attribute
		sidStr := ""
		if sid.Valid {
			sidStr = strconv.FormatInt(sid.Int64, 10)
		}

		userStr := ""
		if username.Valid {
			userStr = username.String
		}

		statusStr := ""
		if status.Valid {
			statusStr = status.String
		}

		programStr := ""
		if program.Valid {
			programStr = program.String
		}

		// Record CPU usage
		if cpuUsageSeconds.Valid {
			s.mb.RecordNewrelicoracledbConnectionSessionCPUUsageDataPoint(
				timestamp,
				cpuUsageSeconds.Float64,
				s.instanceName,
				sidStr,
				userStr,
				statusStr,
				programStr,
			)
		}

		// Record PGA memory usage
		if pgaMemoryBytes.Valid {
			s.mb.RecordNewrelicoracledbConnectionSessionPgaMemoryDataPoint(
				timestamp,
				float64(pgaMemoryBytes.Int64),
				s.instanceName,
				sidStr,
				userStr,
				statusStr,
				programStr,
			)
		}

		// Record logical reads
		if logicalReads.Valid {
			s.mb.RecordNewrelicoracledbConnectionSessionLogicalReadsDataPoint(
				timestamp,
				float64(logicalReads.Int64),
				s.instanceName,
				sidStr,
				userStr,
				statusStr,
				programStr,
			)
		}

		// Record last call elapsed time (session idle time)
		if lastCallET.Valid {
			s.mb.RecordNewrelicoracledbConnectionSessionIdleTimeDataPoint(
				timestamp,
				float64(lastCallET.Int64),
				s.instanceName,
				sidStr,
				userStr,
				statusStr,
				programStr,
			)
		}
	}

	return errors
}

// scrapeWaitEvents scrapes current wait events
func (s *ConnectionScraper) scrapeWaitEvents(ctx context.Context, timestamp pcommon.Timestamp) []error {
	var errors []error

	rows, err := s.db.QueryContext(ctx, queries.CurrentWaitEventsSQL)
	if err != nil {
		s.logger.Error("Failed to execute current wait events query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var sid sql.NullInt64
		var username sql.NullString
		var event sql.NullString
		var waitTime sql.NullInt64
		var state sql.NullString
		var secondsInWait sql.NullInt64
		var waitClass sql.NullString

		if err := rows.Scan(&sid, &username, &event, &waitTime, &state, &secondsInWait, &waitClass); err != nil {
			s.logger.Error("Failed to scan wait events row", zap.Error(err))
			errors = append(errors, err)
			continue
		}

		if !sid.Valid || !event.Valid || !secondsInWait.Valid {
			continue
		}

		sidStr := strconv.FormatInt(sid.Int64, 10)
		userStr := ""
		if username.Valid {
			userStr = username.String
		}
		eventStr := event.String
		stateStr := ""
		if state.Valid {
			stateStr = state.String
		}
		waitClassStr := ""
		if waitClass.Valid {
			waitClassStr = waitClass.String
		}

		s.mb.RecordNewrelicoracledbConnectionWaitEventsDataPoint(
			timestamp,
			float64(secondsInWait.Int64),
			s.instanceName,
			sidStr,
			userStr,
			eventStr,
			stateStr,
			waitClassStr,
		)
	}

	return errors
}

// scrapeBlockingSessions scrapes blocking sessions information
func (s *ConnectionScraper) scrapeBlockingSessions(ctx context.Context, timestamp pcommon.Timestamp) []error {
	var errors []error

	rows, err := s.db.QueryContext(ctx, queries.BlockingSessionsSQL)
	if err != nil {
		s.logger.Error("Failed to execute blocking sessions query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var sid sql.NullInt64
		var serial sql.NullInt64
		var blockingSession sql.NullInt64
		var event sql.NullString
		var username sql.NullString
		var program sql.NullString
		var secondsInWait sql.NullInt64

		if err := rows.Scan(&sid, &serial, &blockingSession, &event, &username, &program, &secondsInWait); err != nil {
			s.logger.Error("Failed to scan blocking sessions row", zap.Error(err))
			errors = append(errors, err)
			continue
		}

		if !sid.Valid || !blockingSession.Valid || !secondsInWait.Valid {
			continue
		}

		sidStr := strconv.FormatInt(sid.Int64, 10)
		blockingSidStr := strconv.FormatInt(blockingSession.Int64, 10)
		userStr := ""
		if username.Valid {
			userStr = username.String
		}
		eventStr := ""
		if event.Valid {
			eventStr = event.String
		}
		programStr := ""
		if program.Valid {
			programStr = program.String
		}

		s.mb.RecordNewrelicoracledbConnectionBlockingSessionsDataPoint(
			timestamp,
			float64(secondsInWait.Int64),
			s.instanceName,
			sidStr,
			blockingSidStr,
			userStr,
			eventStr,
			programStr,
		)
	}

	return errors
}

// scrapeWaitEventSummary scrapes wait event summary
func (s *ConnectionScraper) scrapeWaitEventSummary(ctx context.Context, timestamp pcommon.Timestamp) []error {
	var errors []error

	rows, err := s.db.QueryContext(ctx, queries.WaitEventSummarySQL)
	if err != nil {
		s.logger.Error("Failed to execute wait event summary query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var event sql.NullString
		var totalWaits sql.NullInt64
		var timeWaitedMicro sql.NullInt64
		var averageWaitMicro sql.NullFloat64
		var waitClass sql.NullString

		if err := rows.Scan(&event, &totalWaits, &timeWaitedMicro, &averageWaitMicro, &waitClass); err != nil {
			s.logger.Error("Failed to scan wait event summary row", zap.Error(err))
			errors = append(errors, err)
			continue
		}

		if !event.Valid || !totalWaits.Valid || !timeWaitedMicro.Valid {
			continue
		}

		eventStr := event.String
		waitClassStr := ""
		if waitClass.Valid {
			waitClassStr = waitClass.String
		}

		// Record total waits
		s.mb.RecordNewrelicoracledbConnectionWaitEventTotalWaitsDataPoint(
			timestamp,
			float64(totalWaits.Int64),
			s.instanceName,
			eventStr,
			waitClassStr,
		)

		// Record time waited (convert microseconds to milliseconds)
		s.mb.RecordNewrelicoracledbConnectionWaitEventTimeWaitedDataPoint(
			timestamp,
			float64(timeWaitedMicro.Int64)/1000.0,
			s.instanceName,
			eventStr,
			waitClassStr,
		)

		// Record average wait time
		if averageWaitMicro.Valid {
			s.mb.RecordNewrelicoracledbConnectionWaitEventAvgWaitTimeDataPoint(
				timestamp,
				averageWaitMicro.Float64/1000.0, // Convert to milliseconds
				s.instanceName,
				eventStr,
				waitClassStr,
			)
		}
	}

	return errors
}

// scrapeConnectionPoolMetrics scrapes connection pool metrics
func (s *ConnectionScraper) scrapeConnectionPoolMetrics(ctx context.Context, timestamp pcommon.Timestamp) []error {
	var errors []error

	rows, err := s.db.QueryContext(ctx, queries.ConnectionPoolMetricsSQL)
	if err != nil {
		s.logger.Error("Failed to execute connection pool metrics query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var metricName sql.NullString
		var value sql.NullInt64

		if err := rows.Scan(&metricName, &value); err != nil {
			s.logger.Error("Failed to scan connection pool metrics row", zap.Error(err))
			errors = append(errors, err)
			continue
		}

		if !metricName.Valid || !value.Valid {
			continue
		}

		switch metricName.String {
		case "shared_servers":
			s.mb.RecordNewrelicoracledbConnectionSharedServersDataPoint(
				timestamp,
				float64(value.Int64),
				s.instanceName,
			)
		case "dispatchers":
			s.mb.RecordNewrelicoracledbConnectionDispatchersDataPoint(
				timestamp,
				float64(value.Int64),
				s.instanceName,
			)
		case "circuits":
			s.mb.RecordNewrelicoracledbConnectionCircuitsDataPoint(
				timestamp,
				float64(value.Int64),
				s.instanceName,
			)
		}
	}

	return errors
}

// scrapeSessionLimits scrapes session limits
func (s *ConnectionScraper) scrapeSessionLimits(ctx context.Context, timestamp pcommon.Timestamp) []error {
	var errors []error

	rows, err := s.db.QueryContext(ctx, queries.SessionLimitsSQL)
	if err != nil {
		s.logger.Error("Failed to execute session limits query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var resourceName sql.NullString
		var currentUtilization sql.NullInt64
		var maxUtilization sql.NullInt64
		var initialAllocation sql.NullString
		var limitValue sql.NullString

		if err := rows.Scan(&resourceName, &currentUtilization, &maxUtilization, &initialAllocation, &limitValue); err != nil {
			s.logger.Error("Failed to scan session limits row", zap.Error(err))
			errors = append(errors, err)
			continue
		}

		if !resourceName.Valid || !currentUtilization.Valid {
			continue
		}

		resourceStr := resourceName.String

		// Record current utilization
		s.mb.RecordNewrelicoracledbConnectionResourceCurrentUtilizationDataPoint(
			timestamp,
			float64(currentUtilization.Int64),
			s.instanceName,
			resourceStr,
		)

		// Record max utilization if available
		if maxUtilization.Valid {
			s.mb.RecordNewrelicoracledbConnectionResourceMaxUtilizationDataPoint(
				timestamp,
				float64(maxUtilization.Int64),
				s.instanceName,
				resourceStr,
			)
		}

		// Record limit value if available and numeric
		if limitValue.Valid && limitValue.String != "UNLIMITED" {
			if limit, err := strconv.ParseInt(limitValue.String, 10, 64); err == nil {
				s.mb.RecordNewrelicoracledbConnectionResourceLimitDataPoint(
					timestamp,
					float64(limit),
					s.instanceName,
					resourceStr,
				)
			}
		}
	}

	return errors
}

// scrapeConnectionQuality scrapes connection quality metrics
func (s *ConnectionScraper) scrapeConnectionQuality(ctx context.Context, timestamp pcommon.Timestamp) []error {
	var errors []error

	rows, err := s.db.QueryContext(ctx, queries.ConnectionQualitySQL)
	if err != nil {
		s.logger.Error("Failed to execute connection quality query", zap.Error(err))
		return []error{err}
	}
	defer rows.Close()

	for rows.Next() {
		var name sql.NullString
		var value sql.NullFloat64

		if err := rows.Scan(&name, &value); err != nil {
			s.logger.Error("Failed to scan connection quality row", zap.Error(err))
			errors = append(errors, err)
			continue
		}

		if !name.Valid || !value.Valid {
			continue
		}

		switch name.String {
		case "user commits":
			s.mb.RecordNewrelicoracledbConnectionUserCommitsDataPoint(
				timestamp,
				value.Float64,
				s.instanceName,
			)
		case "user rollbacks":
			s.mb.RecordNewrelicoracledbConnectionUserRollbacksDataPoint(
				timestamp,
				value.Float64,
				s.instanceName,
			)
		case "parse count (total)":
			s.mb.RecordNewrelicoracledbConnectionParseCountTotalDataPoint(
				timestamp,
				value.Float64,
				s.instanceName,
			)
		case "parse count (hard)":
			s.mb.RecordNewrelicoracledbConnectionParseCountHardDataPoint(
				timestamp,
				value.Float64,
				s.instanceName,
			)
		case "execute count":
			s.mb.RecordNewrelicoracledbConnectionExecuteCountDataPoint(
				timestamp,
				value.Float64,
				s.instanceName,
			)
		case "SQL*Net roundtrips to/from client":
			s.mb.RecordNewrelicoracledbConnectionSqlnetRoundtripsDataPoint(
				timestamp,
				value.Float64,
				s.instanceName,
			)
		case "bytes sent via SQL*Net to client":
			s.mb.RecordNewrelicoracledbConnectionBytesSentDataPoint(
				timestamp,
				value.Float64,
				s.instanceName,
			)
		case "bytes received via SQL*Net from client":
			s.mb.RecordNewrelicoracledbConnectionBytesReceivedDataPoint(
				timestamp,
				value.Float64,
				s.instanceName,
			)
		}
	}

	return errors
}

// scrapeSingleValue is a helper function to scrape a single numeric value
func (s *ConnectionScraper) scrapeSingleValue(ctx context.Context, query string, metricType string, timestamp pcommon.Timestamp) error {
	var value sql.NullFloat64

	if err := s.db.QueryRowContext(ctx, query).Scan(&value); err != nil {
		s.logger.Error("Failed to execute single value query", zap.String("metric_type", metricType), zap.Error(err))
		return err
	}

	if !value.Valid {
		s.logger.Debug("Null value returned for metric", zap.String("metric_type", metricType))
		return nil
	}

	switch metricType {
	case "total_sessions":
		s.mb.RecordNewrelicoracledbConnectionTotalSessionsDataPoint(timestamp, value.Float64, s.instanceName)
	case "active_sessions":
		s.mb.RecordNewrelicoracledbConnectionActiveSessionsDataPoint(timestamp, value.Float64, s.instanceName)
	case "inactive_sessions":
		s.mb.RecordNewrelicoracledbConnectionInactiveSessionsDataPoint(timestamp, value.Float64, s.instanceName)
	}

	return nil
}
