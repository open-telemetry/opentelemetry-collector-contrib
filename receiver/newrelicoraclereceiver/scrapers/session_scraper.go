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
	// Session-related SQL queries
	sessionCountSQL     = "SELECT COUNT(*) as SESSION_COUNT FROM v$session WHERE type = 'USER'"
	activeSessionSQL    = "SELECT COUNT(*) as ACTIVE_SESSIONS FROM v$session WHERE status = 'ACTIVE'"
	inactiveSessionSQL  = "SELECT COUNT(*) as INACTIVE_SESSIONS FROM v$session WHERE status = 'INACTIVE'"
	blockedSessionSQL   = "SELECT COUNT(*) as BLOCKED_SESSIONS FROM v$session WHERE blocking_session IS NOT NULL"
)

// scrapeSessionCount collects Oracle session count metrics
func (s *newRelicOracleScraper) scrapeSessionCount(ctx context.Context) []error {
	var errors []error
	
	if !s.metricsBuilderConfig.Metrics.NewrelicoracledbSessionsCount.Enabled {
		return errors
	}

	s.logger.Debug("Scraping Oracle session count")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.sessionCountClient.metricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing session count query: %w", err))
		return errors
	}

	for _, row := range rows {
		sessionCountStr := row["SESSION_COUNT"]
		sessionCount, parseErr := strconv.ParseInt(sessionCountStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse session count value: %s, error: %w", sessionCountStr, parseErr))
			continue
		}
		
		s.mb.RecordNewrelicoracledbSessionsCountDataPoint(now, sessionCount, s.instanceName)
		s.logger.Debug("Collected Oracle session count", zap.Int64("count", sessionCount))
	}
	
	return errors
}

// scrapeActiveSessionCount collects Oracle active session count metrics
func (s *newRelicOracleScraper) scrapeActiveSessionCount(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle active session count")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.sessionCountClient.metricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing active session count query: %w", err))
		return errors
	}

	for _, row := range rows {
		activeSessionsStr := row["ACTIVE_SESSIONS"]
		activeSessions, parseErr := strconv.ParseInt(activeSessionsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse active sessions value: %s, error: %w", activeSessionsStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle active sessions", zap.Int64("active_count", activeSessions))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbActiveSessionsCountDataPoint(now, activeSessions, s.instanceName)
	}
	
	return errors
}

// scrapeInactiveSessionCount collects Oracle inactive session count metrics
func (s *newRelicOracleScraper) scrapeInactiveSessionCount(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle inactive session count")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.sessionCountClient.metricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing inactive session count query: %w", err))
		return errors
	}

	for _, row := range rows {
		inactiveSessionsStr := row["INACTIVE_SESSIONS"]
		inactiveSessions, parseErr := strconv.ParseInt(inactiveSessionsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse inactive sessions value: %s, error: %w", inactiveSessionsStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle inactive sessions", zap.Int64("inactive_count", inactiveSessions))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbInactiveSessionsCountDataPoint(now, inactiveSessions, s.instanceName)
	}
	
	return errors
}

// scrapeBlockedSessionCount collects Oracle blocked session count metrics
func (s *newRelicOracleScraper) scrapeBlockedSessionCount(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle blocked session count")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.sessionCountClient.metricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing blocked session count query: %w", err))
		return errors
	}

	for _, row := range rows {
		blockedSessionsStr := row["BLOCKED_SESSIONS"]
		blockedSessions, parseErr := strconv.ParseInt(blockedSessionsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse blocked sessions value: %s, error: %w", blockedSessionsStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle blocked sessions", zap.Int64("blocked_count", blockedSessions))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbBlockedSessionsCountDataPoint(now, blockedSessions, s.instanceName)
	}
	
	return errors
}
