// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
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

	// Scrape global name instance metrics
	errors = append(errors, s.scrapeGlobalNameInstanceMetrics(ctx, now)...)

	// Scrape database ID instance metrics
	errors = append(errors, s.scrapeDBIDInstanceMetrics(ctx, now)...)

	// Scrape long running queries metrics
	errors = append(errors, s.scrapeLongRunningQueriesMetrics(ctx, now)...)

	// Scrape SGA UGA total memory metrics
	errors = append(errors, s.scrapeSGAUGATotalMemoryMetrics(ctx, now)...)

	// Scrape SGA shared pool library cache metrics
	errors = append(errors, s.scrapeSGASharedPoolLibraryCacheMetrics(ctx, now)...)

	// Scrape SGA shared pool library cache user metrics
	errors = append(errors, s.scrapeSGASharedPoolLibraryCacheUserMetrics(ctx, now)...)

	// Scrape SGA shared pool library cache reload ratio metrics
	errors = append(errors, s.scrapeSGASharedPoolLibraryCacheReloadRatioMetrics(ctx, now)...)

	// Scrape SGA shared pool library cache hit ratio metrics
	errors = append(errors, s.scrapeSGASharedPoolLibraryCacheHitRatioMetrics(ctx, now)...)

	// Scrape SGA shared pool dictionary cache miss ratio metrics
	errors = append(errors, s.scrapeSGASharedPoolDictCacheMissRatioMetrics(ctx, now)...)

	// Scrape SGA log buffer space waits metrics
	errors = append(errors, s.scrapeSGALogBufferSpaceWaitsMetrics(ctx, now)...)

	// Scrape SGA log allocation retries metrics
	errors = append(errors, s.scrapeSGALogAllocRetriesMetrics(ctx, now)...)

	// Scrape SGA hit ratio metrics
	errors = append(errors, s.scrapeSGAHitRatioMetrics(ctx, now)...)

	// Scrape sysstat metrics
	errors = append(errors, s.scrapeSysstatMetrics(ctx, now)...)

	// Scrape SGA metrics
	errors = append(errors, s.scrapeSGAMetrics(ctx, now)...)

	// Scrape rollback segments metrics
	errors = append(errors, s.scrapeRollbackSegmentsMetrics(ctx, now)...)

	// Scrape redo log waits metrics
	errors = append(errors, s.scrapeRedoLogWaitsMetrics(ctx, now)...)

	return errors
}

// scrapeLockedAccountsMetrics handles the locked accounts metrics
func (s *CoreScraper) scrapeLockedAccountsMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	// Check if locked accounts metric is enabled
	if !s.config.Metrics.NewrelicoracledbLockedAccounts.Enabled {
		return nil
	}

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
	// Check if any disk I/O metrics are enabled
	if !s.config.Metrics.NewrelicoracledbDiskReads.Enabled &&
		!s.config.Metrics.NewrelicoracledbDiskWrites.Enabled &&
		!s.config.Metrics.NewrelicoracledbDiskBlocksRead.Enabled &&
		!s.config.Metrics.NewrelicoracledbDiskBlocksWritten.Enabled &&
		!s.config.Metrics.NewrelicoracledbDiskReadTimeMilliseconds.Enabled &&
		!s.config.Metrics.NewrelicoracledbDiskWriteTimeMilliseconds.Enabled {
		return nil
	}

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

		// Record disk I/O metrics only if enabled
		if s.config.Metrics.NewrelicoracledbDiskReads.Enabled {
			s.mb.RecordNewrelicoracledbDiskReadsDataPoint(now, physicalReads, s.instanceName, instanceID)
		}
		if s.config.Metrics.NewrelicoracledbDiskWrites.Enabled {
			s.mb.RecordNewrelicoracledbDiskWritesDataPoint(now, physicalWrites, s.instanceName, instanceID)
		}
		if s.config.Metrics.NewrelicoracledbDiskBlocksRead.Enabled {
			s.mb.RecordNewrelicoracledbDiskBlocksReadDataPoint(now, physicalBlockReads, s.instanceName, instanceID)
		}
		if s.config.Metrics.NewrelicoracledbDiskBlocksWritten.Enabled {
			s.mb.RecordNewrelicoracledbDiskBlocksWrittenDataPoint(now, physicalBlockWrites, s.instanceName, instanceID)
		}
		if s.config.Metrics.NewrelicoracledbDiskReadTimeMilliseconds.Enabled {
			s.mb.RecordNewrelicoracledbDiskReadTimeMillisecondsDataPoint(now, readTime, s.instanceName, instanceID)
		}
		if s.config.Metrics.NewrelicoracledbDiskWriteTimeMilliseconds.Enabled {
			s.mb.RecordNewrelicoracledbDiskWriteTimeMillisecondsDataPoint(now, writeTime, s.instanceName, instanceID)
		}

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

// scrapeGlobalNameInstanceMetrics handles the global name instance metrics
func (s *CoreScraper) scrapeGlobalNameInstanceMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute global name instance metrics query
	s.logger.Debug("Executing global name instance metrics query", zap.String("sql", queries.GlobalNameInstanceSQL))

	rows, err := s.db.QueryContext(ctx, queries.GlobalNameInstanceSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing global name instance metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var instID interface{}
		var globalName string

		err := rows.Scan(&instID, &globalName)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning global name instance metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record global name instance metrics
		s.logger.Info("Global name instance metrics collected",
			zap.String("instance_id", instanceID),
			zap.String("global_name", globalName),
			zap.String("instance", s.instanceName),
		)

		// Record the global name metric (using 1 as value since it's an attribute metric)
		s.mb.RecordNewrelicoracledbGlobalNameDataPoint(now, 1, s.instanceName, instanceID, globalName)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating global name instance metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle global name instance metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeDBIDInstanceMetrics handles the database ID instance metrics
func (s *CoreScraper) scrapeDBIDInstanceMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute database ID instance metrics query
	s.logger.Debug("Executing database ID instance metrics query", zap.String("sql", queries.DBIDInstanceSQL))

	rows, err := s.db.QueryContext(ctx, queries.DBIDInstanceSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing database ID instance metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var instID interface{}
		var dbID string

		err := rows.Scan(&instID, &dbID)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning database ID instance metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record database ID instance metrics
		s.logger.Info("Database ID instance metrics collected",
			zap.String("instance_id", instanceID),
			zap.String("db_id", dbID),
			zap.String("instance", s.instanceName),
		)

		// Record the database ID metric (using 1 as value since it's an attribute metric)
		s.mb.RecordNewrelicoracledbDbIDDataPoint(now, 1, s.instanceName, instanceID, dbID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating database ID instance metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle database ID instance metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeLongRunningQueriesMetrics handles the long running queries metrics
func (s *CoreScraper) scrapeLongRunningQueriesMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute long running queries metrics query
	s.logger.Debug("Executing long running queries metrics query", zap.String("sql", queries.LongRunningQueriesSQL))

	rows, err := s.db.QueryContext(ctx, queries.LongRunningQueriesSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing long running queries metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var instID interface{}
		var total int64

		err := rows.Scan(&instID, &total)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning long running queries metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record long running queries metrics
		s.logger.Info("Long running queries metrics collected",
			zap.String("instance_id", instanceID),
			zap.Int64("long_running_queries", total),
			zap.String("instance", s.instanceName),
		)

		// Record the long running queries metric
		s.mb.RecordNewrelicoracledbLongRunningQueriesDataPoint(now, total, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating long running queries metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle long running queries metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeSGAUGATotalMemoryMetrics handles the SGA UGA total memory metrics
func (s *CoreScraper) scrapeSGAUGATotalMemoryMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute SGA UGA total memory metrics query
	s.logger.Debug("Executing SGA UGA total memory metrics query", zap.String("sql", queries.SGAUGATotalMemorySQL))

	rows, err := s.db.QueryContext(ctx, queries.SGAUGATotalMemorySQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing SGA UGA total memory metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var sum int64
		var instID interface{}

		err := rows.Scan(&sum, &instID)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning SGA UGA total memory metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record SGA UGA total memory metrics
		s.logger.Info("SGA UGA total memory metrics collected",
			zap.String("instance_id", instanceID),
			zap.Int64("sga_uga_total_bytes", sum),
			zap.String("instance", s.instanceName),
		)

		// Record the SGA UGA total memory metric
		s.mb.RecordNewrelicoracledbMemorySgaUgaTotalBytesDataPoint(now, sum, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating SGA UGA total memory metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle SGA UGA total memory metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeSGASharedPoolLibraryCacheMetrics handles the SGA shared pool library cache sharable memory metrics
func (s *CoreScraper) scrapeSGASharedPoolLibraryCacheMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute SGA shared pool library cache metrics query
	s.logger.Debug("Executing SGA shared pool library cache metrics query", zap.String("sql", queries.SGASharedPoolLibraryCacheShareableStatementSQL))

	rows, err := s.db.QueryContext(ctx, queries.SGASharedPoolLibraryCacheShareableStatementSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing SGA shared pool library cache metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var sum int64
		var instID interface{}

		err := rows.Scan(&sum, &instID)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning SGA shared pool library cache metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record SGA shared pool library cache metrics
		s.logger.Info("SGA shared pool library cache metrics collected",
			zap.String("instance_id", instanceID),
			zap.Int64("sga_shared_pool_library_cache_sharable_bytes", sum),
			zap.String("instance", s.instanceName),
		)

		// Record the SGA shared pool library cache metric
		s.mb.RecordNewrelicoracledbMemorySgaSharedPoolLibraryCacheSharableBytesDataPoint(now, sum, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating SGA shared pool library cache metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle SGA shared pool library cache metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeSGASharedPoolLibraryCacheUserMetrics handles the SGA shared pool library cache shareable user memory metrics
func (s *CoreScraper) scrapeSGASharedPoolLibraryCacheUserMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute SGA shared pool library cache user metrics query
	s.logger.Debug("Executing SGA shared pool library cache user metrics query", zap.String("sql", queries.SGASharedPoolLibraryCacheShareableUserSQL))

	rows, err := s.db.QueryContext(ctx, queries.SGASharedPoolLibraryCacheShareableUserSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing SGA shared pool library cache user metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var sum int64
		var instID interface{}

		err := rows.Scan(&sum, &instID)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning SGA shared pool library cache user metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record SGA shared pool library cache user metrics
		s.logger.Info("SGA shared pool library cache user metrics collected",
			zap.String("instance_id", instanceID),
			zap.Int64("sga_shared_pool_library_cache_user_bytes", sum),
			zap.String("instance", s.instanceName),
		)

		// Record the SGA shared pool library cache user metric
		s.mb.RecordNewrelicoracledbMemorySgaSharedPoolLibraryCacheUserBytesDataPoint(now, sum, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating SGA shared pool library cache user metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle SGA shared pool library cache user metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeSGASharedPoolLibraryCacheReloadRatioMetrics handles the SGA shared pool library cache reload ratio metrics
func (s *CoreScraper) scrapeSGASharedPoolLibraryCacheReloadRatioMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute SGA shared pool library cache reload ratio metrics query
	s.logger.Debug("Executing SGA shared pool library cache reload ratio metrics query", zap.String("sql", queries.SGASharedPoolLibraryCacheReloadRatioSQL))

	rows, err := s.db.QueryContext(ctx, queries.SGASharedPoolLibraryCacheReloadRatioSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing SGA shared pool library cache reload ratio metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var ratio float64
		var instID interface{}

		err := rows.Scan(&ratio, &instID)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning SGA shared pool library cache reload ratio metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record SGA shared pool library cache reload ratio metrics
		s.logger.Info("SGA shared pool library cache reload ratio metrics collected",
			zap.String("instance_id", instanceID),
			zap.Float64("sga_shared_pool_library_cache_reload_ratio", ratio),
			zap.String("instance", s.instanceName),
		)

		// Record the SGA shared pool library cache reload ratio metric
		s.mb.RecordNewrelicoracledbSgaSharedPoolLibraryCacheReloadRatioDataPoint(now, ratio, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating SGA shared pool library cache reload ratio metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle SGA shared pool library cache reload ratio metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeSGASharedPoolLibraryCacheHitRatioMetrics handles the SGA shared pool library cache hit ratio metrics
func (s *CoreScraper) scrapeSGASharedPoolLibraryCacheHitRatioMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute SGA shared pool library cache hit ratio metrics query
	s.logger.Debug("Executing SGA shared pool library cache hit ratio metrics query", zap.String("sql", queries.SGASharedPoolLibraryCacheHitRatioSQL))

	rows, err := s.db.QueryContext(ctx, queries.SGASharedPoolLibraryCacheHitRatioSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing SGA shared pool library cache hit ratio metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var ratio float64
		var instID interface{}

		err := rows.Scan(&ratio, &instID)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning SGA shared pool library cache hit ratio metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record SGA shared pool library cache hit ratio metrics
		s.logger.Info("SGA shared pool library cache hit ratio metrics collected",
			zap.String("instance_id", instanceID),
			zap.Float64("sga_shared_pool_library_cache_hit_ratio", ratio),
			zap.String("instance", s.instanceName),
		)

		// Record the SGA shared pool library cache hit ratio metric
		s.mb.RecordNewrelicoracledbSgaSharedPoolLibraryCacheHitRatioDataPoint(now, ratio, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating SGA shared pool library cache hit ratio metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle SGA shared pool library cache hit ratio metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeSGASharedPoolDictCacheMissRatioMetrics handles the SGA shared pool dictionary cache miss ratio metrics
func (s *CoreScraper) scrapeSGASharedPoolDictCacheMissRatioMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute SGA shared pool dictionary cache miss ratio metrics query
	s.logger.Debug("Executing SGA shared pool dictionary cache miss ratio metrics query", zap.String("sql", queries.SGASharedPoolDictCacheMissRatioSQL))

	rows, err := s.db.QueryContext(ctx, queries.SGASharedPoolDictCacheMissRatioSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing SGA shared pool dictionary cache miss ratio metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var ratio float64
		var instID interface{}

		err := rows.Scan(&ratio, &instID)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning SGA shared pool dictionary cache miss ratio metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record SGA shared pool dictionary cache miss ratio metrics
		s.logger.Info("SGA shared pool dictionary cache miss ratio metrics collected",
			zap.String("instance_id", instanceID),
			zap.Float64("sga_shared_pool_dict_cache_miss_ratio", ratio),
			zap.String("instance", s.instanceName),
		)

		// Record the SGA shared pool dictionary cache miss ratio metric
		s.mb.RecordNewrelicoracledbSgaSharedPoolDictCacheMissRatioDataPoint(now, ratio, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating SGA shared pool dictionary cache miss ratio metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle SGA shared pool dictionary cache miss ratio metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeSGALogBufferSpaceWaitsMetrics handles the SGA log buffer space waits metrics
func (s *CoreScraper) scrapeSGALogBufferSpaceWaitsMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error

	// Execute SGA log buffer space waits metrics query
	s.logger.Debug("Executing SGA log buffer space waits metrics query", zap.String("sql", queries.SGALogBufferSpaceWaitsSQL))

	rows, err := s.db.QueryContext(ctx, queries.SGALogBufferSpaceWaitsSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing SGA log buffer space waits metrics query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process each row and record metrics
	metricCount := 0
	for rows.Next() {
		var count int64
		var instID interface{}

		err := rows.Scan(&count, &instID)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning SGA log buffer space waits metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Record SGA log buffer space waits metrics
		s.logger.Info("SGA log buffer space waits metrics collected",
			zap.String("instance_id", instanceID),
			zap.Int64("sga_log_buffer_space_waits", count),
			zap.String("instance", s.instanceName),
		)

		// Record the SGA log buffer space waits metric
		s.mb.RecordNewrelicoracledbSgaLogBufferSpaceWaitsDataPoint(now, count, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating SGA log buffer space waits metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle SGA log buffer space waits metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeSGALogAllocRetriesMetrics handles the SGA log allocation retries metrics
func (s *CoreScraper) scrapeSGALogAllocRetriesMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error
	metricCount := 0

	// Execute SGA log allocation retries query
	s.logger.Debug("Executing SGA log allocation retries query", zap.String("sql", queries.SGALogAllocRetriesSQL))
	rows, err := s.db.QueryContext(ctx, queries.SGALogAllocRetriesSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing SGA log allocation retries query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process query results
	for rows.Next() {
		var ratio sql.NullFloat64
		var instID interface{}

		err := rows.Scan(&ratio, &instID)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning SGA log allocation retries metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Handle NULL ratio values
		ratioValue := 0.0
		if ratio.Valid {
			ratioValue = ratio.Float64
		}

		// Record SGA log allocation retries metrics
		s.logger.Info("SGA log allocation retries metrics collected",
			zap.String("instance_id", instanceID),
			zap.Float64("sga_log_allocation_retries_ratio", ratioValue),
			zap.String("instance", s.instanceName),
		)

		// Record the SGA log allocation retries metric
		s.mb.RecordNewrelicoracledbSgaLogAllocationRetriesRatioDataPoint(now, ratioValue, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating SGA log allocation retries metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle SGA log allocation retries metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeSGAHitRatioMetrics handles the SGA hit ratio metrics
func (s *CoreScraper) scrapeSGAHitRatioMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error
	metricCount := 0

	// Execute SGA hit ratio query
	s.logger.Debug("Executing SGA hit ratio query", zap.String("sql", queries.SGAHitRatioSQL))
	rows, err := s.db.QueryContext(ctx, queries.SGAHitRatioSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing SGA hit ratio query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process query results
	for rows.Next() {
		var instID interface{}
		var ratio sql.NullFloat64

		err := rows.Scan(&instID, &ratio)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning SGA hit ratio metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Handle NULL ratio values
		ratioValue := 0.0
		if ratio.Valid {
			ratioValue = ratio.Float64
		}

		// Record SGA hit ratio metrics
		s.logger.Info("SGA hit ratio metrics collected",
			zap.String("instance_id", instanceID),
			zap.Float64("sga_hit_ratio", ratioValue),
			zap.String("instance", s.instanceName),
		)

		// Record the SGA hit ratio metric
		s.mb.RecordNewrelicoracledbSgaHitRatioDataPoint(now, ratioValue, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating SGA hit ratio metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle SGA hit ratio metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeSysstatMetrics handles the sysstat metrics
func (s *CoreScraper) scrapeSysstatMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error
	metricCount := 0

	// Execute sysstat query
	s.logger.Debug("Executing sysstat query", zap.String("sql", queries.SysstatSQL))
	rows, err := s.db.QueryContext(ctx, queries.SysstatSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing sysstat query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process query results
	for rows.Next() {
		var instID interface{}
		var name string
		var value sql.NullInt64

		err := rows.Scan(&instID, &name, &value)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning sysstat metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Handle NULL values
		valueInt := int64(0)
		if value.Valid {
			valueInt = value.Int64
		}

		// Record appropriate metric based on the name
		switch name {
		case "redo buffer allocation retries":
			s.logger.Info("SGA log buffer redo allocation retries metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("sga_log_buffer_redo_allocation_retries", valueInt),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbSgaLogBufferRedoAllocationRetriesDataPoint(now, valueInt, s.instanceName, instanceID)

		case "redo entries":
			s.logger.Info("SGA log buffer redo entries metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("sga_log_buffer_redo_entries", valueInt),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbSgaLogBufferRedoEntriesDataPoint(now, valueInt, s.instanceName, instanceID)

		case "sorts (memory)":
			s.logger.Info("Sorts memory metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("sorts_memory", valueInt),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbSortsMemoryDataPoint(now, valueInt, s.instanceName, instanceID)

		case "sorts (disk)":
			s.logger.Info("Sorts disk metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("sorts_disk", valueInt),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbSortsDiskDataPoint(now, valueInt, s.instanceName, instanceID)

		default:
			s.logger.Warn("Unknown sysstat metric name", zap.String("name", name), zap.String("instance_id", instanceID))
		}

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating sysstat metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle sysstat metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeSGAMetrics handles the SGA metrics
func (s *CoreScraper) scrapeSGAMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error
	metricCount := 0

	// Execute SGA query
	s.logger.Debug("Executing SGA query", zap.String("sql", queries.SGASQL))
	rows, err := s.db.QueryContext(ctx, queries.SGASQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing SGA query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process query results
	for rows.Next() {
		var instID interface{}
		var name string
		var value sql.NullInt64

		err := rows.Scan(&instID, &name, &value)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning SGA metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Handle NULL values
		valueInt := int64(0)
		if value.Valid {
			valueInt = value.Int64
		}

		// Record appropriate metric based on the name
		switch name {
		case "Fixed Size":
			s.logger.Info("SGA fixed size metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("sga_fixed_size_bytes", valueInt),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbSgaFixedSizeBytesDataPoint(now, valueInt, s.instanceName, instanceID)

		case "Redo Buffers":
			s.logger.Info("SGA redo buffers metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("sga_redo_buffers_bytes", valueInt),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbSgaRedoBuffersBytesDataPoint(now, valueInt, s.instanceName, instanceID)

		default:
			s.logger.Warn("Unknown SGA metric name", zap.String("name", name), zap.String("instance_id", instanceID))
		}

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating SGA metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle SGA metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeRollbackSegmentsMetrics handles the rollback segments metrics
func (s *CoreScraper) scrapeRollbackSegmentsMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error
	metricCount := 0

	// Execute rollback segments query
	s.logger.Debug("Executing rollback segments query", zap.String("sql", queries.RollbackSegmentsSQL))
	rows, err := s.db.QueryContext(ctx, queries.RollbackSegmentsSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing rollback segments query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process query results
	for rows.Next() {
		var gets sql.NullInt64
		var waits sql.NullInt64
		var ratio sql.NullFloat64
		var instID interface{}

		err := rows.Scan(&gets, &waits, &ratio, &instID)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning rollback segments metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Handle NULL values
		getsValue := int64(0)
		if gets.Valid {
			getsValue = gets.Int64
		}

		waitsValue := int64(0)
		if waits.Valid {
			waitsValue = waits.Int64
		}

		ratioValue := 0.0
		if ratio.Valid {
			ratioValue = ratio.Float64
		}

		// Record rollback segments metrics
		s.logger.Info("Rollback segments metrics collected",
			zap.String("instance_id", instanceID),
			zap.Int64("rollback_segments_gets", getsValue),
			zap.Int64("rollback_segments_waits", waitsValue),
			zap.Float64("rollback_segments_wait_ratio", ratioValue),
			zap.String("instance", s.instanceName),
		)

		// Record all rollback segments metrics
		s.mb.RecordNewrelicoracledbRollbackSegmentsGetsDataPoint(now, getsValue, s.instanceName, instanceID)
		s.mb.RecordNewrelicoracledbRollbackSegmentsWaitsDataPoint(now, waitsValue, s.instanceName, instanceID)
		s.mb.RecordNewrelicoracledbRollbackSegmentsWaitRatioDataPoint(now, ratioValue, s.instanceName, instanceID)

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating rollback segments metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle rollback segments metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

	return errors
}

// scrapeRedoLogWaitsMetrics handles the redo log waits metrics
func (s *CoreScraper) scrapeRedoLogWaitsMetrics(ctx context.Context, now pcommon.Timestamp) []error {
	var errors []error
	metricCount := 0

	// Execute redo log waits query
	s.logger.Debug("Executing redo log waits query", zap.String("sql", queries.RedoLogWaitsSQL))
	rows, err := s.db.QueryContext(ctx, queries.RedoLogWaitsSQL)
	if err != nil {
		errors = append(errors, fmt.Errorf("error executing redo log waits query: %w", err))
		return errors
	}
	defer rows.Close()

	// Process query results
	for rows.Next() {
		var totalWaits sql.NullInt64
		var instID interface{}
		var event string

		err := rows.Scan(&totalWaits, &instID, &event)
		if err != nil {
			errors = append(errors, fmt.Errorf("error scanning redo log waits metrics row: %w", err))
			continue
		}

		// Convert instance ID to string
		instanceID := getInstanceIDString(instID)

		// Handle NULL values
		waitsValue := int64(0)
		if totalWaits.Valid {
			waitsValue = totalWaits.Int64
		}

		// Record appropriate metric based on the event name
		switch {
		case strings.Contains(event, "log file parallel write"):
			s.logger.Info("Redo log parallel write waits metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("redo_log_parallel_write_waits", waitsValue),
				zap.String("event", event),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbRedoLogParallelWriteWaitsDataPoint(now, waitsValue, s.instanceName, instanceID)

		case strings.Contains(event, "log file switch completion"):
			s.logger.Info("Redo log switch completion waits metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("redo_log_switch_completion_waits", waitsValue),
				zap.String("event", event),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbRedoLogSwitchCompletionWaitsDataPoint(now, waitsValue, s.instanceName, instanceID)

		case strings.Contains(event, "log file switch (check"):
			s.logger.Info("Redo log switch checkpoint incomplete waits metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("redo_log_switch_checkpoint_incomplete_waits", waitsValue),
				zap.String("event", event),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbRedoLogSwitchCheckpointIncompleteWaitsDataPoint(now, waitsValue, s.instanceName, instanceID)

		case strings.Contains(event, "log file switch (arch"):
			s.logger.Info("Redo log switch archiving needed waits metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("redo_log_switch_archiving_needed_waits", waitsValue),
				zap.String("event", event),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbRedoLogSwitchArchivingNeededWaitsDataPoint(now, waitsValue, s.instanceName, instanceID)

		case strings.Contains(event, "buffer busy waits"):
			s.logger.Info("SGA buffer busy waits metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("sga_buffer_busy_waits", waitsValue),
				zap.String("event", event),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbSgaBufferBusyWaitsDataPoint(now, waitsValue, s.instanceName, instanceID)

		case strings.Contains(event, "freeBufferWaits"):
			s.logger.Info("SGA free buffer waits metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("sga_free_buffer_waits", waitsValue),
				zap.String("event", event),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbSgaFreeBufferWaitsDataPoint(now, waitsValue, s.instanceName, instanceID)

		case strings.Contains(event, "free buffer inspected"):
			s.logger.Info("SGA free buffer inspected waits metrics collected",
				zap.String("instance_id", instanceID),
				zap.Int64("sga_free_buffer_inspected_waits", waitsValue),
				zap.String("event", event),
				zap.String("instance", s.instanceName),
			)
			s.mb.RecordNewrelicoracledbSgaFreeBufferInspectedWaitsDataPoint(now, waitsValue, s.instanceName, instanceID)

		default:
			s.logger.Warn("Unknown redo log waits event", zap.String("event", event), zap.String("instance_id", instanceID))
		}

		metricCount++
	}

	if err = rows.Err(); err != nil {
		errors = append(errors, fmt.Errorf("error iterating redo log waits metrics rows: %w", err))
	}

	s.logger.Debug("Collected Oracle redo log waits metrics", zap.Int("metric_count", metricCount), zap.String("instance", s.instanceName))

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
