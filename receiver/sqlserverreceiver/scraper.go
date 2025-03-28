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
	case getSQLServerQuerySamplesQuery(s.config.MaxRowsPerQuery):
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
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, activeTempTables)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTableCountDataPoint(now, val, metadata.AttributeTableStateActive, metadata.AttributeTableStatusTemporary)
			}
		case backupRestoreThroughputPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, backupRestoreThroughputPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseBackupOrRestoreRateDataPoint(now, val)
			}
		case batchRequestRate:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, batchRequestRate)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchRequestRateDataPoint(now, val)
			}
		case bufferCacheHitRatio:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, bufferCacheHitRatio)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageBufferCacheHitRatioDataPoint(now, val)
			}
		case bytesReceivedFromReplicaPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, bytesReceivedFromReplicaPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverReplicaDataRateDataPoint(now, val, metadata.AttributeReplicaDirectionReceive)
			}
		case bytesSentForReplicaPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, bytesReceivedFromReplicaPerSec)
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
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, executionErrors)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseExecutionErrorsDataPoint(now, val)
			}
		case freeListStalls:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, freeListStalls)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageBufferCacheFreeListStallsRateDataPoint(now, val)
			}
		case fullScansPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, fullScansPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseFullScanRateDataPoint(now, val)
			}
		case freeSpaceInTempdb:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, freeSpaceInTempdb)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseTempdbSpaceDataPoint(now, val, metadata.AttributeTempdbStateFree)
			}
		case indexSearchesPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, indexSearchesPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverIndexSearchRateDataPoint(now, val)
			}
		case lockTimeoutsPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockTimeoutsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockTimeoutRateDataPoint(now, val)
			}
		case lockWaits:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockWaits)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockWaitRateDataPoint(now, val)
			}
		case loginsPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, loginsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLoginRateDataPoint(now, val)
			}
		case logoutPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, logoutPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLogoutRateDataPoint(now, val)
			}
		case memoryGrantsPending:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, memoryGrantsPending)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryGrantsPendingCountDataPoint(now, val)
			}
		case mirrorWritesTransactionPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, mirrorWritesTransactionPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTransactionMirrorWriteRateDataPoint(now, val)
			}
		case numberOfDeadlocksPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, numberOfDeadlocksPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDeadlockRateDataPoint(now, val)
			}
		case pageLookupsPerSec:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, pageLookupsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageLookupRateDataPoint(now, val)
			}
		case processesBlocked:
			errs = append(errs, s.mb.RecordSqlserverProcessesBlockedDataPoint(now, row[valueKey]))
		case sqlCompilationRate:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, sqlCompilationRate)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchSQLCompilationRateDataPoint(now, val)
			}
		case sqlReCompilationsRate:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, sqlReCompilationsRate)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchSQLRecompilationRateDataPoint(now, val)
			}
		case transactionDelay:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, transactionDelay)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTransactionDelayDataPoint(now, val)
			}
		case userConnCount:
			val, err := strconv.ParseInt(row[valueKey], 10, 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, userConnCount)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverUserConnectionCountDataPoint(now, val)
			}
		case usedMemory:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, usedMemory)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryUsageDataPoint(now, val)
			}
		case versionStoreSize:
			val, err := strconv.ParseFloat(row[valueKey], 64)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, versionStoreSize)
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
		record.SetEventName("top query")
		record.Attributes().PutStr("db.system.name", "microsoft.sql_server")

		record.Attributes().PutStr(computerNameKey, row[computerNameKey])
		record.Attributes().PutStr(instanceNameKey, row[instanceNameKey])
		record.Attributes().PutStr(serverAddressKey, s.config.Server)
		record.Attributes().PutInt(serverPortKey, int64(s.config.Port))

		record.Attributes().PutStr(dbPrefix+queryHash, queryHashVal)
		record.Attributes().PutStr(dbPrefix+queryPlanHash, queryPlanHashVal)

		s.logger.Debug(fmt.Sprintf("QueryHash: %v, PlanHash: %v, DataRow: %v", queryHashVal, queryPlanHashVal, row))

		record.Attributes().PutDouble(dbPrefix+totalElapsedTime, float64(totalElapsedTimeDiffs[i])/1000)

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
				record.Attributes().PutDouble(dbPrefix+totalWorkerTime, float64(diff)/1000)
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

type internalAttribute struct {
	key            string
	columnName     string
	valueRetriever func(row sqlquery.StringMap, columnName string) (any, error)
	valueSetter    func(attributes pcommon.Map, key string, value any)
}

func defaultValueRetriever(defaultValue any) func(row sqlquery.StringMap, columnName string) (any, error) {
	return func(_ sqlquery.StringMap, _ string) (any, error) {
		return defaultValue, nil
	}
}

func vanillaRetriever(row sqlquery.StringMap, columnName string) (any, error) {
	return row[columnName], nil
}

func retrieveInt(row sqlquery.StringMap, columnName string) (any, error) {
	var err error
	result := 0
	if row[columnName] != "" {
		result, err = strconv.Atoi(row[columnName])
	}
	return int64(result), err
}

func retrieveIntAndConvert(convert func(int64) any) func(row sqlquery.StringMap, columnName string) (any, error) {
	return func(row sqlquery.StringMap, columnName string) (any, error) {
		result, err := retrieveInt(row, columnName)
		// need to convert even if it failed
		return convert(result.(int64)), err
	}
}

func retrieveFloat(row sqlquery.StringMap, columnName string) (any, error) {
	var err error
	var result float64
	if row[columnName] != "" {
		result, err = strconv.ParseFloat(row[columnName], 64)
	}
	return result, err
}

func setString(attributes pcommon.Map, key string, value any) {
	attributes.PutStr(key, value.(string))
}

func setInt(attributes pcommon.Map, key string, value any) {
	attributes.PutInt(key, value.(int64))
}

func setDouble(attributes pcommon.Map, key string, value any) {
	attributes.PutDouble(key, value.(float64))
}

func (s *sqlServerScraperHelper) recordDatabaseSampleQuery(ctx context.Context) (plog.Logs, error) {
	const blockingSessionID = "blocking_session_id"
	const clientAddress = "client_address"
	const clientPort = "client_port"
	const command = "command"
	const contextInfo = "context_info"
	const cpuTime = "cpu_time"
	const dbName = "db_name"
	const dbPrefix = "sqlserver."
	const deadlockPriority = "deadlock_priority"
	const estimatedCompletionTime = "estimated_completion_time"
	const hostName = "host_name"
	const lockTimeout = "lock_timeout"
	const logicalReads = "logical_reads"
	const openTransactionCount = "open_transaction_count"
	const percentComplete = "percent_complete"
	const queryHash = "query_hash"
	const queryPlanHash = "query_plan_hash"
	const queryStart = "query_start"
	const reads = "reads"
	const requestStatus = "request_status"
	const rowCount = "row_count"
	const sessionID = "session_id"
	const sessionStatus = "session_status"
	const statementText = "statement_text"
	const totalElapsedTime = "total_elapsed_time"
	const transactionID = "transaction_id"
	const transactionIsolationLevel = "transaction_isolation_level"
	const username = "username"
	const waitResource = "wait_resource"
	const waitTime = "wait_time"
	const waitType = "wait_type"
	const writes = "writes"

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return plog.Logs{}, fmt.Errorf("sqlServerScraperHelper failed getting log rows: %w", err)
		}
		// in case the sql returned rows contains null value, we just log a warning and continue
		s.logger.Warn("problems encountered getting log rows", zap.Error(err))
	}

	var errs []error
	logs := plog.NewLogs()

	resourceLog := logs.ResourceLogs().AppendEmpty()

	scopedLog := resourceLog.ScopeLogs().AppendEmpty()
	scopedLog.Scope().SetName(metadata.ScopeName)
	scopedLog.Scope().SetVersion("v0.0.1")
	for _, row := range rows {
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))
		contextInfoVal := hex.EncodeToString([]byte(row[contextInfo]))

		record := scopedLog.LogRecords().AppendEmpty()
		record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

		attributes := []internalAttribute{
			{
				key:            "client.port",
				columnName:     clientPort,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            "db.namespace",
				columnName:     dbName,
				valueRetriever: vanillaRetriever,
				valueSetter:    setString,
			},
			{
				key:        "db.query.text",
				columnName: statementText,
				valueRetriever: func(row sqlquery.StringMap, columnName string) (any, error) {
					return obfuscateSQL(row[columnName])
				},
				valueSetter: setString,
			},
			{
				key:            "db.system.name",
				valueRetriever: defaultValueRetriever("microsoft.sql_server"),
				valueSetter:    setString,
			},
			{
				key:            "network.peer.address",
				columnName:     clientAddress,
				valueRetriever: vanillaRetriever,
				valueSetter:    setString,
			},
			{
				key:            "network.peer.port",
				columnName:     clientPort,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			// the following ones are the attributes that are not in the semantic conventions
			{
				key:            dbPrefix + blockingSessionID,
				columnName:     blockingSessionID,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + command,
				columnName:     command,
				valueRetriever: vanillaRetriever,
				valueSetter:    setString,
			},
			{
				key:            dbPrefix + contextInfo,
				valueRetriever: defaultValueRetriever(contextInfoVal),
				valueSetter:    setString,
			},
			{
				key:        dbPrefix + cpuTime,
				columnName: cpuTime,
				valueRetriever: retrieveIntAndConvert(func(i int64) any {
					return float64(i) / 1000.0
				}),
				valueSetter: setDouble,
			},
			{
				key:            dbPrefix + deadlockPriority,
				columnName:     deadlockPriority,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:        dbPrefix + estimatedCompletionTime,
				columnName: estimatedCompletionTime,
				valueRetriever: retrieveIntAndConvert(func(i int64) any {
					return float64(i) / 1000.0
				}),
				valueSetter: setDouble,
			},
			{
				key:            dbPrefix + lockTimeout,
				columnName:     lockTimeout,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + logicalReads,
				columnName:     logicalReads,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + openTransactionCount,
				columnName:     openTransactionCount,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + percentComplete,
				columnName:     percentComplete,
				valueRetriever: retrieveFloat,
				valueSetter:    setDouble,
			},
			{
				key:            dbPrefix + queryHash,
				valueRetriever: defaultValueRetriever(queryHashVal),
				valueSetter:    setString,
			},
			{
				key:            dbPrefix + queryPlanHash,
				valueRetriever: defaultValueRetriever(queryPlanHashVal),
				valueSetter:    setString,
			},
			{
				key:            dbPrefix + queryStart,
				columnName:     queryStart,
				valueRetriever: vanillaRetriever,
				valueSetter:    setString,
			},
			{
				key:            dbPrefix + reads,
				columnName:     reads,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + requestStatus,
				columnName:     requestStatus,
				valueRetriever: vanillaRetriever,
				valueSetter:    setString,
			},
			{
				key:            dbPrefix + rowCount,
				columnName:     rowCount,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + sessionID,
				columnName:     sessionID,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + sessionStatus,
				columnName:     sessionStatus,
				valueRetriever: vanillaRetriever,
				valueSetter:    setString,
			},
			{
				key:        dbPrefix + totalElapsedTime,
				columnName: totalElapsedTime,
				valueRetriever: retrieveIntAndConvert(func(i int64) any {
					return float64(i) / 1000.0
				}),
				valueSetter: setDouble,
			},
			{
				key:            dbPrefix + transactionID,
				columnName:     transactionID,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + transactionIsolationLevel,
				columnName:     transactionIsolationLevel,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + username,
				columnName:     username,
				valueRetriever: vanillaRetriever,
				valueSetter:    setString,
			},
			{
				key:            dbPrefix + waitResource,
				columnName:     waitResource,
				valueRetriever: vanillaRetriever,
				valueSetter:    setString,
			},
			{
				key:        dbPrefix + waitTime,
				columnName: waitTime,
				valueRetriever: retrieveIntAndConvert(func(i int64) any {
					return float64(i) / 1000.0
				}),
				valueSetter: setDouble,
			},
			{
				key:            dbPrefix + waitType,
				columnName:     waitType,
				valueRetriever: vanillaRetriever,
				valueSetter:    setString,
			},
			{
				key:            dbPrefix + writes,
				columnName:     writes,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
		}

		for _, attr := range attributes {
			value, err := attr.valueRetriever(row, attr.columnName)
			if err != nil {
				s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing %s. original value: %s, err: %s", attr.columnName, row[clientPort], err))
			}
			attr.valueSetter(record.Attributes(), attr.key, value)
		}

		// client.address: use host_name if it has value, if not, use client_net_address.
		// this value may not be accurate if
		// - there is proxy in the middle of sql client and sql server. Or
		// - host_name value is empty or not accurate.
		if row[hostName] != "" {
			record.Attributes().PutStr("client.address", row[hostName])
		} else {
			record.Attributes().PutStr("client.address", row[clientAddress])
		}

		record.SetEventName("query sample")

		record.Body().SetStr("sample")
	}
	return logs, errors.Join(errs...)
}
