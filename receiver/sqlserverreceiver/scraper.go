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
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
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
	lb                 *metadata.LogsBuilder
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
		lb:                 metadata.NewLogsBuilder(params),
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
	var err error
	var resources pcommon.Resource
	switch s.sqlQuery {
	case getSQLServerQueryTextAndPlanQuery():
		resources, err = s.recordDatabaseQueryTextAndPlan(ctx, s.config.TopQueryCount)
	case getSQLServerQuerySamplesQuery():
		resources, err = s.recordDatabaseSampleQuery(ctx)
	default:
		return plog.Logs{}, fmt.Errorf("Attempted to get logs from unsupported query: %s", s.sqlQuery)
	}

	return s.lb.Emit(metadata.WithLogsResource(resources)), err
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

func (s *sqlServerScraperHelper) recordDatabaseQueryTextAndPlan(ctx context.Context, topQueryCount uint) (pcommon.Resource, error) {
	// Constants are the column names of the database status
	const (
		dbPrefix       = "sqlserver."
		executionCount = "execution_count"
		logicalReads   = "total_logical_reads"
		logicalWrites  = "total_logical_writes"
		physicalReads  = "total_physical_reads"
		queryHash      = "query_hash"
		queryPlan      = "query_plan"
		queryPlanHash  = "query_plan_hash"
		queryText      = "query_text"
		rowsReturned   = "total_rows"
		// the time returned from mssql is in microsecond
		totalElapsedTime = "total_elapsed_time"
		totalGrant       = "total_grant_kb"
		// the time returned from mssql is in microsecond
		totalWorkerTime = "total_worker_time"
	)

	resources := pcommon.NewResource()

	rows, err := s.client.QueryRows(
		ctx,
		sql.Named("lookbackTime", -s.config.LookbackTime),
		sql.Named("topNValue", s.config.TopQueryCount),
		sql.Named("instanceName", s.config.InstanceName),
	)
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return resources, fmt.Errorf("sqlServerScraperHelper failed getting rows: %w", err)
		}
		s.logger.Warn("problems encountered getting log rows", zap.Error(err))
	}
	var errs []error

	totalElapsedTimeDiffsMicrosecond := make([]int64, len(rows))

	for i, row := range rows {
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))

		elapsedTimeMicrosecond, err := strconv.ParseInt(row[totalElapsedTime], 10, 64)
		if err != nil {
			s.logger.Info(fmt.Sprintf("sqlServerScraperHelper failed getting rows: %s", err))
			errs = append(errs, err)
		} else {
			// we're trying to get the queries that used the most time.
			// caching the total elapsed time (in microsecond) and compare in the next scrape.
			if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, totalElapsedTime, elapsedTimeMicrosecond); cached && diff > 0 {
				totalElapsedTimeDiffsMicrosecond[i] = diff
			}
		}
	}

	// sort the rows based on the totalElapsedTimeDiffs in descending order,
	// only report first T(T=topQueryCount) rows.
	rows = sortRows(rows, totalElapsedTimeDiffsMicrosecond, topQueryCount)

	// sort the totalElapsedTimeDiffs in descending order as well
	sort.Slice(totalElapsedTimeDiffsMicrosecond, func(i, j int) bool { return totalElapsedTimeDiffsMicrosecond[i] > totalElapsedTimeDiffsMicrosecond[j] })

	resourcesAdded := false
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for i, row := range rows {
		// skipping the rest of the rows as totalElapsedTimeDiffs is sorted in descending order
		if totalElapsedTimeDiffsMicrosecond[i] == 0 {
			break
		}

		// reporting human-readable query hash and query hash plan
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))

		record := plog.NewLogRecord()
		record.SetTimestamp(timestamp)
		record.SetEventName("top query")

		attributes := []internalAttribute{
			{
				key:        "db.query.text",
				columnName: queryText,
				valueRetriever: func(row sqlquery.StringMap, columnName string) (any, error) {
					return obfuscateSQL(row[columnName])
				},
				valueSetter: setString,
			},
			{
				key:            dbPrefix + executionCount,
				columnName:     executionCount,
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
				key:            dbPrefix + logicalWrites,
				columnName:     logicalWrites,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + physicalReads,
				columnName:     physicalReads,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + queryHash,
				valueRetriever: defaultValueRetriever(queryHashVal),
				valueSetter:    setString,
			},
			{
				key:        dbPrefix + queryPlan,
				columnName: queryPlan,
				valueRetriever: func(row sqlquery.StringMap, columnName string) (any, error) {
					return obfuscateXMLPlan(row[columnName])
				},
				valueSetter: setString,
			},
			{
				key:            dbPrefix + queryPlanHash,
				valueRetriever: defaultValueRetriever(queryPlanHashVal),
				valueSetter:    setString,
			},
			{
				key:            dbPrefix + rowsReturned,
				columnName:     rowsReturned,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            dbPrefix + totalElapsedTime,
				valueRetriever: defaultValueRetriever(float64(totalElapsedTimeDiffsMicrosecond[i]) / 1_000_000),
				valueSetter:    setDouble,
			},
			{
				key:            dbPrefix + totalGrant,
				columnName:     totalGrant,
				valueRetriever: retrieveInt,
				valueSetter:    setInt,
			},
			{
				key:            "db.system.name",
				valueRetriever: defaultValueRetriever("microsoft.sql_server"),
				valueSetter:    setString,
			},
			{
				key:            serverAddressKey,
				valueRetriever: defaultValueRetriever(s.config.Server),
				valueSetter:    setString,
			},
			{
				key:            serverPortKey,
				valueRetriever: defaultValueRetriever((int64(s.config.Port))),
				valueSetter:    setInt,
			},
		}

		updatedOnly := map[string]bool{
			rowsReturned:   true,
			logicalReads:   true,
			logicalWrites:  true,
			physicalReads:  true,
			executionCount: true,
			totalGrant:     true,
		}

		s.logger.Debug(fmt.Sprintf("QueryHash: %v, PlanHash: %v, DataRow: %v", queryHashVal, queryPlanHashVal, row))

		// handle `total_worker_time`, storing seconds
		// it is a little bit tricky to put this to the array based workflow,
		// as the value need to be divided -> type assertion -> check cache.
		// hence handle it separately.
		workerTimeMicrosecond, err := strconv.ParseInt(row[totalWorkerTime], 10, 64)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		} else {
			if cached, diffMicrosecond := s.cacheAndDiff(queryHashVal, queryPlanHashVal, totalWorkerTime, workerTimeMicrosecond); cached {
				record.Attributes().PutDouble(dbPrefix+totalWorkerTime, float64(diffMicrosecond)/1_000_000)
			}
		}

		for _, attr := range attributes {
			value, err := attr.valueRetriever(row, attr.columnName)
			if err != nil {
				s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing %s. original value: %s, err: %s", attr.columnName, row[attr.columnName], err))
				errs = append(errs, err)
			}
			if _, ok := updatedOnly[attr.columnName]; ok {
				if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, attr.columnName, value.(int64)); cached {
					attr.valueSetter(record.Attributes(), attr.key, diff)
				}
			} else {
				attr.valueSetter(record.Attributes(), attr.key, value)
			}
		}
		if !resourcesAdded {
			resourceAttributes := resources.Attributes()
			resourceAttributes.PutStr("host.name", s.config.Server)
			resourceAttributes.PutStr("sqlserver.computer.name", row[computerNameKey])
			resourceAttributes.PutStr("sqlserver.instance.name", row[instanceNameKey])

			resourcesAdded = true
		}
		s.lb.AppendLogRecord(record)
	}
	return resources, errors.Join(errs...)
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

func (s *sqlServerScraperHelper) recordDatabaseSampleQuery(ctx context.Context) (pcommon.Resource, error) {
	const blockingSessionID = "blocking_session_id"
	const clientAddress = "client_address"
	const clientPort = "client_port"
	const command = "command"
	const contextInfo = "context_info"
	const cpuTimeMillisecond = "cpu_time"
	const dbName = "db_name"
	const dbPrefix = "sqlserver."
	const deadlockPriority = "deadlock_priority"
	const estimatedCompletionTimeMillisecond = "estimated_completion_time"
	const hostName = "host_name"
	const lockTimeoutMillisecond = "lock_timeout"
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
	const totalElapsedTimeMillisecond = "total_elapsed_time"
	const transactionID = "transaction_id"
	const transactionIsolationLevel = "transaction_isolation_level"
	const username = "username"
	const waitResource = "wait_resource"
	const waitTimeMillisecond = "wait_time"
	const waitType = "wait_type"
	const writes = "writes"

	rows, err := s.client.QueryRows(
		ctx,
		sql.Named("top", s.config.TopQueryCount),
	)
	resources := pcommon.NewResource()
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return resources, fmt.Errorf("sqlServerScraperHelper failed getting log rows: %w", err)
		}
		// in case the sql returned rows contains null value, we just log a warning and continue
		s.logger.Warn("problems encountered getting log rows", zap.Error(err))
	}

	var errs []error

	resourcesAdded := false
	propagator := propagation.TraceContext{}

	for _, row := range rows {
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))

		record := plog.NewLogRecord()
		record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

		// Attributes sorted alphabetically by key
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
				key:        dbPrefix + cpuTimeMillisecond,
				columnName: cpuTimeMillisecond,
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
				key:        dbPrefix + estimatedCompletionTimeMillisecond,
				columnName: estimatedCompletionTimeMillisecond,
				valueRetriever: retrieveIntAndConvert(func(i int64) any {
					return float64(i) / 1000.0
				}),
				valueSetter: setDouble,
			},
			{
				key:        dbPrefix + lockTimeoutMillisecond,
				columnName: lockTimeoutMillisecond,
				valueRetriever: retrieveIntAndConvert(func(i int64) any {
					return float64(i) / 1000.0
				}),
				valueSetter: setDouble,
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
				key:        dbPrefix + totalElapsedTimeMillisecond,
				columnName: totalElapsedTimeMillisecond,
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
				key:        dbPrefix + waitTimeMillisecond,
				columnName: waitTimeMillisecond,
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

		spanContext := trace.SpanContextFromContext(propagator.Extract(context.Background(), propagation.MapCarrier{
			"traceparent": row[contextInfo],
		}))

		if spanContext.IsValid() {
			record.SetTraceID(pcommon.TraceID(spanContext.TraceID()))
			record.SetSpanID(pcommon.SpanID(spanContext.SpanID()))
		} else {
			attributes = append(attributes, internalAttribute{
				key:            dbPrefix + contextInfo,
				valueRetriever: defaultValueRetriever(hex.EncodeToString([]byte(row[contextInfo]))),
				valueSetter:    setString,
			})
		}

		for _, attr := range attributes {
			value, err := attr.valueRetriever(row, attr.columnName)
			if err != nil {
				errs = append(errs, err)
				s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing %s. original value: %s, err: %s", attr.columnName, row[attr.columnName], err))
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
		s.lb.AppendLogRecord(record)

		if !resourcesAdded {
			resourceAttributes := resources.Attributes()
			resourceAttributes.PutStr("host.name", s.config.Server)
			resourceAttributes.PutStr("sqlserver.computer.name", row[computerNameKey])
			resourceAttributes.PutStr("sqlserver.instance.name", row[instanceNameKey])

			resourcesAdded = true
		}
	}
	return resources, errors.Join(errs...)
}
