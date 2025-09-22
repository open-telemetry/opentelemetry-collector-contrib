// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver"

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

const (
	// Database-related SQL queries
	databaseSizeSQL     = "SELECT SUM(bytes) as DB_SIZE FROM dba_data_files"
	userCountSQL        = "SELECT COUNT(*) as USER_COUNT FROM dba_users WHERE account_status = 'OPEN'"
	lockCountSQL        = "SELECT COUNT(*) as LOCK_COUNT FROM v$lock WHERE type IN ('TX', 'TM')"
	archiveLogSQL       = "SELECT COUNT(*) as ARCHIVE_LOGS FROM v$archived_log WHERE first_time >= SYSDATE - 1"
	invalidObjectsSQL   = "SELECT COUNT(*) as INVALID_OBJECTS FROM dba_objects WHERE status = 'INVALID'"
)

// scrapeDatabaseSize collects Oracle database size metrics
func (s *newRelicOracleScraper) scrapeDatabaseSize(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle database size")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.tablespaceClient.metricRows(ctx) // Reusing tablespace client
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing database size query: %w", err))
		return errors
	}

	for _, row := range rows {
		dbSizeStr := row["DB_SIZE"]
		dbSize, parseErr := strconv.ParseInt(dbSizeStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse database size value: %s, error: %w", dbSizeStr, parseErr))
			continue
		}
		
		// Convert bytes to GB for easier reading
		dbSizeGB := float64(dbSize) / (1024 * 1024 * 1024)
		
		s.logger.Debug("Collected Oracle database size", 
			zap.Int64("db_size_bytes", dbSize),
			zap.Float64("db_size_gb", dbSizeGB))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbDatabaseSizeDataPoint(now, dbSize, s.instanceName)
	}
	
	return errors
}

// scrapeUserCount collects Oracle user count metrics
func (s *newRelicOracleScraper) scrapeUserCount(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle user count")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.sessionCountClient.metricRows(ctx) // Reusing session client
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing user count query: %w", err))
		return errors
	}

	for _, row := range rows {
		userCountStr := row["USER_COUNT"]
		userCount, parseErr := strconv.ParseInt(userCountStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse user count value: %s, error: %w", userCountStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle user count", zap.Int64("user_count", userCount))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbUserCountDataPoint(now, userCount, s.instanceName)
	}
	
	return errors
}

// scrapeLockCount collects Oracle lock count metrics
func (s *newRelicOracleScraper) scrapeLockCount(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle lock count")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.sessionCountClient.metricRows(ctx) // Reusing session client
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing lock count query: %w", err))
		return errors
	}

	for _, row := range rows {
		lockCountStr := row["LOCK_COUNT"]
		lockCount, parseErr := strconv.ParseInt(lockCountStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse lock count value: %s, error: %w", lockCountStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle lock count", zap.Int64("lock_count", lockCount))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbLockCountDataPoint(now, lockCount, s.instanceName)
	}
	
	return errors
}

// scrapeArchiveLogCount collects Oracle archive log count metrics
func (s *newRelicOracleScraper) scrapeArchiveLogCount(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle archive log count")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.sessionCountClient.metricRows(ctx) // Reusing session client
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing archive log count query: %w", err))
		return errors
	}

	for _, row := range rows {
		archiveLogsStr := row["ARCHIVE_LOGS"]
		archiveLogs, parseErr := strconv.ParseInt(archiveLogsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse archive logs value: %s, error: %w", archiveLogsStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle archive logs count", zap.Int64("archive_logs", archiveLogs))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbArchiveLogCountDataPoint(now, archiveLogs, s.instanceName)
	}
	
	return errors
}

// scrapeInvalidObjectsCount collects Oracle invalid objects count metrics
func (s *newRelicOracleScraper) scrapeInvalidObjectsCount(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle invalid objects count")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.sessionCountClient.metricRows(ctx) // Reusing session client
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing invalid objects count query: %w", err))
		return errors
	}

	for _, row := range rows {
		invalidObjectsStr := row["INVALID_OBJECTS"]
		invalidObjects, parseErr := strconv.ParseInt(invalidObjectsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse invalid objects value: %s, error: %w", invalidObjectsStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle invalid objects count", zap.Int64("invalid_objects", invalidObjects))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbInvalidObjectsCountDataPoint(now, invalidObjects, s.instanceName)
	}
	
	return errors
}
