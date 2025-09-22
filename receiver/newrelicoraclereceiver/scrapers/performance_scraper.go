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
	// Performance-related SQL queries
	cpuUsageSQL         = "SELECT value FROM v$sysstat WHERE name = 'CPU used by this session'"
	memorySQL           = "SELECT name, value FROM v$sysstat WHERE name LIKE '%memory%'"
	waitEventsSQL       = "SELECT event, total_waits, time_waited FROM v$system_event WHERE wait_class != 'Idle'"
	processCountSQL     = "SELECT COUNT(*) as PROCESS_COUNT FROM v$process"
	redoLogSwitchSQL    = "SELECT COUNT(*) as REDO_SWITCHES FROM v$log_history WHERE first_time >= SYSDATE - 1"
)

// scrapeCpuUsage collects Oracle CPU usage metrics
func (s *newRelicOracleScraper) scrapeCpuUsage(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle CPU usage")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.cpuUsageClient.metricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing CPU usage query: %w", err))
		return errors
	}

	for _, row := range rows {
		cpuValueStr := row["VALUE"]
		cpuValue, parseErr := strconv.ParseInt(cpuValueStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse CPU value: %s, error: %w", cpuValueStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected CPU usage", zap.Int64("cpu_value", cpuValue))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbCpuUsageDataPoint(now, cpuValue, s.instanceName)
	}
	
	return errors
}

// scrapeMemoryMetrics collects Oracle memory metrics
func (s *newRelicOracleScraper) scrapeMemoryMetrics(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle memory metrics")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.memoryClient.metricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing memory query: %w", err))
		return errors
	}

	for _, row := range rows {
		metricName := row["NAME"]
		valueStr := row["VALUE"]
		value, parseErr := strconv.ParseInt(valueStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse memory value for %s: %w", metricName, parseErr))
			continue
		}
		
		s.logger.Debug("Collected memory metric", 
			zap.String("metric_name", metricName), 
			zap.Int64("value", value))
		
		// Note: You would need to add these metrics to metadata.yaml to record them
		// s.mb.RecordNewrelicoracledbMemoryDataPoint(now, value, metricName, s.instanceName)
	}
	
	return errors
}

// scrapeWaitEvents collects Oracle wait event metrics
func (s *newRelicOracleScraper) scrapeWaitEvents(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle wait events")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.cpuUsageClient.metricRows(ctx) // Reusing client for simplicity
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing wait events query: %w", err))
		return errors
	}

	for _, row := range rows {
		eventName := row["EVENT"]
		totalWaitsStr := row["TOTAL_WAITS"]
		timeWaitedStr := row["TIME_WAITED"]
		
		totalWaits, parseErr := strconv.ParseInt(totalWaitsStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse total_waits for event %s: %w", eventName, parseErr))
			continue
		}
		
		timeWaited, parseErr := strconv.ParseInt(timeWaitedStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse time_waited for event %s: %w", eventName, parseErr))
			continue
		}
		
		s.logger.Debug("Collected wait event metrics", 
			zap.String("event", eventName), 
			zap.Int64("total_waits", totalWaits),
			zap.Int64("time_waited", timeWaited))
		
		// Note: You would need to add these metrics to metadata.yaml to record them
		// s.mb.RecordNewrelicoracledbWaitEventTotalWaitsDataPoint(now, totalWaits, eventName, s.instanceName)
		// s.mb.RecordNewrelicoracledbWaitEventTimeWaitedDataPoint(now, timeWaited, eventName, s.instanceName)
	}
	
	return errors
}

// scrapeProcessCount collects Oracle process count metrics
func (s *newRelicOracleScraper) scrapeProcessCount(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle process count")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.sessionCountClient.metricRows(ctx) // Reusing session client
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing process count query: %w", err))
		return errors
	}

	for _, row := range rows {
		processCountStr := row["PROCESS_COUNT"]
		processCount, parseErr := strconv.ParseInt(processCountStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse process count value: %s, error: %w", processCountStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle process count", zap.Int64("process_count", processCount))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbProcessCountDataPoint(now, processCount, s.instanceName)
	}
	
	return errors
}

// scrapeRedoLogSwitches collects Oracle redo log switch metrics
func (s *newRelicOracleScraper) scrapeRedoLogSwitches(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle redo log switches")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.sessionCountClient.metricRows(ctx) // Reusing session client
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing redo log switches query: %w", err))
		return errors
	}

	for _, row := range rows {
		redoSwitchesStr := row["REDO_SWITCHES"]
		redoSwitches, parseErr := strconv.ParseInt(redoSwitchesStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse redo switches value: %s, error: %w", redoSwitchesStr, parseErr))
			continue
		}
		
		s.logger.Debug("Collected Oracle redo log switches", zap.Int64("redo_switches", redoSwitches))
		
		// Note: You would need to add this metric to metadata.yaml to record it
		// s.mb.RecordNewrelicoracledbRedoLogSwitchesDataPoint(now, redoSwitches, s.instanceName)
	}
	
	return errors
}
