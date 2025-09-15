// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoracledbreceiver

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoracledbreceiver/internal/metadata"
)

// oracledbScraper scrapes Oracle database metrics
type oracledbScraper struct {
	dbClient  dbClient
	mb        *metadata.MetricsBuilder
	logger    *zap.Logger
	cfg       *Config
	startTime pcommon.Timestamp
}

// newScraper creates a new Oracle database scraper
func newScraper(cfg *Config, settings receiver.Settings) (*oracledbScraper, error) {
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings)

	return &oracledbScraper{
		mb:        mb,
		logger:    settings.Logger,
		cfg:       cfg,
		startTime: pcommon.NewTimestampFromTime(time.Now()),
	}, nil
}

// Start initializes the scraper
func (s *oracledbScraper) Start(_ context.Context, _ component.Host) error {
	client, err := newDBClient(s.cfg, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create database client: %w", err)
	}

	s.dbClient = client
	return nil
}

// scrape collects Oracle database metrics
func (s *oracledbScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	var errs error

	// Scrape system statistics with basic metrics
	if err := s.scrapeBasicStats(ctx, now); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to scrape basic stats: %w", err))
	}

	return s.mb.Emit(), errs
}

// scrapeBasicStats collects basic Oracle metrics
func (s *oracledbScraper) scrapeBasicStats(ctx context.Context, now pcommon.Timestamp) error {
	var errs error

	// Scrape core category metrics
	if err := s.scrapeCPUMetrics(ctx, now); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to scrape CPU metrics: %w", err))
	}

	if err := s.scrapePhysicalIOMetrics(ctx, now); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to scrape physical I/O metrics: %w", err))
	}

	if err := s.scrapeSessionMetrics(ctx, now); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to scrape session metrics: %w", err))
	}

	if err := s.scrapeTablespaceMetrics(ctx, now); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to scrape tablespace metrics: %w", err))
	}

	// Scrape additional requested metrics
	if err := s.scrapePGAMemoryMetrics(ctx, now); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to scrape PGA memory metrics: %w", err))
	}

	if err := s.scrapeParseMetrics(ctx, now); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to scrape parse metrics: %w", err))
	}

	if err := s.scrapeConnectionMetrics(ctx, now); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to scrape connection metrics: %w", err))
	}

	// Query basic system statistics for additional metrics
	query := "SELECT name, value FROM v$sysstat WHERE name IN ('execute count', 'user commits', 'user rollbacks')"
	rows, err := s.dbClient.metricRows(ctx, query)
	if err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to fetch basic stats: %w", err))
		return errs
	}

	for _, row := range rows {
		statName := row["NAME"]
		value := row["VALUE"]

		switch statName {
		case "execute count":
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				s.mb.RecordOracledbExecutionsDataPoint(now, val)
			}
		case "user commits":
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				s.mb.RecordOracledbUserCommitsDataPoint(now, val)
			}
		case "user rollbacks":
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				s.mb.RecordOracledbUserRollbacksDataPoint(now, val)
			}
		}
	}

	return errs
}

// scrapeCPUMetrics collects CPU time and usage metrics
func (s *oracledbScraper) scrapeCPUMetrics(ctx context.Context, now pcommon.Timestamp) error {
	query := `
		SELECT name, value 
		FROM v$sysstat 
		WHERE name IN ('CPU used by this session', 'CPU used when call started')
		UNION ALL
		SELECT 'cpu_time_total', ROUND(value/100, 2) 
		FROM v$sysstat 
		WHERE name = 'CPU used by this session'`

	rows, err := s.dbClient.metricRows(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to fetch CPU metrics: %w", err)
	}

	for _, row := range rows {
		statName := row["NAME"]
		value := row["VALUE"]

		switch statName {
		case "CPU used by this session", "cpu_time_total":
			if val, err := strconv.ParseFloat(value, 64); err == nil {
				s.mb.RecordOracledbCPUTimeDataPoint(now, val)
			}
		}
	}

	return nil
}

// scrapePhysicalIOMetrics collects physical reads and writes metrics
func (s *oracledbScraper) scrapePhysicalIOMetrics(ctx context.Context, now pcommon.Timestamp) error {
	query := `
		SELECT name, value 
		FROM v$sysstat 
		WHERE name IN (
			'physical reads',
			'physical writes', 
			'physical read IO requests',
			'physical write IO requests',
			'physical reads direct',
			'physical writes direct',
			'session logical reads'
		)`

	rows, err := s.dbClient.metricRows(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to fetch physical I/O metrics: %w", err)
	}

	for _, row := range rows {
		statName := row["NAME"]
		value := row["VALUE"]

		if val, err := strconv.ParseInt(value, 10, 64); err == nil {
			switch statName {
			case "physical reads":
				s.mb.RecordOracledbPhysicalReadsDataPoint(now, val)
			case "physical writes":
				s.mb.RecordOracledbPhysicalWritesDataPoint(now, val)
			case "physical read IO requests":
				s.mb.RecordOracledbPhysicalReadIoRequestsDataPoint(now, val)
			case "physical write IO requests":
				s.mb.RecordOracledbPhysicalWriteIoRequestsDataPoint(now, val)
			case "physical reads direct":
				s.mb.RecordOracledbPhysicalReadsDirectDataPoint(now, val)
			case "physical writes direct":
				s.mb.RecordOracledbPhysicalWritesDirectDataPoint(now, val)
			case "session logical reads":
				s.mb.RecordOracledbLogicalReadsDataPoint(now, val)
			}
		}
	}

	return nil
}

// scrapeSessionMetrics collects session count and status metrics
func (s *oracledbScraper) scrapeSessionMetrics(ctx context.Context, now pcommon.Timestamp) error {
	query := `
		SELECT 
			status as session_status,
			type as session_type,
			COUNT(*) as session_count
		FROM v$session 
		WHERE status IN ('ACTIVE', 'INACTIVE', 'KILLED', 'CACHED', 'SNIPED')
		  AND type IN ('USER', 'BACKGROUND', 'RECURSIVE')
		GROUP BY status, type`

	rows, err := s.dbClient.metricRows(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to fetch session metrics: %w", err)
	}

	for _, row := range rows {
		sessionStatus := row["SESSION_STATUS"]
		sessionType := row["SESSION_TYPE"]
		countStr := row["SESSION_COUNT"]

		if count, err := strconv.ParseInt(countStr, 10, 64); err == nil {
			s.mb.RecordOracledbSessionsDataPoint(now, count, sessionStatus, sessionType)
		}
	}

	return nil
}

// scrapeTablespaceMetrics collects tablespace usage and limit metrics
func (s *oracledbScraper) scrapeTablespaceMetrics(ctx context.Context, now pcommon.Timestamp) error {
	query := `
		SELECT 
			ts.tablespace_name,
			NVL(df.bytes, 0) as max_bytes,
			NVL(df.bytes - NVL(fs.bytes, 0), 0) as used_bytes,
			CASE 
				WHEN df.bytes > 0 THEN ROUND((df.bytes - NVL(fs.bytes, 0)) / df.bytes * 100, 2)
				ELSE 0 
			END as usage_percentage,
			CASE 
				WHEN ts.status = 'OFFLINE' THEN 1 
				ELSE 0 
			END as offline_status
		FROM dba_tablespaces ts
		LEFT JOIN (
			SELECT tablespace_name, SUM(bytes) as bytes
			FROM dba_data_files 
			GROUP BY tablespace_name
		) df ON ts.tablespace_name = df.tablespace_name
		LEFT JOIN (
			SELECT tablespace_name, SUM(bytes) as bytes
			FROM dba_free_space 
			GROUP BY tablespace_name
		) fs ON ts.tablespace_name = fs.tablespace_name
		WHERE ts.contents != 'TEMPORARY'`

	rows, err := s.dbClient.metricRows(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to fetch tablespace metrics: %w", err)
	}

	for _, row := range rows {
		tablespaceName := row["TABLESPACE_NAME"]

		if maxBytes, err := strconv.ParseInt(row["MAX_BYTES"], 10, 64); err == nil {
			s.mb.RecordOracledbTablespaceSizeLimitDataPoint(now, maxBytes, tablespaceName)
		}

		if usedBytes, err := strconv.ParseInt(row["USED_BYTES"], 10, 64); err == nil {
			s.mb.RecordOracledbTablespaceSizeUsageDataPoint(now, usedBytes, tablespaceName)
		}

		if usagePercentage, err := strconv.ParseFloat(row["USAGE_PERCENTAGE"], 64); err == nil {
			s.mb.RecordOracledbTablespaceUsagePercentageDataPoint(now, usagePercentage, tablespaceName)
		}

		if offlineStatus, err := strconv.ParseInt(row["OFFLINE_STATUS"], 10, 64); err == nil {
			s.mb.RecordOracledbTablespaceOfflineDataPoint(now, offlineStatus, tablespaceName)
		}
	}

	return nil
}

// scrapePGAMemoryMetrics collects PGA memory metrics
func (s *oracledbScraper) scrapePGAMemoryMetrics(ctx context.Context, now pcommon.Timestamp) error {
	query := `
		SELECT 
			name,
			value 
		FROM v$pgastat 
		WHERE name IN (
			'total PGA inuse',
			'total PGA allocated', 
			'total freeable PGA memory',
			'maximum PGA allocated'
		)
		UNION ALL
		SELECT 
			'aggregate PGA target parameter' as name,
			value
		FROM v$parameter 
		WHERE name = 'pga_aggregate_target'`

	rows, err := s.dbClient.metricRows(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to fetch PGA memory metrics: %w", err)
	}

	for _, row := range rows {
		statName := row["NAME"]
		value := row["VALUE"]

		switch statName {
		case "total PGA inuse":
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				s.mb.RecordOracledbPgaMemoryDataPoint(now, val)
				s.mb.RecordOracledbPgaUsedMemoryDataPoint(now, val)
			}
		case "total PGA allocated":
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				s.mb.RecordOracledbPgaAllocatedMemoryDataPoint(now, val)
			}
		case "total freeable PGA memory":
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				s.mb.RecordOracledbPgaFreeableMemoryDataPoint(now, val)
			}
		case "maximum PGA allocated":
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				s.mb.RecordOracledbPgaMaximumMemoryDataPoint(now, val)
			}
		}
	}

	return nil
}

// scrapeParseMetrics collects parse operation metrics
func (s *oracledbScraper) scrapeParseMetrics(ctx context.Context, now pcommon.Timestamp) error {
	query := `
		SELECT name, value 
		FROM v$sysstat 
		WHERE name IN (
			'parse count (total)',
			'parse count (hard)'
		)`

	rows, err := s.dbClient.metricRows(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to fetch parse metrics: %w", err)
	}

	for _, row := range rows {
		statName := row["NAME"]
		value := row["VALUE"]

		switch statName {
		case "parse count (total)":
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				s.mb.RecordOracledbParseCallsDataPoint(now, val)
			}
		case "parse count (hard)":
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				s.mb.RecordOracledbHardParsesDataPoint(now, val)
			}
		}
	}

	return nil
}

// scrapeConnectionMetrics collects connection and session metrics
func (s *oracledbScraper) scrapeConnectionMetrics(ctx context.Context, now pcommon.Timestamp) error {
	// Get session limit and logon statistics
	query := `
		SELECT 
			'sessions_limit' as metric_name,
			value as metric_value
		FROM v$parameter 
		WHERE name = 'sessions'
		UNION ALL
		SELECT 
			'logons_cumulative' as metric_name,
			value as metric_value
		FROM v$sysstat 
		WHERE name = 'logons cumulative'`

	rows, err := s.dbClient.metricRows(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to fetch connection metrics: %w", err)
	}

	for _, row := range rows {
		metricName := row["METRIC_NAME"]
		value := row["METRIC_VALUE"]

		switch metricName {
		case "sessions_limit":
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				s.mb.RecordOracledbSessionsLimitDataPoint(now, val)
			}
		case "logons_cumulative":
			if val, err := strconv.ParseInt(value, 10, 64); err == nil {
				s.mb.RecordOracledbLogonsDataPoint(now, val)
			}
		}
	}

	return nil
}

// Shutdown closes database connections
func (*oracledbScraper) Shutdown(_ context.Context) error {
	return nil
}
