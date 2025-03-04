// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver"

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
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
	id                 component.ID
	config             *Config
	sqlQuery           string
	instanceName       string
	scrapeCfg          scraperhelper.ControllerConfig
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
	switch s.sqlQuery {
	case getSQLServerQuerySamplesQuery():
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

func (s *sqlServerScraperHelper) recordDatabaseSampleQuery(ctx context.Context) (plog.Logs, error) {
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
		if errors.Is(err, sqlquery.ErrNullValueWarning) {
			// TODO: ignore this for now.
			s.logger.Warn("problems encountered getting log rows", zap.Error(err))
		} else {
			return plog.Logs{}, fmt.Errorf("sqlServerScraperHelper failed getting log rows: %w", err)
		}
	}

	var errs []error
	logs := plog.NewLogs()

	for _, row := range rows {
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))
		contextInfoVal := hex.EncodeToString([]byte(row[contextInfo]))

		cacheKey := queryHashVal + "-" + queryPlanHashVal

		if _, ok := s.cache.Get(cacheKey); !ok {
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
			estimatedCompletionTimeVal, err := strconv.ParseFloat(row[estimatedCompletionTime], 32)
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
			// todo: check case sensitive?
			lockTimeoutVal := 0
			if row[lockTimeout] != "" {
				lockTimeoutVal, err = strconv.Atoi(row[lockTimeout])
				if err != nil {
					s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing lock timeout. original value: %s, err: %s", row[lockTimeout], err))
				}
			}

			deadlockPriorityVal := 0
			if row[deadlockPriority] != "" {
				deadlockPriorityVal, err = strconv.Atoi(row[deadlockPriority])
				if err != nil {
					s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing deadlock priority. original value: %s, err: %s", row[deadlockPriority], err))
				}
			}

			rowCountVal, err := strconv.Atoi(row[rowCount])
			if err != nil {
				s.logger.Error(fmt.Sprintf("sqlServerScraperHelper failed parsing row count. original value: %s, err: %s", row[rowCount], err))
			}

			obfuscatedStatement, err := obfuscateSQL(row[statementText])
			if err != nil {
				s.logger.Error(fmt.Sprintf("failed to obfuscate SQL statement value: %s err: %s", row[statementText], err))
			}

			// TODO: report this value
			record := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			record.Attributes().PutStr(DBName, row[DBName])
			record.Attributes().PutStr(clientAddress, row[clientAddress])
			record.Attributes().PutInt(clientPort, int64(clientPortNumber))
			record.Attributes().PutStr(queryStart, row[queryStart])
			record.Attributes().PutInt(sessionID, int64(sessionIDNumber))
			record.Attributes().PutStr(sessionStatus, row[sessionStatus])
			record.Attributes().PutStr(requestStatus, row[requestStatus])
			record.Attributes().PutStr(hostname, row[hostname])
			record.Attributes().PutStr(command, row[command])
			record.Attributes().PutStr(statementText, obfuscatedStatement)
			record.Attributes().PutInt(blockingSessionID, int64(blockingSessionIDNumber))
			record.Attributes().PutStr(waitType, row[waitType])
			record.Attributes().PutInt(waitTime, int64(waitTimeVal))
			record.Attributes().PutStr(waitResource, row[waitResource])
			record.Attributes().PutInt(openTransactionCount, int64(openTransactionCountVal))
			record.Attributes().PutInt(transactionID, int64(transactionIDVal))
			record.Attributes().PutDouble(percentComplete, percentCompleteVal)
			record.Attributes().PutDouble(estimatedCompletionTime, estimatedCompletionTimeVal)
			record.Attributes().PutInt(cpuTime, int64(cpuTimeVal))
			record.Attributes().PutInt(totalElapsedTime, int64(totalElapsedTimeVal))
			record.Attributes().PutInt(reads, int64(readsVal))
			record.Attributes().PutInt(writes, int64(writesVal))
			record.Attributes().PutInt(logicalReads, int64(logicalReadsVal))
			record.Attributes().PutInt(transactionIsolationLevel, int64(transactionIsolationLevelVal))
			record.Attributes().PutInt(lockTimeout, int64(lockTimeoutVal))
			record.Attributes().PutInt(deadlockPriority, int64(deadlockPriorityVal))
			record.Attributes().PutInt(rowCount, int64(rowCountVal))
			record.Attributes().PutStr(queryHash, queryHashVal)
			record.Attributes().PutStr(queryPlanHash, queryPlanHashVal)
			record.Attributes().PutStr(contextInfo, contextInfoVal)

			record.Attributes().PutStr(username, row[username])

			waitCode, waitCategory := getWaitCategory(row[waitType])
			record.Attributes().PutInt("wait_code", int64(waitCode))
			record.Attributes().PutStr("wait_category", waitCategory)
			record.Body().SetStr("sample")
		} else {
			s.cache.Add(cacheKey, 1)
		}
	}

	return logs, errors.Join(errs...)
}

func anyOf(s string, f func(a string, b string) bool, vals ...string) bool {
	if len(vals) == 0 {
		return false
	}

	for _, v := range vals {
		if f(s, v) {
			return true
		}
	}
	return false
}

func getWaitCategory(s string) (uint, string) {
	if code, exists := detailedWaitTypes[s]; exists {
		return code, waitTypes[code]
	}

	switch {
	case strings.HasPrefix(s, "LOCK_M_"):
		return 3, "Lock"
	case strings.HasPrefix(s, "LATCH_"):
		return 4, "Latch"
	case strings.HasPrefix(s, "PAGELATCH_"):
		return 5, "Buffer Latch"
	case strings.HasPrefix(s, "PAGEIOLATCH_"):
		return 6, "Buffer IO"
	case anyOf(s, strings.HasPrefix, "CLR", "SQLCLR"):
		return 8, "SQL CLR"
	case strings.HasPrefix(s, "DBMIRROR"):
		return 9, "Mirroring"
	case anyOf(s, strings.HasPrefix, "XACT", "DTC", "TRAN_MARKLATCH_", "MSQL_XACT_"):
		return 10, "Transaction"
	case strings.HasPrefix(s, "SLEEP_"):
		return 11, "Idle"
	case strings.HasPrefix(s, "PREEMPTIVE_"):
		return 12, "Preemptive"
	case strings.HasPrefix(s, "BROKER_") && s != "BROKER_RECEIVE_WAITFOR":
		return 13, "Service Broker"
	case anyOf(s, strings.HasPrefix, "HT", "BMP", "BP"):
		return 16, "Parallelism"
	case anyOf(s, strings.HasPrefix, "SE_REPL_", "REPL_", "PWAIT_HADR_"),
		strings.HasPrefix(s, "HADR_") && s != "HADR_THROTTLE_LOG_RATE_GOVERNOR":
		return 22, "Replication"
	case strings.HasPrefix(s, "RBIO_RG_"):
		return 23, "Log Rate Governor"
	default:
		return 0, "Unknown"
	}
}
