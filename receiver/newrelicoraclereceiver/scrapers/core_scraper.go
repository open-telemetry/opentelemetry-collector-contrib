// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// CoreScraper handles Oracle core database metrics
type CoreScraper struct {
	db           *sql.DB
	mb           *metadata.MetricsBuilder
	logger       *zap.Logger
	instanceName string
	config       metadata.MetricsBuilderConfig
}

// NewCoreScraper creates a new core scraper
func NewCoreScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, config metadata.MetricsBuilderConfig) *CoreScraper {
	return &CoreScraper{
		db:           db,
		mb:           mb,
		logger:       logger,
		instanceName: instanceName,
		config:       config,
	}
}

// ScrapeCoreMetrics collects Oracle core database metrics
func (s *CoreScraper) ScrapeCoreMetrics(ctx context.Context) []error {
	var errors []error

	s.logger.Debug("Scraping Oracle core database metrics")
	now := pcommon.NewTimestampFromTime(time.Now())

	// Scrape locked accounts metrics
	errors = append(errors, s.scrapeLockedAccountsMetrics(ctx, now)...)

	// Scrape read/write disk I/O metrics
	errors = append(errors, s.scrapeReadWriteMetrics(ctx, now)...)

	// Scrape PGA memory metrics
	errors = append(errors, s.scrapePGAMetrics(ctx, now)...)

	return errors
}

// scrapeLockedAccountsMetrics handles the locked accounts metrics
func (s *CoreScraper) scrapeLockedAccountsMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute locked accounts query
	s.logger.Debug("Executing locked accounts query", zap.String("sql", queries.LockedAccountsSQL))

	rows, err := s.db.QueryContext(ctx, queries.LockedAccountsSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing locked accounts query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var instID interface{}
		var lockedAccounts int64

		err := rows.Scan(&instID, &lockedAccounts)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning locked accounts row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record locked accounts metrics
		s.logger.Info("Locked accounts metrics collected",
			zap.String("instance_id", instanceID),
			zap.Int64("locked_accounts", lockedAccounts),
			zap.String("instance", s.instanceName),
		)

		// Record the locked accounts metric
		s.mb.RecordNewrelicoracledbLockedAccountsDataPoint(now, lockedAccounts, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating locked accounts rows: %w", err))
	}

	s.logger.Debug("Collected Oracle locked accounts metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeReadWriteMetrics handles the disk read/write I/O metrics
func (s *CoreScraper) scrapeReadWriteMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute read/write metrics query
	s.logger.Debug("Executing read/write metrics query", zap.String("sql", queries.ReadWriteMetricsSQL))

	rows, err := s.db.QueryContext(ctx, queries.ReadWriteMetricsSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing read/write metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var instID interface{}
		var physicalReads, physicalWrites, physicalBlockReads, physicalBlockWrites, readTime, writeTime int64

		err := rows.Scan(&instID, &physicalReads, &physicalWrites, &physicalBlockReads, &physicalBlockWrites, &readTime, &writeTime)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning read/write metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record disk I/O metrics
		s.logger.Info("Disk I/O metrics collected",
			zap.String("instance_id", instanceID),
			zap.Int64("physical_reads", physicalReads),
			zap.Int64("physical_writes", physicalWrites),
			zap.Int64("physical_block_reads", physicalBlockReads),
			zap.Int64("physical_block_writes", physicalBlockWrites),
			zap.Int64("read_time_ms", readTime),
			zap.Int64("write_time_ms", writeTime),
			zap.String("instance", s.instanceName),
		)

		// Record all disk I/O metrics
		s.mb.RecordNewrelicoracledbDiskReadsDataPoint(now, physicalReads, s.instanceName, instanceID)
		s.mb.RecordNewrelicoracledbDiskWritesDataPoint(now, physicalWrites, s.instanceName, instanceID)
		s.mb.RecordNewrelicoracledbDiskBlocksReadDataPoint(now, physicalBlockReads, s.instanceName, instanceID)
		s.mb.RecordNewrelicoracledbDiskBlocksWrittenDataPoint(now, physicalBlockWrites, s.instanceName, instanceID)
		s.mb.RecordNewrelicoracledbDiskReadTimeMillisecondsDataPoint(now, readTime, s.instanceName, instanceID)
		s.mb.RecordNewrelicoracledbDiskWriteTimeMillisecondsDataPoint(now, writeTime, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating read/write metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle disk I/O metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapePGAMetrics handles the PGA memory metrics
func (s *CoreScraper) scrapePGAMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute PGA metrics query
	s.logger.Debug("Executing PGA metrics query", zap.String("sql", queries.PGAMetricsSQL))

	rows, err := s.db.QueryContext(ctx, queries.PGAMetricsSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing PGA metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Track metrics by instance ID to collect all PGA metrics for each instance
	instanceMetrics := make(map[string]map[string]int64)

	// Process each row and collect metrics
	for rows.Next() {
		var instID interface{}
		var name string
		var value float64

		err := rows.Scan(&instID, &name, &value)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning PGA metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Initialize the instance metrics map if needed
		if instanceMetrics[instanceID] == nil {
			instanceMetrics[instanceID] = make(map[string]int64)
		}

		// Store the metric value
		instanceMetrics[instanceID][name] = int64(value)
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating PGA metrics rows: %w", err))
	}

	// Record metrics for each instance
	metricCount := 0
	for instanceID, metrics := range instanceMetrics {
		// Record PGA memory metrics
		s.logger.Info("PGA memory metrics collected",
			zap.String("instance_id", instanceID),
			zap.Int64("pga_in_use", metrics["total PGA inuse"]),
			zap.Int64("pga_allocated", metrics["total PGA allocated"]),
			zap.Int64("pga_freeable", metrics["total freeable PGA memory"]),
			zap.Int64("pga_max_size", metrics["global memory bound"]),
			zap.String("instance", s.instanceName),
		)

		// Record each PGA metric if it exists
		if val, exists := metrics["total PGA inuse"]; exists {
			s.mb.RecordNewrelicoracledbMemoryPgaInUseBytesDataPoint(now, val, s.instanceName, instanceID)
		}
		if val, exists := metrics["total PGA allocated"]; exists {
			s.mb.RecordNewrelicoracledbMemoryPgaAllocatedBytesDataPoint(now, val, s.instanceName, instanceID)
		}
		if val, exists := metrics["total freeable PGA memory"]; exists {
			s.mb.RecordNewrelicoracledbMemoryPgaFreeableBytesDataPoint(now, val, s.instanceName, instanceID)
		}
		if val, exists := metrics["global memory bound"]; exists {
			s.mb.RecordNewrelicoracledbMemoryPgaMaxSizeBytesDataPoint(now, val, s.instanceName, instanceID)
		}

		metricCount++
	}

	s.logger.Debug("Collected Oracle PGA memory metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// getInstanceIDString converts instance ID interface to string
func getInstanceIDString(instID interface{}) string {
	if instID == nil {
		return "unknown"
	}

	switch v := instID.(type) {
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
