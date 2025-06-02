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
	databaseNameKey  = "database_name"
	instanceNameKey  = "sql_instance"
	serverAddressKey = "server.address"
	serverPortKey    = "server.port"
)

type sqlServerScraperHelper struct {
	id                     component.ID
	config                 *Config
	sqlQuery               string
	instanceName           string
	clientProviderFunc     sqlquery.ClientProviderFunc
	dbProviderFunc         sqlquery.DbProviderFunc
	logger                 *zap.Logger
	telemetry              sqlquery.TelemetryConfig
	client                 sqlquery.DbClient
	db                     *sql.DB
	mb                     *metadata.MetricsBuilder
	lb                     *metadata.LogsBuilder
	cache                  *lru.Cache[string, int64]
	lastExecutionTimestamp time.Time
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
		id:                     id,
		config:                 cfg,
		sqlQuery:               query,
		logger:                 params.Logger,
		telemetry:              telemetry,
		dbProviderFunc:         dbProviderFunc,
		clientProviderFunc:     clientProviderFunc,
		mb:                     metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
		lb:                     metadata.NewLogsBuilder(cfg.LogsBuilderConfig, params),
		cache:                  cache,
		lastExecutionTimestamp: time.Unix(0, 0),
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
	case getSQLServerWaitStatsQuery(s.config.InstanceName):
		err = s.recordDatabaseWaitMetrics(ctx)
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
		if s.lastExecutionTimestamp.Add(s.config.TopQueryCollection.CollectionInterval).After(time.Now()) {
			s.logger.Debug("Skipping the collection of top queries because the current time has not yet exceeded the last execution time plus the specified collection interval")
			return plog.NewLogs(), nil
		}
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
	var val any
	for i, row := range rows {
		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverComputerName(row[computerNameKey])
		rb.SetSqlserverDatabaseName(row[databaseNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])
		rb.SetServerAddress(s.config.Server)
		rb.SetServerPort(int64(s.config.Port))

		val, err = retrieveFloat(row, readLatencyMsKey)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		} else {
			s.mb.RecordSqlserverDatabaseLatencyDataPoint(now, val.(float64)/1e3, row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionRead)
		}

		val, err = retrieveFloat(row, writeLatencyMsKey)
		if err != nil {
			err = fmt.Errorf("row %d: %w", i, err)
			errs = append(errs, err)
		} else {
			s.mb.RecordSqlserverDatabaseLatencyDataPoint(now, val.(float64)/1e3, row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionWrite)
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
	const diskReadIOSec = "Disk Read IO/sec"
	const diskReadIOThrottled = "Disk Read IO Throttled/sec"
	const diskWriteIOSec = "Disk Write IO/sec"
	const diskWriteIOThrottled = "Disk Write IO Throttled/sec"
	const executionErrors = "Execution Errors"
	const freeListStalls = "Free list stalls/sec"
	const freeSpaceInTempdb = "Free Space in tempdb (KB)"
	const fullScansPerSec = "Full Scans/sec"
	const indexSearchesPerSec = "Index Searches/sec"
	const lockTimeoutsPerSec = "Lock Timeouts/sec"
	const lockWaitCount = "Lock Wait Count"
	const lockWaits = "Lock Waits/sec"
	const loginsPerSec = "Logins/sec"
	const logoutPerSec = "Logouts/sec"
	const numberOfDeadlocksPerSec = "Number of Deadlocks/sec"
	const mirrorWritesTransactionPerSec = "Mirrored Write Transactions/sec"
	const memoryGrantsPending = "Memory Grants Pending"
	const pageLifeExpectancy = "Page life expectancy"
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
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, activeTempTables)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTableCountDataPoint(now, val.(int64), metadata.AttributeTableStateActive, metadata.AttributeTableStatusTemporary)
			}
		case backupRestoreThroughputPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, backupRestoreThroughputPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseBackupOrRestoreRateDataPoint(now, val.(float64))
			}
		case batchRequestRate:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, batchRequestRate)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchRequestRateDataPoint(now, val.(float64))
			}
		case bufferCacheHitRatio:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, bufferCacheHitRatio)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageBufferCacheHitRatioDataPoint(now, val.(float64))
			}
		case bytesReceivedFromReplicaPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, bytesReceivedFromReplicaPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverReplicaDataRateDataPoint(now, val.(float64), metadata.AttributeReplicaDirectionReceive)
			}
		case bytesSentForReplicaPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, bytesReceivedFromReplicaPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverReplicaDataRateDataPoint(now, val.(float64), metadata.AttributeReplicaDirectionTransmit)
			}
		case diskReadIOSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, diskReadIOSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverResourcePoolDiskOperationsDataPoint(now, val.(float64), metadata.AttributeDirectionRead)
			}
		case diskReadIOThrottled:
			errs = append(errs, s.mb.RecordSqlserverResourcePoolDiskThrottledReadRateDataPoint(now, row[valueKey]))
		case diskWriteIOSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, diskWriteIOSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverResourcePoolDiskOperationsDataPoint(now, val.(float64), metadata.AttributeDirectionWrite)
			}
		case diskWriteIOThrottled:
			errs = append(errs, s.mb.RecordSqlserverResourcePoolDiskThrottledWriteRateDataPoint(now, row[valueKey]))
		case executionErrors:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, executionErrors)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseExecutionErrorsDataPoint(now, val.(int64))
			}
		case freeListStalls:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, freeListStalls)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageBufferCacheFreeListStallsRateDataPoint(now, val.(int64))
			}
		case fullScansPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, fullScansPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseFullScanRateDataPoint(now, val.(float64))
			}
		case freeSpaceInTempdb:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, freeSpaceInTempdb)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseTempdbSpaceDataPoint(now, val.(int64), metadata.AttributeTempdbStateFree)
			}
		case indexSearchesPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, indexSearchesPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverIndexSearchRateDataPoint(now, val.(float64))
			}
		case lockTimeoutsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockTimeoutsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockTimeoutRateDataPoint(now, val.(float64))
			}
		case lockWaitCount:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockWaitCount)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockWaitCountDataPoint(now, val.(int64))
			}
		case lockWaits:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockWaits)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockWaitRateDataPoint(now, val.(float64))
			}
		case loginsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, loginsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLoginRateDataPoint(now, val.(float64))
			}
		case logoutPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, logoutPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLogoutRateDataPoint(now, val.(float64))
			}
		case memoryGrantsPending:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, memoryGrantsPending)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryGrantsPendingCountDataPoint(now, val.(int64))
			}
		case mirrorWritesTransactionPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, mirrorWritesTransactionPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTransactionMirrorWriteRateDataPoint(now, val.(float64))
			}
		case numberOfDeadlocksPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, numberOfDeadlocksPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDeadlockRateDataPoint(now, val.(float64))
			}
		case pageLifeExpectancy:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, pageLifeExpectancy)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageLifeExpectancyDataPoint(now, val.(int64))
			}
		case pageLookupsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, pageLookupsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageLookupRateDataPoint(now, val.(float64))
			}
		case processesBlocked:
			errs = append(errs, s.mb.RecordSqlserverProcessesBlockedDataPoint(now, row[valueKey]))
		case sqlCompilationRate:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, sqlCompilationRate)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchSQLCompilationRateDataPoint(now, val.(float64))
			}
		case sqlReCompilationsRate:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, sqlReCompilationsRate)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchSQLRecompilationRateDataPoint(now, val.(float64))
			}
		case transactionDelay:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, transactionDelay)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTransactionDelayDataPoint(now, val.(float64))
			}
		case userConnCount:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, userConnCount)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverUserConnectionCountDataPoint(now, val.(int64))
			}
		case usedMemory:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, usedMemory)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryUsageDataPoint(now, val.(float64))
			}
		case versionStoreSize:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, versionStoreSize)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseTempdbVersionStoreSizeDataPoint(now, val.(float64))
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

func (s *sqlServerScraperHelper) recordDatabaseWaitMetrics(ctx context.Context) error {
	// Constants are the columns for metrics from query
	const (
		waitCategory = "wait_category"
		waitTimeMs   = "wait_time_ms"
		waitType     = "wait_type"
	)

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return fmt.Errorf("sqlServerScraperHelper: %w", err)
		}
		s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())
	var val any
	for i, row := range rows {
		rb := s.mb.NewResourceBuilder()
		rb.SetSqlserverDatabaseName(row[databaseNameKey])
		rb.SetSqlserverInstanceName(row[instanceNameKey])
		rb.SetServerAddress(s.config.Server)
		rb.SetServerPort(int64(s.config.Port))

		val, err = retrieveFloat(row, waitTimeMs)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, waitTimeMs))
		} else {
			// The value is divided here because it's stored in SQL Server in ms, need to convert to s
			s.mb.RecordSqlserverOsWaitDurationDataPoint(now, val.(float64)/1e3, row[waitCategory], row[waitType])
		}

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return errors.Join(errs...)
}

func (s *sqlServerScraperHelper) recordDatabaseQueryTextAndPlan(ctx context.Context, topQueryCount uint) (pcommon.Resource, error) {
	// Constants are the column names of the database status
	const (
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

		dbSystemNameVal = "microsoft.sql_server"
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
	now := time.Now()
	timestamp := pcommon.NewTimestampFromTime(now)
	s.lastExecutionTimestamp = now
	for i, row := range rows {
		// skipping the rest of the rows as totalElapsedTimeDiffs is sorted in descending order
		if totalElapsedTimeDiffsMicrosecond[i] == 0 {
			break
		}
		totalElapsedTimeVal := float64(totalElapsedTimeDiffsMicrosecond[i]) / 1_000_000

		// reporting human-readable query hash and query hash plan
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))

		queryTextVal := s.retrieveValue(row, queryText, &errs, func(row sqlquery.StringMap, columnName string) (any, error) {
			statement := row[columnName]
			obfuscated, err := obfuscateSQL(statement)
			if err != nil {
				s.logger.Error(fmt.Sprintf("failed to obfuscate SQL statement: %v", statement))
				return statement, nil
			}

			return obfuscated, nil
		})

		executionCountVal := s.retrieveValue(row, executionCount, &errs, retrieveInt)
		cached, executionCountVal := s.cacheAndDiff(queryHashVal, queryPlanHashVal, executionCount, executionCountVal.(int64))
		if !cached {
			executionCountVal = int64(0)
		}

		logicalReadsVal := s.retrieveValue(row, logicalReads, &errs, retrieveInt)
		cached, logicalReadsVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, logicalReads, logicalReadsVal.(int64))
		if !cached {
			logicalReadsVal = int64(0)
		}

		logicalWritesVal := s.retrieveValue(row, logicalWrites, &errs, retrieveInt)
		cached, logicalWritesVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, logicalWrites, logicalWritesVal.(int64))
		if !cached {
			logicalWritesVal = int64(0)
		}

		physicalReadsVal := s.retrieveValue(row, physicalReads, &errs, retrieveInt)
		cached, physicalReadsVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, physicalReads, physicalReadsVal.(int64))
		if !cached {
			physicalReadsVal = int64(0)
		}

		queryPlanVal := s.retrieveValue(row, queryPlan, &errs, func(row sqlquery.StringMap, columnName string) (any, error) { return obfuscateXMLPlan(row[columnName]) })

		rowsReturnedVal := s.retrieveValue(row, rowsReturned, &errs, retrieveInt)
		cached, rowsReturnedVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, rowsReturned, rowsReturnedVal.(int64))
		if !cached {
			rowsReturnedVal = int64(0)
		}

		totalGrantVal := s.retrieveValue(row, totalGrant, &errs, retrieveInt)
		cached, totalGrantVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, totalGrant, totalGrantVal.(int64))
		if !cached {
			totalGrantVal = int64(0)
		}

		totalWorkerTimeVal := s.retrieveValue(row, totalWorkerTime, &errs, retrieveInt)
		cached, totalWorkerTimeVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, totalWorkerTime, totalWorkerTimeVal.(int64))
		totalWorkerTimeInSecVal := float64(0)
		if cached {
			totalWorkerTimeInSecVal = float64(totalWorkerTimeVal.(int64)) / 1_000_000
		}

		s.logger.Debug(fmt.Sprintf("QueryHash: %v, PlanHash: %v, DataRow: %v", queryHashVal, queryPlanHashVal, row))

		if !resourcesAdded {
			resourceAttributes := resources.Attributes()
			resourceAttributes.PutStr("host.name", s.config.Server)
			resourceAttributes.PutStr("sqlserver.computer.name", row[computerNameKey])
			resourceAttributes.PutStr("sqlserver.instance.name", row[instanceNameKey])

			resourcesAdded = true
		}
		s.lb.RecordDbServerTopQueryEvent(
			context.Background(),
			timestamp,
			totalWorkerTimeInSecVal,
			queryTextVal.(string),
			executionCountVal.(int64),
			logicalReadsVal.(int64),
			logicalWritesVal.(int64),
			physicalReadsVal.(int64),
			queryHashVal,
			queryPlanVal.(string),
			queryPlanHashVal,
			rowsReturnedVal.(int64),
			totalElapsedTimeVal,
			totalGrantVal.(int64),
			s.config.Server,
			int64(s.config.Port),
			dbSystemNameVal)
	}
	return resources, errors.Join(errs...)
}

func (s *sqlServerScraperHelper) retrieveValue(
	row sqlquery.StringMap,
	column string,
	errs *[]error,
	valueRetriever func(sqlquery.StringMap, string) (any, error),
) any {
	value, err := valueRetriever(row, column)
	if err != nil {
		s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing %s. original value: %s, err: %s", column, row[column], err))
		*errs = append(*errs, err)
	}

	return value
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

func retrieveInt(row sqlquery.StringMap, columnName string) (any, error) {
	var err error
	var result int64
	if row[columnName] != "" {
		result, err = strconv.ParseInt(row[columnName], 10, 64)
		if err != nil {
			// SQL Server stores large integers in scientific e notation
			// (eg 123456 is stored as 1.23456e+5)
			// This value cannot be parsed by strconv.ParseInt, but is successfully
			// parsed by strconv.ParseFloat. The goal is here to convert to int
			// even if the stored value is in scientific e notation.
			var resultFloat float64
			resultFloat, err = strconv.ParseFloat(row[columnName], 64)
			if err == nil {
				result = int64(resultFloat)
			}
		}
	} else {
		err = fmt.Errorf("no value found for column %s", columnName)
	}
	return result, err
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
	} else {
		err = fmt.Errorf("no value found for column %s", columnName)
	}
	return result, err
}

func (s *sqlServerScraperHelper) recordDatabaseSampleQuery(ctx context.Context) (pcommon.Resource, error) {
	const blockingSessionID = "blocking_session_id"
	const clientAddress = "client_address"
	const clientPort = "client_port"
	const command = "command"
	const contextInfo = "context_info"
	const cpuTimeMillisecond = "cpu_time"
	const dbName = "db_name"
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
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	dbSystemNameVal := "microsoft.sql_server"

	for _, row := range rows {
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))

		clientPortVal := s.retrieveValue(row, clientPort, &errs, retrieveInt).(int64)
		dbNamespaceVal := row[dbName]
		queryTextVal := s.retrieveValue(row, statementText, &errs, func(row sqlquery.StringMap, columnName string) (any, error) {
			statement := row[columnName]
			obfuscated, err := obfuscateSQL(statement)
			if err != nil {
				s.logger.Error(fmt.Sprintf("failed to obfuscate SQL statement: %v", statement))
				return statement, nil
			}
			return obfuscated, nil
		}).(string)
		networkPeerAddressVal := row[clientAddress]
		networkPeerPortVal := s.retrieveValue(row, clientPort, &errs, retrieveInt).(int64)
		blockSessionIDVal := s.retrieveValue(row, blockingSessionID, &errs, retrieveInt).(int64)
		commandVal := row[command]
		cpuTimeSecondVal := s.retrieveValue(row, cpuTimeMillisecond, &errs, retrieveIntAndConvert(func(i int64) any {
			return float64(i) / 1000.0
		})).(float64)
		deadlockPriorityVal := s.retrieveValue(row, deadlockPriority, &errs, retrieveInt).(int64)
		estimatedCompletionTimeSecondVal := s.retrieveValue(row, estimatedCompletionTimeMillisecond, &errs, retrieveIntAndConvert(func(i int64) any {
			return float64(i) / 1000.0
		})).(float64)
		lockTimeoutSecondVal := s.retrieveValue(row, lockTimeoutMillisecond, &errs, retrieveIntAndConvert(func(i int64) any {
			return float64(i) / 1000.0
		})).(float64)
		logicalReadsVal := s.retrieveValue(row, logicalReads, &errs, retrieveInt).(int64)
		openTransactionCountVal := s.retrieveValue(row, openTransactionCount, &errs, retrieveInt).(int64)
		percentCompleteVal := s.retrieveValue(row, percentComplete, &errs, retrieveFloat).(float64)
		queryStartVal := row[queryStart]
		readsVal := s.retrieveValue(row, reads, &errs, retrieveInt).(int64)
		requestStatusVal := row[requestStatus]
		rowCountVal := s.retrieveValue(row, rowCount, &errs, retrieveInt).(int64)
		sessionIDVal := s.retrieveValue(row, sessionID, &errs, retrieveInt).(int64)
		sessionStatusVal := row[sessionStatus]
		totalElapsedTimeSecondVal := s.retrieveValue(row, totalElapsedTimeMillisecond, &errs, retrieveIntAndConvert(func(i int64) any {
			return float64(i) / 1000.0
		})).(float64)
		transactionIDVal := s.retrieveValue(row, transactionID, &errs, retrieveInt).(int64)
		transactionIsolationLevelVal := s.retrieveValue(row, transactionIsolationLevel, &errs, retrieveInt).(int64)
		usernameVal := row[username]
		waitResourceVal := row[waitResource]
		waitTimeSecondVal := s.retrieveValue(row, waitTimeMillisecond, &errs, retrieveIntAndConvert(func(i int64) any {
			return float64(i) / 1000.0
		})).(float64)
		waitTypeVal := row[waitType]
		writesVal := s.retrieveValue(row, writes, &errs, retrieveInt).(int64)

		contextFromQuery := propagator.Extract(context.Background(), propagation.MapCarrier{
			"traceparent": row[contextInfo],
		})

		spanContext := trace.SpanContextFromContext(contextFromQuery)
		contextInfoVal := ""

		if !spanContext.IsValid() {
			contextInfoVal = hex.EncodeToString([]byte(row[contextInfo]))
		}

		// client.address: use host_name if it has value, if not, use client_net_address.
		// this value may not be accurate if
		// - there is proxy in the middle of sql client and sql server. Or
		// - host_name value is empty or not accurate.
		var clientAddressVal string
		if row[hostName] != "" {
			clientAddressVal = row[hostName]
		} else {
			clientAddressVal = row[clientAddress]
		}

		s.lb.RecordDbServerQuerySampleEvent(
			contextFromQuery,
			timestamp, clientAddressVal, clientPortVal,
			dbNamespaceVal, queryTextVal, dbSystemNameVal,
			networkPeerAddressVal, networkPeerPortVal,
			blockSessionIDVal, contextInfoVal,
			commandVal, cpuTimeSecondVal,
			deadlockPriorityVal, estimatedCompletionTimeSecondVal,
			lockTimeoutSecondVal, logicalReadsVal,
			openTransactionCountVal, percentCompleteVal, queryHashVal, queryPlanHashVal,
			queryStartVal, readsVal,
			requestStatusVal, rowCountVal,
			sessionIDVal, sessionStatusVal,
			totalElapsedTimeSecondVal, transactionIDVal, transactionIsolationLevelVal,
			waitResourceVal, waitTimeSecondVal, waitTypeVal, writesVal, usernameVal,
		)

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
