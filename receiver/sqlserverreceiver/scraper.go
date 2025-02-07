// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

const (
	computerNameKey = "computer_name"
	instanceNameKey = "sql_instance"
)

type sqlServerScraperHelper struct {
	id                  component.ID
	sqlQuery            string
	maxQuerySampleCount uint
	granularity         uint
	topQueryCount       uint
	instanceName        string
	scrapeCfg           scraperhelper.ControllerConfig
	clientProviderFunc  sqlquery.ClientProviderFunc
	dbProviderFunc      sqlquery.DbProviderFunc
	logger              *zap.Logger
	telemetry           sqlquery.TelemetryConfig
	client              sqlquery.DbClient
	db                  *sql.DB
	mb                  *metadata.MetricsBuilder
	cache               *lru.Cache[string, float64]
}

var _ scraper.Metrics = (*sqlServerScraperHelper)(nil)

func newSQLServerScraper(id component.ID,
	query string,
	maxQuerySampleCount uint,
	granularity uint,
	topQueryCount uint,
	instanceName string,
	scrapeCfg scraperhelper.ControllerConfig,
	logger *zap.Logger,
	telemetry sqlquery.TelemetryConfig,
	dbProviderFunc sqlquery.DbProviderFunc,
	clientProviderFunc sqlquery.ClientProviderFunc,
	mb *metadata.MetricsBuilder,
	cache *lru.Cache[string, float64],
) *sqlServerScraperHelper {
	return &sqlServerScraperHelper{
		id:                  id,
		sqlQuery:            query,
		maxQuerySampleCount: maxQuerySampleCount,
		granularity:         granularity,
		topQueryCount:       topQueryCount,
		instanceName:        instanceName,
		scrapeCfg:           scrapeCfg,
		logger:              logger,
		telemetry:           telemetry,
		dbProviderFunc:      dbProviderFunc,
		clientProviderFunc:  clientProviderFunc,
		mb:                  mb,
		cache:               cache,
	}
}

func (s *sqlServerScraperHelper) ID() component.ID {
	return s.id
}

func (s *sqlServerScraperHelper) Start(context.Context, component.Host) error {
	var err error
	s.db, err = s.dbProviderFunc()
	if err != nil {
		return fmt.Errorf("failed to open Db connection: %w", err)
	}
	s.client = s.clientProviderFunc(sqlquery.DbWrapper{Db: s.db}, s.sqlQuery, s.logger, s.telemetry)

	return nil
}

func (s *sqlServerScraperHelper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	var err error

	switch s.sqlQuery {
	case getSQLServerDatabaseIOQuery(s.instanceName):
		err = s.recordDatabaseIOMetrics(ctx)
	case getSQLServerPerformanceCounterQuery(s.instanceName):
		err = s.recordDatabasePerfCounterMetrics(ctx)
	case getSQLServerPropertiesQuery(s.instanceName):
		err = s.recordDatabaseStatusMetrics(ctx)
	case getSQLServerQueryMetricsQuery(s.instanceName, s.maxQuerySampleCount, s.granularity):
		err = s.recordDatabaseQueryMetrics(ctx, s.topQueryCount)
	default:
		return pmetric.Metrics{}, fmt.Errorf("Attempted to get metrics from unsupported query: %s", s.sqlQuery)
	}

	if err != nil {
		return pmetric.Metrics{}, err
	}

	return s.mb.Emit(), nil
}

func (s *sqlServerScraperHelper) Shutdown(_ context.Context) error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *sqlServerScraperHelper) recordDatabaseIOMetrics(ctx context.Context) error {
	const databaseNameKey = "database_name"
	const physicalFilenameKey = "physical_filename"
	const logicalFilenameKey = "logical_filename"
	const fileTypeKey = "file_type"
	const readLatencyMsKey = "read_latency_ms"
	const writeLatencyMsKey = "write_latency_ms"
	const readCountKey = "reads"
	const writeCountKey = "writes"
	const readBytesKey = "read_bytes"
	const writeBytesKey = "write_bytes"

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if errors.Is(err, sqlquery.ErrNullValueWarning) {
			s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
		} else {
			return fmt.Errorf("sqlServerScraperHelper: %w", err)
		}
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())
	var val float64
	for i, row := range rows {
		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverComputerName(row[computerNameKey])
		rb.SetSqlserverDatabaseName(row[databaseNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])

		val, err = strconv.ParseFloat(row[readLatencyMsKey], 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		} else {
			s.mb.RecordSqlserverDatabaseLatencyDataPoint(now, val/1e3, row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionRead)
		}

		val, err = strconv.ParseFloat(row[writeLatencyMsKey], 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		} else {
			s.mb.RecordSqlserverDatabaseLatencyDataPoint(now, val/1e3, row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionWrite)
		}

		errs = append(errs, s.mb.RecordSqlserverDatabaseOperationsDataPoint(now, row[readCountKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionRead))
		errs = append(errs, s.mb.RecordSqlserverDatabaseOperationsDataPoint(now, row[writeCountKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionWrite))
		errs = append(errs, s.mb.RecordSqlserverDatabaseIoDataPoint(now, row[readBytesKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionRead))
		errs = append(errs, s.mb.RecordSqlserverDatabaseIoDataPoint(now, row[writeBytesKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionWrite))

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	if len(rows) == 0 {
		s.logger.Info("SQLServerScraperHelper: No rows found by query")
	}

	return errors.Join(errs...)
}

func (s *sqlServerScraperHelper) recordDatabasePerfCounterMetrics(ctx context.Context) error {
	const counterKey = "counter"
	const valueKey = "value"
	// Constants are the columns for metrics from query
	const batchRequestRate = "Batch Requests/sec"
	const bufferCacheHitRatio = "Buffer cache hit ratio"
	const diskReadIOThrottled = "Disk Read IO Throttled/sec"
	const diskWriteIOThrottled = "Disk Write IO Throttled/sec"
	const lockWaits = "Lock Waits/sec"
	const processesBlocked = "Processes blocked"
	const sqlCompilationRate = "SQL Compilations/sec"
	const sqlReCompilationsRate = "SQL Re-Compilations/sec"
	const userConnCount = "User Connections"

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if errors.Is(err, sqlquery.ErrNullValueWarning) {
			s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
		} else {
			return fmt.Errorf("sqlServerScraperHelper: %w", err)
		}
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())
	for i, row := range rows {
		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverComputerName(row[computerNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])

		switch row[counterKey] {
		case batchRequestRate:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchRequestRateDataPoint(now, val)
			}
		case bufferCacheHitRatio:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageBufferCacheHitRatioDataPoint(now, val)
			}
		case diskReadIOThrottled:
			errs = append(errs, s.mb.RecordSqlserverResourcePoolDiskThrottledReadRateDataPoint(now, row[valueKey]))
		case diskWriteIOThrottled:
			errs = append(errs, s.mb.RecordSqlserverResourcePoolDiskThrottledWriteRateDataPoint(now, row[valueKey]))
		case lockWaits:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockWaitRateDataPoint(now, val)
			}
		case processesBlocked:
			errs = append(errs, s.mb.RecordSqlserverProcessesBlockedDataPoint(now, row[valueKey]))
		case sqlCompilationRate:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchSQLCompilationRateDataPoint(now, val)
			}
		case sqlReCompilationsRate:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchSQLRecompilationRateDataPoint(now, val)
			}
		case userConnCount:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverUserConnectionCountDataPoint(now, val)
			}
		}

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return errors.Join(errs...)
}

func (s *sqlServerScraperHelper) recordDatabaseStatusMetrics(ctx context.Context) error {
	// Constants are the column names of the database status
	const dbOnline = "db_online"
	const dbRestoring = "db_restoring"
	const dbRecovering = "db_recovering"
	const dbPendingRecovery = "db_recoveryPending"
	const dbSuspect = "db_suspect"
	const dbOffline = "db_offline"

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if errors.Is(err, sqlquery.ErrNullValueWarning) {
			s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
		} else {
			return fmt.Errorf("sqlServerScraperHelper failed getting metric rows: %w", err)
		}
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())
	for _, row := range rows {
		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverComputerName(row[computerNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])

		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbOnline], metadata.AttributeDatabaseStatusOnline))
		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbRestoring], metadata.AttributeDatabaseStatusRestoring))
		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbRecovering], metadata.AttributeDatabaseStatusRecovering))
		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbPendingRecovery], metadata.AttributeDatabaseStatusPendingRecovery))
		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbSuspect], metadata.AttributeDatabaseStatusSuspect))
		errs = append(errs, s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbOffline], metadata.AttributeDatabaseStatusOffline))

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return errors.Join(errs...)
}

func (s *sqlServerScraperHelper) recordDatabaseQueryMetrics(ctx context.Context, topQueryCount uint) error {
	// Constants are the column names of the database status
	const totalElapsedTime = "total_elapsed_time"
	const rowsReturned = "total_rows"
	const totalWorkerTime = "total_worker_time"
	const queryHash = "query_hash"
	const queryPlanHash = "query_plan_hash"
	const logicalReads = "total_logical_reads"
	const logicalWrites = "total_logical_writes"
	const physicalReads = "total_physical_reads"
	const executionCount = "execution_count"
	const totalGrant = "total_grant_kb"
	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if errors.Is(err, sqlquery.ErrNullValueWarning) {
			s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
		} else {
			return fmt.Errorf("sqlServerScraperHelper failed getting metric rows: %w", err)
		}
	}
	var errs []error

	totalElapsedTimeDiffs := make([]int64, len(rows))

	for i, row := range rows {
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))

		elapsedTime, err := strconv.ParseFloat(row[totalElapsedTime], 64)
		if err != nil {
			s.logger.Info(fmt.Sprintf("sqlServerScraperHelper failed getting metric rows: %s", err))
			errs = append(errs, err)
		} else {
			if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, totalElapsedTime, elapsedTime); cached && diff > 0 {
				totalElapsedTimeDiffs[i] = int64(diff)
			}
		}
	}

	rows = sortRows(rows, totalElapsedTimeDiffs)

	sort.Slice(totalElapsedTimeDiffs, func(i, j int) bool { return totalElapsedTimeDiffs[i] > totalElapsedTimeDiffs[j] })

	for i, row := range rows {
		if i >= int(topQueryCount) {
			break
		}

		if totalElapsedTimeDiffs[i] == 0 {
			continue
		}

		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))

		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverComputerName(row[computerNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])
		rb.SetSqlserverQueryHash(queryHashVal)
		rb.SetSqlserverQueryPlanHash(queryPlanHashVal)
		s.logger.Debug(fmt.Sprintf("DataRow: %v, PlanHash: %v, Hash: %v", row, queryPlanHashVal, queryHashVal))

		timeStamp := pcommon.NewTimestampFromTime(time.Now())

		s.mb.RecordSqlserverQueryTotalElapsedTimeDataPoint(timeStamp, float64(totalElapsedTimeDiffs[i]))

		rowsReturnVal, err := strconv.ParseInt(row[rowsReturned], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		}
		if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, rowsReturned, float64(rowsReturnVal)); cached && diff > 0 {
			s.mb.RecordSqlserverQueryTotalRowsDataPoint(timeStamp, int64(diff))
		}

		logicalReadsVal, err := strconv.ParseInt(row[logicalReads], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		}
		if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, logicalReads, float64(logicalReadsVal)); cached && diff > 0 {
			s.mb.RecordSqlserverQueryTotalLogicalReadsDataPoint(timeStamp, int64(diff))
		}

		logicalWritesVal, err := strconv.ParseInt(row[logicalWrites], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		}
		if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, logicalWrites, float64(logicalWritesVal)); cached && diff > 0 {
			s.mb.RecordSqlserverQueryTotalLogicalWritesDataPoint(timeStamp, int64(diff))
		}

		physicalReadsVal, err := strconv.ParseInt(row[physicalReads], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		}
		if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, physicalReads, float64(physicalReadsVal)); cached && diff > 0 {
			s.mb.RecordSqlserverQueryTotalPhysicalReadsDataPoint(timeStamp, int64(diff))
		}

		totalExecutionCount, err := strconv.ParseFloat(row[executionCount], 64)
		if err != nil {
			s.logger.Info(fmt.Sprintf("sqlServerScraperHelper failed getting metric rows: %s", err))
		} else {
			if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, executionCount, totalExecutionCount); cached && diff > 0 {
				s.mb.RecordSqlserverQueryExecutionCountDataPoint(timeStamp, diff)
			}
		}

		workerTime, err := strconv.ParseFloat(row[totalWorkerTime], 64)
		if err != nil {
			s.logger.Info(fmt.Sprintf("sqlServerScraperHelper failed parsing metric total_worker_time: %s", err))
			errs = append(errs, err)
		} else {
			if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, totalWorkerTime, workerTime); cached && diff > 0 {
				s.mb.RecordSqlserverQueryTotalWorkerTimeDataPoint(timeStamp, diff)
			}
		}

		memoryGranted, err := strconv.ParseFloat(row[totalGrant], 64)
		if err != nil {
			s.logger.Info(fmt.Sprintf("sqlServerScraperHelper failed parsing metric total_grant_kb: %s", err))
			errs = append(errs, err)
		} else {
			if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, totalGrant, memoryGranted); cached && diff > 0 {
				s.mb.RecordSqlserverQueryTotalGrantKbDataPoint(timeStamp, diff)
			}
		}

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
	return errors.Join(errs...)
}

func (s *sqlServerScraperHelper) cacheAndDiff(queryHash string, queryPlanHash string, column string, val float64) (bool, float64) {
	if s.cache == nil {
		s.logger.Error("LRU cache is not successfully initialized, skipping caching and diffing")
		return false, 0
	}

	if val < 0 {
		return false, 0
	}

	key := queryHash + "-" + queryPlanHash + "-" + column

	cached, ok := s.cache.Get(key)
	if !ok {
		s.cache.Add(key, val)
		return false, val
	}

	if val > cached {
		s.cache.Add(key, val)
		return true, val - cached
	}

	return true, 0
}

func sortRows(rows []sqlquery.StringMap, values []int64) []sqlquery.StringMap {
	// Create an index slice to track the original indices of rows
	indices := make([]int, len(values))
	for i := range indices {
		indices[i] = i
	}

	// Sort the indices based on the values slice
	sort.Slice(indices, func(i, j int) bool {
		return values[indices[i]] > values[indices[j]]
	})

	// Create a new sorted slice for rows based on the sorted indices
	sorted := make([]sqlquery.StringMap, len(rows))
	for i, idx := range indices {
		sorted[i] = rows[idx]
	}

	return sorted
}
