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
	// Tablespace-related SQL queries
	tablespaceSQL        = "SELECT tablespace_name, bytes, maxbytes FROM dba_data_files"
	tablespaceUsageSQL   = "SELECT tablespace_name, used_space, tablespace_size FROM dba_tablespace_usage_metrics"
	tempTablespaceSQL    = "SELECT tablespace_name, bytes_used, bytes_free FROM v$temp_space_header"
)

// scrapeTablespaceMetrics collects Oracle tablespace metrics
func (s *newRelicOracleScraper) scrapeTablespaceMetrics(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle tablespace metrics")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.tablespaceClient.metricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing tablespace query: %w", err))
		return errors
	}

	for _, row := range rows {
		tablespaceName := row["TABLESPACE_NAME"]
		bytesStr := row["BYTES"]
		maxBytesStr := row["MAXBYTES"]
		
		// Parse bytes values
		bytes, parseErr := strconv.ParseInt(bytesStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse bytes value for tablespace %s: %w", tablespaceName, parseErr))
			continue
		}
		
		maxBytes, parseErr := strconv.ParseInt(maxBytesStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse max_bytes value for tablespace %s: %w", tablespaceName, parseErr))
			continue
		}
		
		s.logger.Debug("Collected tablespace metrics", 
			zap.String("tablespace", tablespaceName), 
			zap.Int64("bytes", bytes),
			zap.Int64("max_bytes", maxBytes))
		
		// Note: You would need to add these metrics to metadata.yaml to record them
		// s.mb.RecordNewrelicoracledbTablespaceBytesDataPoint(now, bytes, tablespaceName, s.instanceName)
		// s.mb.RecordNewrelicoracledbTablespaceMaxBytesDataPoint(now, maxBytes, tablespaceName, s.instanceName)
	}
	
	return errors
}

// scrapeTablespaceUsageMetrics collects Oracle tablespace usage metrics
func (s *newRelicOracleScraper) scrapeTablespaceUsageMetrics(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle tablespace usage metrics")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.tablespaceClient.metricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing tablespace usage query: %w", err))
		return errors
	}

	for _, row := range rows {
		tablespaceName := row["TABLESPACE_NAME"]
		usedSpaceStr := row["USED_SPACE"]
		tablespaceSizeStr := row["TABLESPACE_SIZE"]
		
		usedSpace, parseErr := strconv.ParseInt(usedSpaceStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse used_space for tablespace %s: %w", tablespaceName, parseErr))
			continue
		}
		
		tablespaceSize, parseErr := strconv.ParseInt(tablespaceSizeStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse tablespace_size for tablespace %s: %w", tablespaceName, parseErr))
			continue
		}
		
		// Calculate usage percentage
		usagePercent := float64(0)
		if tablespaceSize > 0 {
			usagePercent = (float64(usedSpace) / float64(tablespaceSize)) * 100
		}
		
		s.logger.Debug("Collected tablespace usage metrics", 
			zap.String("tablespace", tablespaceName), 
			zap.Int64("used_space", usedSpace),
			zap.Int64("tablespace_size", tablespaceSize),
			zap.Float64("usage_percent", usagePercent))
		
		// Note: You would need to add these metrics to metadata.yaml to record them
		// s.mb.RecordNewrelicoracledbTablespaceUsedSpaceDataPoint(now, usedSpace, tablespaceName, s.instanceName)
		// s.mb.RecordNewrelicoracledbTablespaceUsagePercentDataPoint(now, usagePercent, tablespaceName, s.instanceName)
	}
	
	return errors
}

// scrapeTempTablespaceMetrics collects Oracle temporary tablespace metrics
func (s *newRelicOracleScraper) scrapeTempTablespaceMetrics(ctx context.Context) []error {
	var errors []error
	
	s.logger.Debug("Scraping Oracle temporary tablespace metrics")
	now := pcommon.NewTimestampFromTime(time.Now())
	
	rows, err := s.tablespaceClient.metricRows(ctx)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing temp tablespace query: %w", err))
		return errors
	}

	for _, row := range rows {
		tablespaceName := row["TABLESPACE_NAME"]
		bytesUsedStr := row["BYTES_USED"]
		bytesFreeStr := row["BYTES_FREE"]
		
		bytesUsed, parseErr := strconv.ParseInt(bytesUsedStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse bytes_used for temp tablespace %s: %w", tablespaceName, parseErr))
			continue
		}
		
		bytesFree, parseErr := strconv.ParseInt(bytesFreeStr, 10, 64)
		if parseErr != nil {
			errors = append(errors, fmt.Errorf("failed to parse bytes_free for temp tablespace %s: %w", tablespaceName, parseErr))
			continue
		}
		
		totalBytes := bytesUsed + bytesFree
		
		s.logger.Debug("Collected temp tablespace metrics", 
			zap.String("tablespace", tablespaceName), 
			zap.Int64("bytes_used", bytesUsed),
			zap.Int64("bytes_free", bytesFree),
			zap.Int64("total_bytes", totalBytes))
		
		// Note: You would need to add these metrics to metadata.yaml to record them
		// s.mb.RecordNewrelicoracledbTempTablespaceBytesUsedDataPoint(now, bytesUsed, tablespaceName, s.instanceName)
		// s.mb.RecordNewrelicoracledbTempTablespaceBytesFreeDataPoint(now, bytesFree, tablespaceName, s.instanceName)
	}
	
	return errors
}
