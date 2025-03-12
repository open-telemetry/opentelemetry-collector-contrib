// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"container/heap"
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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

const (
	computerNameKey  = "computer_name"
	instanceNameKey  = "sql_instance"
	serverAddressKey = "server.address"
	serverPortKey    = "server.port"
)

type sqlServerScraperHelper struct {
	id                 component.ID
	config             *Config
	sqlQuery           string
	instanceName       string
	clientProviderFunc sqlquery.ClientProviderFunc
	dbProviderFunc     sqlquery.DbProviderFunc
	logger             *zap.Logger
	telemetry          sqlquery.TelemetryConfig
	client             sqlquery.DbClient
	db                 *sql.DB
	mb                 *metadata.MetricsBuilder
	cache              *lru.Cache[string, int64]
}

var (
	_ scraper.Metrics = (*sqlServerScraperHelper)(nil)
	_ scraper.Logs    = (*sqlServerScraperHelper)(nil)
)

func newSQLServerScraper(id component.ID,
	query string,
	telemetry sqlquery.TelemetryConfig,
	dbProviderFunc sqlquery.DbProviderFunc,
	clientProviderFunc sqlquery.ClientProviderFunc,
	params receiver.Settings,
	cfg *Config,
	cache *lru.Cache[string, int64],
) *sqlServerScraperHelper {
	return &sqlServerScraperHelper{
		id:                 id,
		config:             cfg,
		sqlQuery:           query,
		logger:             params.Logger,
		telemetry:          telemetry,
		dbProviderFunc:     dbProviderFunc,
		clientProviderFunc: clientProviderFunc,
		mb:                 metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
		cache:              cache,
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
	case getSQLServerDatabaseIOQuery(s.config.InstanceName):
		err = s.recordDatabaseIOMetrics(ctx)
	case getSQLServerPerformanceCounterQuery(s.config.InstanceName):
		err = s.recordDatabasePerfCounterMetrics(ctx)
	case getSQLServerPropertiesQuery(s.config.InstanceName):
		err = s.recordDatabaseStatusMetrics(ctx)
	default:
		return pmetric.Metrics{}, fmt.Errorf("Attempted to get metrics from unsupported query: %s", s.sqlQuery)
	}

	if err != nil {
		return pmetric.Metrics{}, err
	}

	return s.mb.Emit(), nil
}

func (s *sqlServerScraperHelper) ScrapeLogs(ctx context.Context) (plog.Logs, error) {
	queryTextAndPlanQuery, err := getSQLServerQueryTextAndPlanQuery(s.config.InstanceName, s.config.MaxQuerySampleCount, s.config.LookbackTime)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to template needed queries: %w", err)
	}

	switch s.sqlQuery {
	case queryTextAndPlanQuery:
		return s.recordDatabaseQueryTextAndPlan(ctx, s.config.TopQueryCount)
	case getSQLServerQuerySamplesQuery(s.config.MaxResultPerQuery):
		return s.recordDatabaseSampleQuery(ctx)
	default:
		return plog.Logs{}, fmt.Errorf("Attempted to get logs from unsupported query: %s", s.sqlQuery)
	}
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
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return fmt.Errorf("sqlServerScraperHelper: %w", err)
		}
		s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())
	var val float64
	for i, row := range rows {
		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverComputerName(row[computerNameKey])
		rb.SetSqlserverDatabaseName(row[databaseNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])
		rb.SetServerAddress(s.config.Server)
		rb.SetServerPort(int64(s.config.Port))

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
	const activeTempTables = "Active Temp Tables"
	const backupRestoreThroughputPerSec = "Backup/Restore Throughput/sec"
	const batchRequestRate = "Batch Requests/sec"
	const bufferCacheHitRatio = "Buffer cache hit ratio"
	const bytesReceivedFromReplicaPerSec = "Bytes Received from Replica/sec"
	const bytesSentForReplicaPerSec = "Bytes Sent to Replica/sec"
	const diskReadIOThrottled = "Disk Read IO Throttled/sec"
	const diskWriteIOThrottled = "Disk Write IO Throttled/sec"
	const executionErrors = "Execution Errors"
	const freeListStalls = "Free list stalls/sec"
	const freeSpaceInTempdb = "Free Space in tempdb (KB)"
	const fullScansPerSec = "Full Scans/sec"
	const indexSearchesPerSec = "Index Searches/sec"
	const lockTimeoutsPerSec = "Lock Timeouts/sec"
	const lockWaits = "Lock Waits/sec"
	const loginsPerSec = "Logins/sec"
	const logoutPerSec = "Logouts/sec"
	const numberOfDeadlocksPerSec = "Number of Deadlocks/sec"
	const mirrorWritesTransactionPerSec = "Mirrored Write Transactions/sec"
	const memoryGrantsPending = "Memory Grants Pending"
	const pageLookupsPerSec = "Page lookups/sec"
	const processesBlocked = "Processes blocked"
	const sqlCompilationRate = "SQL Compilations/sec"
	const sqlReCompilationsRate = "SQL Re-Compilations/sec"
	const transactionDelay = "Transaction Delay"
	const userConnCount = "User Connections"
	const usedMemory = "Used memory (KB)"
	const versionStoreSize = "Version Store Size (KB)"

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return fmt.Errorf("sqlServerScraperHelper: %w", err)
		}
		s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())

	for i, row := range rows {
		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverComputerName(row[computerNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])
		rb.SetServerAddress(s.config.Server)
		rb.SetServerPort(int64(s.config.Port))

		switch row[counterKey] {
		case activeTempTables:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTableCountDataPoint(now, val, metadata.AttributeTableStateActive, metadata.AttributeTableStatusTemporary)
			}
		case backupRestoreThroughputPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseBackupOrRestoreRateDataPoint(now, val)
			}
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
		case bytesReceivedFromReplicaPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverReplicaDataRateDataPoint(now, val, metadata.AttributeReplicaDirectionReceive)
			}
		case bytesSentForReplicaPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverReplicaDataRateDataPoint(now, val, metadata.AttributeReplicaDirectionTransmit)
			}
		case diskReadIOThrottled:
			errs = append(errs, s.mb.RecordSqlserverResourcePoolDiskThrottledReadRateDataPoint(now, row[valueKey]))
		case diskWriteIOThrottled:
			errs = append(errs, s.mb.RecordSqlserverResourcePoolDiskThrottledWriteRateDataPoint(now, row[valueKey]))
		case executionErrors:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseExecutionErrorsDataPoint(now, val)
			}
		case freeListStalls:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageBufferCacheFreeListStallsRateDataPoint(now, val)
			}
		case fullScansPerSec:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseFullScanRateDataPoint(now, val)
			}
		case freeSpaceInTempdb:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseTempdbSpaceDataPoint(now, val, metadata.AttributeTempdbStateFree)
			}
		case indexSearchesPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverIndexSearchRateDataPoint(now, val)
			}
		case lockTimeoutsPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockTimeoutRateDataPoint(now, val)
			}
		case lockWaits:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockWaitRateDataPoint(now, val)
			}
		case loginsPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLoginRateDataPoint(now, val)
			}
		case logoutPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLogoutRateDataPoint(now, val)
			}
		case memoryGrantsPending:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryGrantsPendingCountDataPoint(now, val)
			}
		case mirrorWritesTransactionPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTransactionMirrorWriteRateDataPoint(now, val)
			}
		case numberOfDeadlocksPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDeadlockRateDataPoint(now, val)
			}
		case pageLookupsPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageLookupRateDataPoint(now, val)
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
		case transactionDelay:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTransactionDelayDataPoint(now, val)
			}
		case userConnCount:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverUserConnectionCountDataPoint(now, val)
			}
		case usedMemory:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryUsageDataPoint(now, val)
			}
		case versionStoreSize:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("row %d: %w", i, err)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseTempdbVersionStoreSizeDataPoint(now, val)
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
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return fmt.Errorf("sqlServerScraperHelper failed getting metric rows: %w", err)
		}
		s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())
	for _, row := range rows {
		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverComputerName(row[computerNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])
		rb.SetServerAddress(s.config.Server)
		rb.SetServerPort(int64(s.config.Port))

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

func (s *sqlServerScraperHelper) recordDatabaseQueryTextAndPlan(ctx context.Context, topQueryCount uint) (plog.Logs, error) {
	// Constants are the column names of the database status
	const dbPrefix = "sqlserver."
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
	const queryText = "query_text"
	const queryPlan = "query_plan"
	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return plog.Logs{}, fmt.Errorf("sqlServerScraperHelper failed getting rows: %w", err)
		}
		s.logger.Warn("problems encountered getting log rows", zap.Error(err))
	}
	var errs []error

	totalElapsedTimeDiffs := make([]int64, len(rows))

	for i, row := range rows {
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))

		elapsedTime, err := strconv.ParseInt(row[totalElapsedTime], 10, 64)
		if err != nil {
			s.logger.Info(fmt.Sprintf("sqlServerScraperHelper failed getting rows: %s", err))
			errs = append(errs, err)
		} else {
			// we're trying to get the queries that used the most time.
			// caching the total elapsed time (in millisecond) and compare in the next scrape.
			if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, totalElapsedTime, elapsedTime/1000); cached && diff > 0 {
				totalElapsedTimeDiffs[i] = diff
			}
		}
	}

	// sort the rows based on the totalElapsedTimeDiffs in descending order,
	// only report first T(T=topQueryCount) rows.
	rows = sortRows(rows, totalElapsedTimeDiffs, topQueryCount)

	// sort the totalElapsedTimeDiffs in descending order as well
	sort.Slice(totalElapsedTimeDiffs, func(i, j int) bool { return totalElapsedTimeDiffs[i] > totalElapsedTimeDiffs[j] })

	logs := plog.NewLogs()
	resourceLog := logs.ResourceLogs().AppendEmpty()
	resourceLog.Resource().Attributes().PutStr("db.system.type", "microsoft.sql_server")

	scopedLog := resourceLog.ScopeLogs().AppendEmpty()
	scopedLog.Scope().SetName(metadata.ScopeName)
	scopedLog.Scope().SetVersion("0.0.1")

	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for i, row := range rows {
		// skipping the rest of the rows as totalElapsedTimeDiffs is sorted in descending order
		if totalElapsedTimeDiffs[i] == 0 {
			break
		}

		// reporting human-readable query hash and query hash plan
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))

		record := scopedLog.LogRecords().AppendEmpty()
		record.SetTimestamp(timestamp)

		record.Attributes().PutStr(computerNameKey, row[computerNameKey])
		record.Attributes().PutStr(instanceNameKey, row[instanceNameKey])
		record.Attributes().PutStr(serverAddressKey, s.config.Server)
		record.Attributes().PutInt(serverPortKey, int64(s.config.Port))

		record.Attributes().PutStr(dbPrefix+queryHash, queryHashVal)
		record.Attributes().PutStr(dbPrefix+queryPlanHash, queryPlanHashVal)

		s.logger.Debug(fmt.Sprintf("QueryHash: %v, PlanHash: %v, DataRow: %v", queryHashVal, queryPlanHashVal, row))

		record.Attributes().PutInt(dbPrefix+totalElapsedTime, totalElapsedTimeDiffs[i])

		// handling `total_rows`
		rowsReturnVal, err := strconv.ParseInt(row[rowsReturned], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		}
		if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, rowsReturned, rowsReturnVal); cached {
			record.Attributes().PutInt(dbPrefix+rowsReturned, diff)
		}

		// handling `total_logical_reads`
		logicalReadsVal, err := strconv.ParseInt(row[logicalReads], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		}
		if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, logicalReads, logicalReadsVal); cached {
			record.Attributes().PutInt(dbPrefix+logicalReads, diff)
		}

		// handling `total_logical_writes`
		logicalWritesVal, err := strconv.ParseInt(row[logicalWrites], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		}
		if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, logicalWrites, logicalWritesVal); cached {
			record.Attributes().PutInt(dbPrefix+logicalWrites, diff)
		}

		// handling `physical_reads`
		physicalReadsVal, err := strconv.ParseInt(row[physicalReads], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		}
		if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, physicalReads, physicalReadsVal); cached {
			record.Attributes().PutInt(dbPrefix+physicalReads, diff)
		}

		// handling `execution_count`
		totalExecutionCount, err := strconv.ParseInt(row[executionCount], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		} else {
			if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, executionCount, totalExecutionCount); cached {
				record.Attributes().PutInt(dbPrefix+executionCount, diff)
			}
		}

		// handle `total_worker_time`, storing milliseconds
		workerTime, err := strconv.ParseInt(row[totalWorkerTime], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		} else {
			if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, totalWorkerTime, workerTime/1000); cached {
				record.Attributes().PutInt(dbPrefix+totalWorkerTime, diff)
			}
		}

		// handle `total_grant_kb`
		memoryGranted, err := strconv.ParseInt(row[totalGrant], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		} else {
			if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, totalGrant, memoryGranted); cached {
				record.Attributes().PutInt(dbPrefix+totalGrant, diff)
			}
		}

		// handling `query_text`
		obfuscatedSQL, err := obfuscateSQL(row[queryText])
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		}
		// Follow the semantic conventions for DB
		// https://github.com/open-telemetry/semantic-conventions/blob/main/docs/attributes-registry/db.md
		record.Attributes().PutStr("db.query.text", obfuscatedSQL)

		// handling `query_plan`
		obfuscatedQueryPlan, err := obfuscateXMLPlan(row[queryPlan])
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		}
		record.Attributes().PutStr(dbPrefix+queryPlan, obfuscatedQueryPlan)
	}

	return logs, errors.Join(errs...)
}

// cacheAndDiff store row(in int) with query hash and query plan hash variables
// (1) returns true if the key is cached before
// (2) returns positive value if the value is larger than the cached value
func (s *sqlServerScraperHelper) cacheAndDiff(queryHash string, queryPlanHash string, column string, val int64) (bool, int64) {
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

type Item struct {
	row      sqlquery.StringMap
	priority int64
	index    int
}

// reference: https://pkg.go.dev/container/heap#example-package-priorityQueue
type priorityQueue []*Item

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].priority > pq[j].priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // don't stop the GC from reclaiming the item eventually
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// sortRows sorts the rows based on the `values` slice in descending order and return the first M(M=maximum) rows
// Input: (row: [row1, row2, row3], values: [100, 10, 1000], maximum: 2
// Expected Output: (row: [row3, row1]
func sortRows(rows []sqlquery.StringMap, values []int64, maximum uint) []sqlquery.StringMap {
	results := make([]sqlquery.StringMap, 0)

	if len(rows) == 0 ||
		len(values) == 0 ||
		len(rows) != len(values) ||
		maximum <= 0 {
		return []sqlquery.StringMap{}
	}
	pq := make(priorityQueue, len(rows))
	for i, row := range rows {
		value := values[i]
		pq[i] = &Item{
			row:      row,
			priority: value,
			index:    i,
		}
	}
	heap.Init(&pq)

	for pq.Len() > 0 && len(results) < int(maximum) {
		item := heap.Pop(&pq).(*Item)
		results = append(results, item.row)
	}
	return results
}

func (s *sqlServerScraperHelper) recordDatabaseSampleQuery(ctx context.Context) (plog.Logs, error) {
	const dbPrefix = "sqlserver."
	// Constants are the column names of the database status
	const DBName = "db_name"
	const clientAddress = "client_address"
	const clientPort = "client_port"
	const queryStart = "query_start"
	const sessionID = "session_id"
	const sessionStatus = "session_status"
	const requestStatus = "request_status"
	const hostname = "host_name"
	const command = "command"
	const statementText = "statement_text"
	const blockingSessionID = "blocking_session_id"
	const waitType = "wait_type"
	const waitTime = "wait_time"
	const waitResource = "wait_resource"
	const openTransactionCount = "open_transaction_count"
	const transactionID = "transaction_id"
	const percentComplete = "percent_complete"
	const estimatedCompletionTime = "estimated_completion_time"
	const cpuTime = "cpu_time"
	const totalElapsedTime = "total_elapsed_time"
	const reads = "reads"
	const writes = "writes"
	const logicalReads = "logical_reads"
	const transactionIsolationLevel = "transaction_isolation_level"
	const lockTimeout = "lock_timeout"
	const deadlockPriority = "deadlock_priority"
	const rowCount = "row_count"
	const queryHash = "query_hash"
	const queryPlanHash = "query_plan_hash"
	const contextInfo = "context_info"

	const username = "username"
	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return plog.Logs{}, fmt.Errorf("sqlServerScraperHelper failed getting log rows: %w", err)
		}
		// TODO: ignore this for now.
		s.logger.Warn("problems encountered getting log rows", zap.Error(err))
	}

	var errs []error
	logs := plog.NewLogs()

	resourceLog := logs.ResourceLogs().AppendEmpty()
	resourceLog.Resource().Attributes().PutStr("db.system.type", "microsoft.sql_server")

	scopedLog := resourceLog.ScopeLogs().AppendEmpty()
	scopedLog.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver")
	scopedLog.Scope().SetVersion("v0.0.1")
	for _, row := range rows {
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))
		contextInfoVal := hex.EncodeToString([]byte(row[contextInfo]))

		// clientPort could be null, and it will be converted to empty string with ISNULL in our query. when it is
		// an empty string, clientPortNumber would be 0.
		clientPortNumber := 0
		if row[clientPort] != "" {
			clientPortNumber, err = strconv.Atoi(row[clientPort])
			if err != nil {
				s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing client port number. original value: %s, err: %s", row[clientPort], err))
			}
		}

		sessionIDNumber, err := strconv.Atoi(row[sessionID])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing session id number. original value: %s, err: %s", row[sessionID], err))
		}
		blockingSessionIDNumber, err := strconv.Atoi(row[blockingSessionID])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing blocking session id number. value: %s, err: %s", row[blockingSessionID], err))
		}
		waitTimeVal, err := strconv.Atoi(row[waitTime])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing wait time number. original value: %s, err: %s", row[waitTime], err))
		}
		openTransactionCountVal, err := strconv.Atoi(row[openTransactionCount])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing open transaction count. original value: %s, err: %s", row[openTransactionCount], err))
		}
		transactionIDVal, err := strconv.Atoi(row[transactionID])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing transaction id number. original value: %s, err: %s", row[transactionID], err))
		}
		// percent complete and estimated completion time is a real value in mssql
		percentCompleteVal, err := strconv.ParseFloat(row[percentComplete], 32)
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing percent complete. original value: %s, err: %s", row[percentComplete], err))
		}
		estimatedCompletionTimeVal, err := strconv.Atoi(row[estimatedCompletionTime])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing estimated completion time number. original value: %s, err: %s", row[estimatedCompletionTime], err))
		}
		cpuTimeVal, err := strconv.Atoi(row[cpuTime])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing cpu time number. original value: %s, err: %s", row[cpuTime], err))
		}
		totalElapsedTimeVal, err := strconv.Atoi(row[totalElapsedTime])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing total elapsed time. original value: %s, err: %s", row[totalElapsedTime], err))
		}
		readsVal, err := strconv.Atoi(row[reads])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing read count. original value: %s, err: %s", row[reads], err))
		}
		writesVal, err := strconv.Atoi(row[writes])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing write count. original value: %s, err: %s", row[writes], err))
		}
		logicalReadsVal, err := strconv.Atoi(row[logicalReads])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing logical read count. original value: %s, err: %s", row[logicalReads], err))
		}
		transactionIsolationLevelVal, err := strconv.Atoi(row[transactionIsolationLevel])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing transaction isolation level. original value: %s, err: %s", row[transactionIsolationLevel], err))
		}

		lockTimeoutVal, err := strconv.Atoi(row[lockTimeout])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing lock timeout. original value: %s, err: %s", row[lockTimeout], err))
		}

		deadlockPriorityVal, err := strconv.Atoi(row[deadlockPriority])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing deadlock priority. original value: %s, err: %s", row[deadlockPriority], err))
		}

		rowCountVal, err := strconv.Atoi(row[rowCount])
		if err != nil {
			s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing row count. original value: %s, err: %s", row[rowCount], err))
		}

		obfuscatedStatement, err := obfuscateSQL(row[statementText])
		if err != nil {
			s.logger.Error(fmt.Sprintf("failed to obfuscate SQL statement value: %s err: %s", row[statementText], err))
		}

		record := scopedLog.LogRecords().AppendEmpty()
		record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		record.Attributes().PutStr("db.namespace", row[DBName])
		record.Attributes().PutStr("network.peer.address", row[clientAddress])
		record.Attributes().PutInt("network.peer.port", int64(clientPortNumber))
		record.Attributes().PutStr(dbPrefix+queryStart, row[queryStart])
		record.Attributes().PutInt(dbPrefix+sessionID, int64(sessionIDNumber))
		record.Attributes().PutStr(dbPrefix+sessionStatus, row[sessionStatus])
		record.Attributes().PutStr(dbPrefix+requestStatus, row[requestStatus])
		record.Attributes().PutStr(dbPrefix+command, row[command])
		// Following Opentelemetry Semantic Convention for this naming.
		record.Attributes().PutStr("db.query.text", obfuscatedStatement)
		record.Attributes().PutInt(dbPrefix+blockingSessionID, int64(blockingSessionIDNumber))
		record.Attributes().PutStr(dbPrefix+waitType, row[waitType])
		record.Attributes().PutInt(dbPrefix+waitTime, int64(waitTimeVal))
		record.Attributes().PutStr(dbPrefix+waitResource, row[waitResource])
		record.Attributes().PutInt(dbPrefix+openTransactionCount, int64(openTransactionCountVal))
		record.Attributes().PutInt(dbPrefix+transactionID, int64(transactionIDVal))
		record.Attributes().PutDouble(dbPrefix+percentComplete, percentCompleteVal)
		record.Attributes().PutInt(dbPrefix+estimatedCompletionTime, int64(estimatedCompletionTimeVal))
		record.Attributes().PutInt(dbPrefix+cpuTime, int64(cpuTimeVal))
		record.Attributes().PutInt(dbPrefix+totalElapsedTime, int64(totalElapsedTimeVal))
		record.Attributes().PutInt(dbPrefix+reads, int64(readsVal))
		record.Attributes().PutInt(dbPrefix+writes, int64(writesVal))
		record.Attributes().PutInt(dbPrefix+logicalReads, int64(logicalReadsVal))
		record.Attributes().PutInt(dbPrefix+transactionIsolationLevel, int64(transactionIsolationLevelVal))
		record.Attributes().PutInt(dbPrefix+lockTimeout, int64(lockTimeoutVal))
		record.Attributes().PutInt(dbPrefix+deadlockPriority, int64(deadlockPriorityVal))
		record.Attributes().PutInt(dbPrefix+rowCount, int64(rowCountVal))
		record.Attributes().PutStr(dbPrefix+queryHash, queryHashVal)
		record.Attributes().PutStr(dbPrefix+queryPlanHash, queryPlanHashVal)
		record.Attributes().PutStr(dbPrefix+contextInfo, contextInfoVal)

		record.Attributes().PutStr(dbPrefix+username, row[username])

		// client.address: use host_name if it has value, if not, use client_net_address.
		// this value may not be accurate if
		// - there is proxy in the middle of sql client and sql server. Or
		// - host_name value is empty or not accurate.
		if row[hostname] != "" {
			record.Attributes().PutStr("client.address", row[hostname])
		} else {
			record.Attributes().PutStr("client.address", row[clientAddress])
		}
		record.Attributes().PutInt("client.port", int64(clientPortNumber))

		record.Body().SetStr("sample")
	}
	return logs, errors.Join(errs...)
}
