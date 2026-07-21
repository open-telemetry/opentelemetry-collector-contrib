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
	"math"
	"sort"
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
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/priorityqueue"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

const (
	computerNameKey = "computer_name"
	databaseNameKey = "database_name"
	instanceNameKey = "sql_instance"

	defaultServiceName = "unknown_service:microsoft.sql_server"
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
	obfuscator             *obfuscator
	serviceInstanceID      string
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
	// Compute service instance ID
	serviceInstanceID, err := computeServiceInstanceID(cfg)
	if err != nil {
		params.Logger.Warn("Failed to compute service.instance.id", zap.Error(err))
		serviceInstanceID = "unknown:1433"
	}

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
		obfuscator:             newObfuscator(),
		serviceInstanceID:      serviceInstanceID,
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
	case getSQLServerAvailabilityGroupQuery(s.config.InstanceName):
		err = s.recordAvailabilityGroupMetrics(ctx)
	case getSQLServerDatabaseIOQuery(s.config.InstanceName):
		err = s.recordDatabaseIOMetrics(ctx)
	case getSQLServerPerformanceCounterQuery(s.config.InstanceName):
		err = s.recordDatabasePerfCounterMetrics(ctx)
	case getSQLServerPropertiesQuery(s.config.InstanceName):
		err = s.recordDatabaseStatusMetrics(ctx)
	case getSQLServerWaitStatsQuery(s.config.InstanceName):
		err = s.recordDatabaseWaitMetrics(ctx)
	case getSQLServerWorkerThreadsQuery(s.config.InstanceName):
		err = s.recordWorkerThreadMetrics(ctx)
	case getSQLServerIndexPhysicalStatsQuery(s.config.InstanceName):
		err = s.recordIndexPhysicalMetrics(ctx)
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
	var isQuerySample bool
	switch s.sqlQuery {
	case getSQLServerQueryTextAndPlanQuery():
		if int(math.Ceil(time.Since(s.lastExecutionTimestamp).Seconds())) < int(s.config.TopQueryCollection.CollectionInterval.Seconds()) {
			s.logger.Debug("Skipping the collection of top queries because the current time has not yet exceeded the last execution time plus the specified collection interval")
			return plog.NewLogs(), nil
		}
		resources, err = s.recordDatabaseQueryTextAndPlan(ctx)
	case getSQLServerQuerySamplesQuery():
		isQuerySample = true
		resources, err = s.recordDatabaseSampleQuery(ctx)
	default:
		return plog.Logs{}, fmt.Errorf("Attempted to get logs from unsupported query: %s", s.sqlQuery)
	}

	logs := s.lb.Emit(metadata.WithLogsResource(resources))
	if isQuerySample {
		sanitizeQuerySampleOptionalAttributes(logs)
	}

	return logs, err
}

func sanitizeQuerySampleOptionalAttributes(logs plog.Logs) {
	resourceLogs := logs.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		scopeLogs := resourceLogs.At(i).ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			logRecords := scopeLogs.At(j).LogRecords()
			for k := 0; k < logRecords.Len(); k++ {
				logRecord := logRecords.At(k)
				if logRecord.EventName() != "db.server.query_sample" {
					continue
				}

				blockingSessionIDAttr, hasBlockingSession := logRecord.Attributes().Get("sqlserver.blocking_session_id")
				blockingStartTimeAttr, hasBlockingStartTime := logRecord.Attributes().Get("sqlserver.blocking.start_time")
				if !hasBlockingSession || !hasBlockingStartTime || blockingSessionIDAttr.Int() <= 0 || blockingStartTimeAttr.Str() == "" {
					logRecord.Attributes().Remove("sqlserver.blocking.start_time")
				}
				resourceTypeAttr, hasResourceType := logRecord.Attributes().Get("sqlserver.wait.resource.type")
				if !hasResourceType || resourceTypeAttr.Str() == "" {
					logRecord.Attributes().Remove("sqlserver.wait.resource.type")
				}
				resourceIDAttr, hasResourceID := logRecord.Attributes().Get("sqlserver.wait.resource.id")
				if !hasResourceID || resourceIDAttr.Str() == "" {
					logRecord.Attributes().Remove("sqlserver.wait.resource.id")
				}
			}
		}
	}
}

func parseWaitResource(waitResource string) (resourceType, resourceID string) {
	if waitResource == "" {
		return "", ""
	}
	waitResource = strings.TrimSpace(waitResource)
	if waitResource == "" {
		return "", ""
	}

	sep := strings.IndexByte(waitResource, ':')
	if sep <= 0 {
		return "", ""
	}
	resourceType = waitResource[:sep]
	rest := strings.TrimSpace(waitResource[sep+1:])
	if rest == "" {
		return "", ""
	}
	if space := strings.IndexByte(rest, ' '); space >= 0 {
		rest = rest[:space]
	}
	if rest == "" {
		return "", ""
	}

	switch resourceType {
	case "DATABASE":
		if !isDigits(rest) {
			return "", ""
		}
		return resourceType, rest
	case "FILE", "KEY":
		_, second, ok := splitTwoSegments(rest)
		if !ok || !isDigits(second) {
			return "", ""
		}
		return resourceType, second
	case "PAGE", "OBJECT":
		tail, ok := splitAfterFirstSegment(rest)
		if !ok || !isTwoNumericSegments(tail) {
			return "", ""
		}
		return resourceType, tail
	case "RID":
		tail, ok := splitAfterFirstSegment(rest)
		if !ok || !isThreeNumericSegments(tail) {
			return "", ""
		}
		return resourceType, tail
	default:
		return "", ""
	}
}

func splitTwoSegments(s string) (first, second string, ok bool) {
	sep := strings.IndexByte(s, ':')
	if sep <= 0 || sep >= len(s)-1 {
		return "", "", false
	}
	if strings.IndexByte(s[sep+1:], ':') >= 0 {
		return "", "", false
	}
	first, second = s[:sep], s[sep+1:]
	if !isDigits(first) {
		return "", "", false
	}
	return first, second, true
}

func splitAfterFirstSegment(s string) (tail string, ok bool) {
	sep := strings.IndexByte(s, ':')
	if sep <= 0 || sep >= len(s)-1 {
		return "", false
	}
	first := s[:sep]
	tail = s[sep+1:]
	if !isDigits(first) {
		return "", false
	}
	return tail, true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func isTwoNumericSegments(s string) bool {
	first, second, ok := splitTwoSegments(s)
	return ok && isDigits(first) && isDigits(second)
}

func isThreeNumericSegments(s string) bool {
	firstSep := strings.IndexByte(s, ':')
	if firstSep <= 0 || firstSep >= len(s)-1 {
		return false
	}
	secondSepRel := strings.IndexByte(s[firstSep+1:], ':')
	if secondSepRel <= 0 {
		return false
	}
	secondSep := firstSep + 1 + secondSepRel
	if secondSep >= len(s)-1 {
		return false
	}
	if strings.IndexByte(s[secondSep+1:], ':') >= 0 {
		return false
	}
	return isDigits(s[:firstSep]) && isDigits(s[firstSep+1:secondSep]) && isDigits(s[secondSep+1:])
}

func (s *sqlServerScraperHelper) Shutdown(context.Context) error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// setupResourceBuilder configures common resource attributes for metrics and logs.
func (s *sqlServerScraperHelper) setupResourceBuilder(rb *metadata.ResourceBuilder, row sqlquery.StringMap) *metadata.ResourceBuilder {
	rb.SetSqlserverComputerName(row[computerNameKey])
	rb.SetSqlserverInstanceName(row[instanceNameKey])

	hostName := s.config.Server
	serverAddress := s.config.Server
	serverPort := int64(s.config.Port)

	if s.config.DataSource != "" {
		host, port, err := parseDataSource(s.config.DataSource)
		if err != nil {
			s.logger.Warn("Failed to parse datasource for host.name attribute, using fallback", zap.Error(err))
		} else {
			hostName = host
			serverAddress = host
			serverPort = int64(port)
		}
	}

	rb.SetHostName(hostName)
	rb.SetServiceInstanceID(s.serviceInstanceID)
	rb.SetServiceName(defaultServiceName)
	rb.SetServiceNamespace("")

	if !metadata.ReceiverSqlserverRemoveServerResourceAttributeFeatureGate.IsEnabled() {
		rb.SetServerAddress(serverAddress)
		rb.SetServerPort(serverPort)
	}

	return rb
}

func (s *sqlServerScraperHelper) recordAvailabilityGroupMetrics(ctx context.Context) error {
	const (
		agNameKey           = "availability_group_name"
		logSendQueueSizeKey = "log_send_queue_size"
		logSendRateKey      = "log_send_rate"
		redoQueueSizeKey    = "redo_queue_size"
		redoRateKey         = "redo_rate"
		replicaNameKey      = "replica_name"
		secondaryLagKey     = "secondary_lag"
	)

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return fmt.Errorf("sqlServerScraperHelper: %w", err)
		}
		s.logger.Debug("problems encountered getting metric rows", zap.Error(err))
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())
	var val any
	for i, row := range rows {
		rb := s.setupResourceBuilder(s.mb.NewResourceBuilder(), row)
		rb.SetSqlserverDatabaseName(row[databaseNameKey])

		agName := row[agNameKey]
		replicaName := row[replicaNameKey]

		if row[logSendQueueSizeKey] != "" {
			val, err = retrieveInt(row, logSendQueueSizeKey)
			if err != nil {
				errs = append(errs, fmt.Errorf("row %d: %w", i, err))
			} else {
				s.mb.RecordSqlserverAvailabilityGroupDatabaseReplicaQueueSizeDataPoint(now, val.(int64)*1024, agName, replicaName, metadata.AttributeSqlserverAvailabilityGroupQueueTypeLogSend)
			}
		}

		if row[logSendRateKey] != "" {
			val, err = retrieveInt(row, logSendRateKey)
			if err != nil {
				errs = append(errs, fmt.Errorf("row %d: %w", i, err))
			} else {
				s.mb.RecordSqlserverAvailabilityGroupDatabaseReplicaQueueRateDataPoint(now, val.(int64)*1024, agName, replicaName, metadata.AttributeSqlserverAvailabilityGroupQueueTypeLogSend)
			}
		}

		if row[redoQueueSizeKey] != "" {
			val, err = retrieveInt(row, redoQueueSizeKey)
			if err != nil {
				errs = append(errs, fmt.Errorf("row %d: %w", i, err))
			} else {
				s.mb.RecordSqlserverAvailabilityGroupDatabaseReplicaQueueSizeDataPoint(now, val.(int64)*1024, agName, replicaName, metadata.AttributeSqlserverAvailabilityGroupQueueTypeRedo)
			}
		}

		if row[redoRateKey] != "" {
			val, err = retrieveInt(row, redoRateKey)
			if err != nil {
				errs = append(errs, fmt.Errorf("row %d: %w", i, err))
			} else {
				s.mb.RecordSqlserverAvailabilityGroupDatabaseReplicaQueueRateDataPoint(now, val.(int64)*1024, agName, replicaName, metadata.AttributeSqlserverAvailabilityGroupQueueTypeRedo)
			}
		}

		// secondary_lag_seconds is NULL on SQL Server < 2016
		if row[secondaryLagKey] != "" {
			val, err = retrieveFloat(row, secondaryLagKey)
			if err != nil {
				errs = append(errs, fmt.Errorf("row %d: %w", i, err))
			} else {
				s.mb.RecordSqlserverAvailabilityGroupDatabaseReplicaSecondaryLagDataPoint(now, val.(float64), agName, replicaName)
			}
		}

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	if len(rows) == 0 {
		s.logger.Info("SQLServerScraperHelper: No rows found by query")
	}

	return errors.Join(errs...)
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
		rb := s.setupResourceBuilder(s.mb.NewResourceBuilder(), row)
		rb.SetSqlserverDatabaseName(row[databaseNameKey])

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

		errs = append(errs,
			s.mb.RecordSqlserverDatabaseOperationsDataPoint(now, row[readCountKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionRead),
			s.mb.RecordSqlserverDatabaseOperationsDataPoint(now, row[writeCountKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionWrite),
			s.mb.RecordSqlserverDatabaseIoDataPoint(now, row[readBytesKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionRead),
			s.mb.RecordSqlserverDatabaseIoDataPoint(now, row[writeBytesKey], row[physicalFilenameKey], row[logicalFilenameKey], row[fileTypeKey], metadata.AttributeDirectionWrite))

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
	const autoParamAttemptsPerSec = "Auto-Param Attempts/sec"
	const backupRestoreThroughputPerSec = "Backup/Restore Throughput/sec"
	const batchRequestRate = "Batch Requests/sec"
	const bufferCacheHitRatio = "Buffer cache hit ratio"
	const bytesReceivedFromReplicaPerSec = "Bytes Received from Replica/sec"
	const bytesSentForReplicaPerSec = "Bytes Sent to Replica/sec"
	const connectionResetPerSec = "Connection Reset/sec"
	const diskReadIOSec = "Disk Read IO/sec"
	const diskReadIOThrottled = "Disk Read IO Throttled/sec"
	const diskWriteIOSec = "Disk Write IO/sec"
	const diskWriteIOThrottled = "Disk Write IO Throttled/sec"
	const errorsPerSec = "Errors/sec"
	const executionErrors = "Execution Errors"
	const extentDeallocationsPerSec = "Extent Deallocations/sec"
	const extentsAllocatedPerSec = "Extents Allocated/sec"
	const failedAutoParamsPerSec = "Failed Auto-Params/sec"
	const forcedParameterizationsPerSec = "Forced Parameterizations/sec"
	const freeListStalls = "Free list stalls/sec"
	const freeSpaceInTempdb = "Free Space in tempdb (KB)"
	const freeSpaceScansPerSec = "FreeSpace Scans/sec"
	const fullScansPerSec = "Full Scans/sec"
	const guidedPlanExecutionsPerSec = "Guided plan executions/sec"
	const indexSearchesPerSec = "Index Searches/sec"
	const lockBlocks = "Lock Blocks"
	const lockBlocksAllocated = "Lock Blocks Allocated"
	const lockMemoryKB = "Lock Memory (KB)"
	const lockOwnerBlocks = "Lock Owner Blocks"
	const lockOwnerBlocksAllocated = "Lock Owner Blocks Allocated"
	const lockRequestsPerSec = "Lock Requests/sec"
	const lockTimeoutsNonzeroPerSec = "Lock Timeouts (timeout > 0)/sec"
	const lockTimeoutsPerSec = "Lock Timeouts/sec"
	const lockWaitCount = "Lock Wait Count"
	const lockWaitTimeMS = "Lock Wait Time (ms)"
	const lockWaits = "Lock Waits/sec"
	const loginsPerSec = "Logins/sec"
	const logoutPerSec = "Logouts/sec"
	const misguidedPlanExecutionsPerSec = "Misguided plan executions/sec"
	const numberOfDeadlocksPerSec = "Number of Deadlocks/sec"
	const mirrorWritesTransactionPerSec = "Mirrored Write Transactions/sec"
	const memoryGrantsPending = "Memory Grants Pending"
	const mixedPageAllocationsPerSec = "Mixed page allocations/sec"
	const pageCompressionAttemptsPerSec = "Page Compression Attempts/sec"
	const pageDeallocationsPerSec = "Page Deallocations/sec"
	const pageLifeExpectancy = "Page life expectancy"
	const pageLookupsPerSec = "Page lookups/sec"
	const pagesAllocatedPerSec = "Pages Allocated/sec"
	const pagesCompressedPerSec = "Pages Compressed/sec"
	const probeScansPerSec = "Probe Scans/sec"
	const processesBlocked = "Processes blocked"
	const rangeScansPerSec = "Range Scans/sec"
	const readaheadPagesPerSec = "Readahead pages/sec"
	const safeAutoParamsPerSec = "Safe Auto-Params/sec"
	const scanPointRevalidationsPerSec = "Scan Point Revalidations/sec"
	const skippedGhostedRecordsPerSec = "Skipped Ghosted Records/sec"
	const sqlAttentionRate = "SQL Attention rate"
	const sqlCompilationRate = "SQL Compilations/sec"
	const sqlReCompilationsRate = "SQL Re-Compilations/sec"
	const tableLockEscalationsPerSec = "Table Lock Escalations/sec"
	const transactionDelay = "Transaction Delay"
	const unsafeAutoParamsPerSec = "Unsafe Auto-Params/sec"
	const userConnCount = "User Connections"
	const usedMemory = "Used memory (KB)"
	const versionStoreSize = "Version Store Size (KB)"
	const sqlCacheMemory = "SQL Cache Memory (KB)"
	const optimizerMemory = "Optimizer Memory (KB)"
	const connectionMemory = "Connection Memory (KB)"
	const grantedWorkspaceMemory = "Granted Workspace Memory (KB)"
	const maximumWorkspaceMemory = "Maximum Workspace Memory (KB)"
	const targetServerMemory = "Target Server Memory (KB)"
	const totalServerMemory = "Total Server Memory (KB)"
	const cachePages = "Cache Pages"
	const totalPages = "Total Pages"
	const targetPages = "Target Pages"
	const databasePages = "Database pages"
	const stolenPages = "Stolen Pages"
	const reservedPages = "Reserved Pages"
	const freePages = "Free Pages"
	const cacheObjectCounts = "Cache Object Counts"
	const cacheObjectsInUse = "Cache Objects in use"
	const averageLatchWaitTime = "Average Latch Wait Time (ms)"
	const latchWaitsPerSec = "Latch Waits/sec"
	const numberOfSuperLatches = "Number of SuperLatches"
	const superLatchDemotionsPerSec = "SuperLatch Demotions/sec"
	const superLatchPromotionsPerSec = "SuperLatch Promotions/sec"
	const totalLatchWaitTime = "Total Latch Wait Time (ms)"
	const worktablesFromCacheRatio = "Worktables From Cache Ratio"
	const activeCursors = "Active cursors"
	const cachedCursorCounts = "Cached Cursor Counts"
	const cursorMemoryUsage = "Cursor memory usage"
	const cursorRequestsPerSec = "Cursor Requests/sec"
	const numberOfActiveCursorPlans = "Number of active cursor plans"
	const clrExecution = "CLR Execution"
	const tasksRunning = "Tasks Running"
	const taskLimitReached = "Task Limit Reached"
	const tasksStartedPerSec = "Tasks Started/sec"
	const tasksAbortedPerSec = "Tasks Aborted/sec"
	const storedProceduresInvokedPerSec = "Stored Procedures Invoked/sec"

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return fmt.Errorf("sqlServerScraperHelper: %w", err)
		}
		s.logger.Warn("problems encountered getting metric rows", zap.Error(err))
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())

	// Track SQL compilation and recompilation rates so the derived
	// sqlserver.recompilation.ratio metric can be emitted after the row loop.
	var (
		compRate, recompRate float64
		compSeen, recompSeen bool
		recompRatioRow       sqlquery.StringMap
	)

	for i, row := range rows {
		rb := s.setupResourceBuilder(s.mb.NewResourceBuilder(), row)

		switch row[counterKey] {
		case activeCursors:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, activeCursors)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverCursorCountDataPoint(now, val.(int64), metadata.AttributeCursorStateActive)
			}
		case activeTempTables:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, activeTempTables)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTableCountDataPoint(now, val.(int64), metadata.AttributeTableStateActive, metadata.AttributeTableStatusTemporary)
			}
		case autoParamAttemptsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, autoParamAttemptsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverParameterizationRateDataPoint(now, val.(float64), metadata.AttributeSqlserverParameterizationResultAutoAttempted)
			}
		case averageLatchWaitTime:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, averageLatchWaitTime)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLatchWaitTimeAvgDataPoint(now, val.(float64)/1000.0)
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
		case cacheObjectCounts:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, cacheObjectCounts)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryCacheObjectCountDataPoint(now, val.(int64), metadata.AttributeCacheStateTotal)
			}
		case cacheObjectsInUse:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, cacheObjectsInUse)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryCacheObjectCountDataPoint(now, val.(int64), metadata.AttributeCacheStateInUse)
			}
		case cachePages:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, cachePages)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryPageCountDataPoint(now, val.(int64), metadata.AttributePagePoolCache)
			}
		case cachedCursorCounts:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, cachedCursorCounts)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverCursorCountDataPoint(now, val.(int64), metadata.AttributeCursorStateCached)
			}
		case clrExecution:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, clrExecution)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverClrExecutionTimeDataPoint(now, float64(val.(int64))/1_000_000.0)
			}
		case connectionMemory:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, connectionMemory)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryAreaDataPoint(now, val.(int64)*1024, metadata.AttributeMemoryPoolConnection)
			}
		case connectionResetPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, connectionResetPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverConnectionResetRateDataPoint(now, val.(float64))
			}
		case cursorMemoryUsage:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, cursorMemoryUsage)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverCursorMemoryUsageDataPoint(now, val.(int64)*1024)
			}
		case cursorRequestsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, cursorRequestsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverCursorRequestRateDataPoint(now, val.(float64))
			}
		case databasePages:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, databasePages)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryPageCountDataPoint(now, val.(int64), metadata.AttributePagePoolDatabase)
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
		case errorsPerSec:
			category, ok := errorCategoryAttr(row["instance"])
			if !ok {
				break
			}
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, errorsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverErrorRateDataPoint(now, val.(float64), category)
			}
		case executionErrors:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, executionErrors)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseExecutionErrorsDataPoint(now, val.(int64))
			}
		case extentDeallocationsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, extentDeallocationsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverExtentOperationRateDataPoint(now, val.(float64), metadata.AttributeSqlserverExtentOperationTypeDeallocated)
			}
		case extentsAllocatedPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, extentsAllocatedPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverExtentOperationRateDataPoint(now, val.(float64), metadata.AttributeSqlserverExtentOperationTypeAllocated)
			}
		case failedAutoParamsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, failedAutoParamsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverParameterizationRateDataPoint(now, val.(float64), metadata.AttributeSqlserverParameterizationResultFailed)
			}
		case forcedParameterizationsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, forcedParameterizationsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverParameterizationRateDataPoint(now, val.(float64), metadata.AttributeSqlserverParameterizationResultForced)
			}
		case freeListStalls:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, freeListStalls)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageBufferCacheFreeListStallsRateDataPoint(now, val.(int64))
			}
		case freePages:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, freePages)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryPageCountDataPoint(now, val.(int64), metadata.AttributePagePoolFree)
			}
		case freeSpaceInTempdb:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, freeSpaceInTempdb)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseTempdbSpaceDataPoint(now, val.(int64), metadata.AttributeTempdbStateFree)
			}
		case freeSpaceScansPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, freeSpaceScansPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverAccessScanRateDataPoint(now, val.(float64), metadata.AttributeSqlserverAccessScanTypeFreeSpace)
			}
		case fullScansPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, fullScansPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseFullScanRateDataPoint(now, val.(float64))
			}
		case grantedWorkspaceMemory:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, grantedWorkspaceMemory)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryAreaDataPoint(now, val.(int64)*1024, metadata.AttributeMemoryPoolGrantedWorkspace)
			}
		case guidedPlanExecutionsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, guidedPlanExecutionsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPlanExecutionRateDataPoint(now, val.(float64), metadata.AttributeSqlserverPlanGuidanceResultGuided)
			}
		case indexSearchesPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, indexSearchesPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverIndexSearchRateDataPoint(now, val.(float64))
			}
		case latchWaitsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, latchWaitsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLatchWaitRateDataPoint(now, val.(float64))
			}
		case lockBlocks:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockBlocks)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockBlockCountDataPoint(now, val.(int64), metadata.AttributeSqlserverLockBlockTypeBlocks)
			}
		case lockBlocksAllocated:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockBlocksAllocated)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockBlockCountDataPoint(now, val.(int64), metadata.AttributeSqlserverLockBlockTypeAllocated)
			}
		case lockMemoryKB:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockMemoryKB)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockMemoryDataPoint(now, val.(int64)*1024)
			}
		case lockOwnerBlocks:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockOwnerBlocks)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockBlockCountDataPoint(now, val.(int64), metadata.AttributeSqlserverLockBlockTypeOwner)
			}
		case lockOwnerBlocksAllocated:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockOwnerBlocksAllocated)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockBlockCountDataPoint(now, val.(int64), metadata.AttributeSqlserverLockBlockTypeOwnerAllocated)
			}
		case lockRequestsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockRequestsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockRequestRateDataPoint(now, val.(float64))
			}
		case lockTimeoutsNonzeroPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockTimeoutsNonzeroPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockTimeoutRateDataPoint(now, val.(float64), metadata.AttributeSqlserverLockTimeoutTypeNonzero)
			}
		case lockTimeoutsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockTimeoutsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockTimeoutRateDataPoint(now, val.(float64), metadata.AttributeSqlserverLockTimeoutTypeAll)
			}
		case lockWaitCount:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockWaitCount)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockWaitCountDataPoint(now, val.(int64))
			}
		case lockWaitTimeMS:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, lockWaitTimeMS)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockWaitTimeTotalDataPoint(now, val.(float64)/1000.0)
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
		case maximumWorkspaceMemory:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, maximumWorkspaceMemory)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryAreaDataPoint(now, val.(int64)*1024, metadata.AttributeMemoryPoolMaxWorkspace)
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
		case misguidedPlanExecutionsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, misguidedPlanExecutionsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPlanExecutionRateDataPoint(now, val.(float64), metadata.AttributeSqlserverPlanGuidanceResultMisguided)
			}
		case mixedPageAllocationsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, mixedPageAllocationsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageAllocationRateDataPoint(now, val.(float64), metadata.AttributeSqlserverPageAllocationTypeMixed)
			}
		case numberOfActiveCursorPlans:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, numberOfActiveCursorPlans)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverCursorPlanCountDataPoint(now, val.(int64))
			}
		case numberOfDeadlocksPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, numberOfDeadlocksPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDeadlockRateDataPoint(now, val.(float64))
			}
		case numberOfSuperLatches:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, numberOfSuperLatches)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLatchSuperlatchCountDataPoint(now, val.(int64))
			}
		case optimizerMemory:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, optimizerMemory)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryAreaDataPoint(now, val.(int64)*1024, metadata.AttributeMemoryPoolOptimizer)
			}
		case pageCompressionAttemptsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, pageCompressionAttemptsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageCompressionRateDataPoint(now, val.(float64), metadata.AttributeSqlserverPageCompressionTypeAttempted)
			}
		case pageDeallocationsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, pageDeallocationsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageAllocationRateDataPoint(now, val.(float64), metadata.AttributeSqlserverPageAllocationTypeDeallocated)
			}
		case pageLifeExpectancy:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, pageLifeExpectancy)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageLifeExpectancyDataPoint(now, val.(int64), row["object"])
			}
		case pageLookupsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, pageLookupsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageLookupRateDataPoint(now, val.(float64))
			}
		case pagesAllocatedPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, pagesAllocatedPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageAllocationRateDataPoint(now, val.(float64), metadata.AttributeSqlserverPageAllocationTypeAllocated)
			}
		case pagesCompressedPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, pagesCompressedPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageCompressionRateDataPoint(now, val.(float64), metadata.AttributeSqlserverPageCompressionTypeSucceeded)
			}
		case probeScansPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, probeScansPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverAccessScanRateDataPoint(now, val.(float64), metadata.AttributeSqlserverAccessScanTypeProbe)
			}
		case processesBlocked:
			errs = append(errs, s.mb.RecordSqlserverProcessesBlockedDataPoint(now, row[valueKey]))
		case rangeScansPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, rangeScansPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverAccessScanRateDataPoint(now, val.(float64), metadata.AttributeSqlserverAccessScanTypeRange)
			}
		case readaheadPagesPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, readaheadPagesPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverPageReadAheadRateDataPoint(now, val.(float64))
			}
		case reservedPages:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, reservedPages)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryPageCountDataPoint(now, val.(int64), metadata.AttributePagePoolReserved)
			}
		case safeAutoParamsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, safeAutoParamsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverParameterizationRateDataPoint(now, val.(float64), metadata.AttributeSqlserverParameterizationResultSafe)
			}
		case scanPointRevalidationsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, scanPointRevalidationsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverScanPointRevalidationRateDataPoint(now, val.(float64))
			}
		case skippedGhostedRecordsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, skippedGhostedRecordsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverGhostRecordSkippedRateDataPoint(now, val.(float64))
			}
		case sqlAttentionRate:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, sqlAttentionRate)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverAttentionRateDataPoint(now, val.(float64))
			}
		case sqlCacheMemory:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, sqlCacheMemory)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryAreaDataPoint(now, val.(int64)*1024, metadata.AttributeMemoryPoolSQLCache)
			}
		case sqlCompilationRate:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, sqlCompilationRate)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchSQLCompilationRateDataPoint(now, val.(float64))
				compRate = val.(float64)
				compSeen = true
				recompRatioRow = row
			}
		case sqlReCompilationsRate:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, sqlReCompilationsRate)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverBatchSQLRecompilationRateDataPoint(now, val.(float64))
				recompRate = val.(float64)
				recompSeen = true
				if recompRatioRow == nil {
					recompRatioRow = row
				}
			}
		case stolenPages:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, stolenPages)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryPageCountDataPoint(now, val.(int64), metadata.AttributePagePoolStolen)
			}
		case storedProceduresInvokedPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, storedProceduresInvokedPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverStoredProcedureInvocationRateDataPoint(now, val.(float64))
			}
		case superLatchDemotionsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, superLatchDemotionsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLatchSuperlatchTransitionRateDataPoint(now, val.(float64), metadata.AttributeTransitionDirectionDemotion)
			}
		case superLatchPromotionsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, superLatchPromotionsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLatchSuperlatchTransitionRateDataPoint(now, val.(float64), metadata.AttributeTransitionDirectionPromotion)
			}
		case tableLockEscalationsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, tableLockEscalationsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLockEscalationRateDataPoint(now, val.(float64))
			}
		case targetPages:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, targetPages)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryPageCountDataPoint(now, val.(int64), metadata.AttributePagePoolTarget)
			}
		case targetServerMemory:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, targetServerMemory)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryAreaDataPoint(now, val.(int64)*1024, metadata.AttributeMemoryPoolTarget)
			}
		case taskLimitReached:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, taskLimitReached)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTaskCountDataPoint(now, val.(int64), metadata.AttributeTaskStateLimitReached)
			}
		case tasksAbortedPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, tasksAbortedPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTaskRateDataPoint(now, val.(float64), metadata.AttributeTaskResultAborted)
			}
		case tasksRunning:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, tasksRunning)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTaskCountDataPoint(now, val.(int64), metadata.AttributeTaskStateRunning)
			}
		case tasksStartedPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, tasksStartedPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTaskRateDataPoint(now, val.(float64), metadata.AttributeTaskResultStarted)
			}
		case totalLatchWaitTime:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, totalLatchWaitTime)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverLatchWaitTimeTotalDataPoint(now, val.(float64)/1000.0)
			}
		case totalPages:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, totalPages)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryPageCountDataPoint(now, val.(int64), metadata.AttributePagePoolTotal)
			}
		case totalServerMemory:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, totalServerMemory)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryAreaDataPoint(now, val.(int64)*1024, metadata.AttributeMemoryPoolTotal)
			}
		case transactionDelay:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, transactionDelay)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverTransactionDelayDataPoint(now, val.(float64))
			}
		case unsafeAutoParamsPerSec:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, unsafeAutoParamsPerSec)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverParameterizationRateDataPoint(now, val.(float64), metadata.AttributeSqlserverParameterizationResultUnsafe)
			}
		case usedMemory:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, usedMemory)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverMemoryUsageDataPoint(now, val.(float64))
			}
		case userConnCount:
			val, err := retrieveInt(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, userConnCount)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverUserConnectionCountDataPoint(now, val.(int64))
			}
		case versionStoreSize:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, versionStoreSize)
				errs = append(errs, err)
			} else {
				s.mb.RecordSqlserverDatabaseTempdbVersionStoreSizeDataPoint(now, val.(float64))
			}
		case worktablesFromCacheRatio:
			val, err := retrieveFloat(row, valueKey)
			if err != nil {
				err = fmt.Errorf("failed to parse valueKey for row %d: %w in %s", i, err, worktablesFromCacheRatio)
				errs = append(errs, err)
			} else {
				// The query returns this ratio counter as a percentage (0-100); emit it as a 0-1 fraction.
				s.mb.RecordSqlserverWorktableCacheHitRatioDataPoint(now, val.(float64)/100)
			}
		}

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	// Emit derived sqlserver.recompilation.ratio metric (recomp / comp * 100)
	// once both source counters have been observed in this scrape cycle.
	if compSeen && recompSeen && compRate > 0 {
		rb := s.setupResourceBuilder(s.mb.NewResourceBuilder(), recompRatioRow)
		s.mb.RecordSqlserverRecompilationRatioDataPoint(now, recompRate/compRate*100)
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
	const cpuCount = "cpu_count"
	const computerUptime = "computer_uptime"

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
		rb := s.setupResourceBuilder(s.mb.NewResourceBuilder(), row)

		errs = append(errs,
			s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbOnline], metadata.AttributeDatabaseStatusOnline),
			s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbRestoring], metadata.AttributeDatabaseStatusRestoring),
			s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbRecovering], metadata.AttributeDatabaseStatusRecovering),
			s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbPendingRecovery], metadata.AttributeDatabaseStatusPendingRecovery),
			s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbSuspect], metadata.AttributeDatabaseStatusSuspect),
			s.mb.RecordSqlserverDatabaseCountDataPoint(now, row[dbOffline], metadata.AttributeDatabaseStatusOffline),
			s.mb.RecordSqlserverCPUCountDataPoint(now, row[cpuCount]),
			s.mb.RecordSqlserverComputerUptimeDataPoint(now, row[computerUptime]),
		)

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
		rb := s.setupResourceBuilder(s.mb.NewResourceBuilder(), row)
		rb.SetSqlserverDatabaseName(row[databaseNameKey])

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

func (s *sqlServerScraperHelper) recordIndexPhysicalMetrics(ctx context.Context) error {
	const (
		fragKey             = "avg_fragmentation_in_percent"
		indexIDKey          = "index_id"
		objectNameKey       = "object_name"
		pageCountKey        = "page_count"
		pageSpaceUsedKey    = "avg_page_space_used_in_percent"
		recordCountKey      = "record_count"
		schemaNameKey       = "schema_name"
		sqlServerPageSizeBy = int64(8192) // SQL Server pages are 8 KB
	)

	rows, err := s.client.QueryRows(ctx)
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return fmt.Errorf("sqlServerScraperHelper: %w", err)
		}
		s.logger.Warn("problems encountered getting index physical stats rows", zap.Error(err))
	}

	var errs []error
	now := pcommon.NewTimestampFromTime(time.Now())
	for i, row := range rows {
		rb := s.setupResourceBuilder(s.mb.NewResourceBuilder(), row)
		rb.SetSqlserverDatabaseName(row[databaseNameKey])

		indexID, err := retrieveInt(row, indexIDKey)
		if err != nil {
			errs = append(errs, fmt.Errorf("row %d: failed to parse %s: %w", i, indexIDKey, err))
			continue
		}

		val, err := retrieveFloat(row, fragKey)
		if err != nil {
			errs = append(errs, fmt.Errorf("row %d: failed to parse %s: %w", i, fragKey, err))
		} else {
			s.mb.RecordSqlserverIndexFragmentationDataPoint(now, val.(float64), row[databaseNameKey], indexID.(int64), row[objectNameKey], row[schemaNameKey])
		}

		pageCount, err := retrieveInt(row, pageCountKey)
		if err != nil {
			errs = append(errs, fmt.Errorf("row %d: failed to parse %s: %w", i, pageCountKey, err))
		} else {
			errs = append(errs,
				s.mb.RecordSqlserverIndexPageCountDataPoint(now, row[pageCountKey], row[databaseNameKey], indexID.(int64), row[objectNameKey], row[schemaNameKey]),
				s.mb.RecordSqlserverIndexSizeDataPoint(now, strconv.FormatInt(pageCount.(int64)*sqlServerPageSizeBy, 10), row[databaseNameKey], indexID.(int64), row[objectNameKey], row[schemaNameKey]),
			)
		}

		val, err = retrieveFloat(row, pageSpaceUsedKey)
		if err != nil {
			errs = append(errs, fmt.Errorf("row %d: failed to parse %s: %w", i, pageSpaceUsedKey, err))
		} else {
			s.mb.RecordSqlserverIndexPageUtilizationDataPoint(now, val.(float64), row[databaseNameKey], indexID.(int64), row[objectNameKey], row[schemaNameKey])
		}

		errs = append(errs,
			s.mb.RecordSqlserverIndexRecordCountDataPoint(now, row[recordCountKey], row[databaseNameKey], indexID.(int64), row[objectNameKey], row[schemaNameKey]),
		)

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	if len(rows) == 0 {
		s.logger.Info("SQLServerScraperHelper: No rows found by index physical stats query")
	}

	return errors.Join(errs...)
}

func (s *sqlServerScraperHelper) recordDatabaseQueryTextAndPlan(ctx context.Context) (pcommon.Resource, error) {
	// Constants are the column names of the database status
	const (
		databaseName      = "database_name"
		executionCount    = "execution_count"
		lastExecutionTime = "last_execution_time"
		planCreationTime  = "plan_creation_time"
		logicalReads      = "total_logical_reads"
		logicalWrites     = "total_logical_writes"
		physicalReads     = "total_physical_reads"
		queryHash         = "query_hash"
		queryPlan         = "query_plan"
		queryPlanHash     = "query_plan_hash"
		queryText         = "query_text"
		rowsReturned      = "total_rows"
		// the time returned from mssql is in microsecond
		totalElapsedTime = "total_elapsed_time"
		totalGrant       = "total_grant_kb"
		// the time returned from mssql is in microsecond
		totalWorkerTime = "total_worker_time"

		dbSystemNameVal = "microsoft.sql_server"

		// stored procedure columns
		storedProcedureID             = "procedure_id"
		storedProcedureName           = "procedure_name"
		storedProcedureExecutionCount = "procedure_execution_count"
	)

	resources := pcommon.NewResource()

	rows, err := s.client.QueryRows(
		ctx,
		sql.Named("lookbackTime", -int(s.config.EffectiveLookbackTime().Seconds())),
		sql.Named("maxSampleCount", s.config.MaxQuerySampleCount),
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
		procID := row[storedProcedureID] // defaulted to '0' if not present

		elapsedTimeMicrosecond, err := strconv.ParseInt(row[totalElapsedTime], 10, 64)
		if err != nil {
			s.logger.Warn(fmt.Sprintf("sqlServerScraperHelper failed getting rows: %s", err))
			errs = append(errs, err)
		} else {
			// we're trying to get the queries that used the most time.
			// caching the total elapsed time (in microsecond) and compare in the next scrape.
			if cached, diff := s.cacheAndDiff(queryHashVal, queryPlanHashVal, procID, totalElapsedTime, elapsedTimeMicrosecond); cached && diff > 0 {
				totalElapsedTimeDiffsMicrosecond[i] = diff
			}
		}
	}
	// sort the rows based on the totalElapsedTimeDiffs in descending order,
	// only report first T(T=topQueryCount) rows.
	rows = sortRows(rows, totalElapsedTimeDiffsMicrosecond, s.config.TopQueryCount)

	// sort the totalElapsedTimeDiffs in descending order as well
	sort.Slice(totalElapsedTimeDiffsMicrosecond, func(i, j int) bool { return totalElapsedTimeDiffsMicrosecond[i] > totalElapsedTimeDiffsMicrosecond[j] })

	resourcesAdded := false
	now := time.Now()
	timestamp := pcommon.NewTimestampFromTime(now)
	s.lastExecutionTimestamp = now
	for i, row := range rows {
		// reporting human-readable query hash and query hash plan
		queryHashVal := hex.EncodeToString([]byte(row[queryHash]))
		queryPlanHashVal := hex.EncodeToString([]byte(row[queryPlanHash]))
		procID := row[storedProcedureID]

		queryTextVal := s.retrieveValue(row, queryText, &errs, func(row sqlquery.StringMap, columnName string) (any, error) {
			statement := row[columnName]
			obfuscated, err := s.obfuscator.obfuscateSQLString(statement)
			if err != nil {
				s.logger.Error(fmt.Sprintf("failed to obfuscate SQL statement: %v", statement))
				return "", nil
			}

			return obfuscated, nil
		})

		databaseNameVal := row[databaseName]

		var cached bool

		executionCountVal := s.retrieveValue(row, executionCount, &errs, retrieveInt)
		cached, executionCountVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, procID, executionCount, executionCountVal.(int64))
		if !cached {
			executionCountVal = int64(0)
		}

		logicalReadsVal := s.retrieveValue(row, logicalReads, &errs, retrieveInt)
		cached, logicalReadsVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, procID, logicalReads, logicalReadsVal.(int64))
		if !cached {
			logicalReadsVal = int64(0)
		}

		logicalWritesVal := s.retrieveValue(row, logicalWrites, &errs, retrieveInt)
		cached, logicalWritesVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, procID, logicalWrites, logicalWritesVal.(int64))
		if !cached {
			logicalWritesVal = int64(0)
		}

		physicalReadsVal := s.retrieveValue(row, physicalReads, &errs, retrieveInt)
		cached, physicalReadsVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, procID, physicalReads, physicalReadsVal.(int64))
		if !cached {
			physicalReadsVal = int64(0)
		}

		queryPlanVal := s.retrieveValue(row, queryPlan, &errs, func(row sqlquery.StringMap, columnName string) (any, error) {
			return s.obfuscator.obfuscateXMLPlan(row[columnName])
		})

		rowsReturnedVal := s.retrieveValue(row, rowsReturned, &errs, retrieveInt)
		cached, rowsReturnedVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, procID, rowsReturned, rowsReturnedVal.(int64))
		if !cached {
			rowsReturnedVal = int64(0)
		}

		totalGrantVal := s.retrieveValue(row, totalGrant, &errs, retrieveInt)
		cached, totalGrantVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, procID, totalGrant, totalGrantVal.(int64))
		if !cached {
			totalGrantVal = int64(0)
		}

		totalWorkerTimeVal := s.retrieveValue(row, totalWorkerTime, &errs, retrieveInt)
		cached, totalWorkerTimeVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, procID, totalWorkerTime, totalWorkerTimeVal.(int64))
		totalWorkerTimeInSecVal := float64(0)
		if cached {
			totalWorkerTimeInSecVal = float64(totalWorkerTimeVal.(int64)) / 1_000_000
		}

		procExecCountVal := s.retrieveValue(row, storedProcedureExecutionCount, &errs, retrieveInt)
		cached, procExecCountVal = s.cacheAndDiff(queryHashVal, queryPlanHashVal, procID, storedProcedureExecutionCount, procExecCountVal.(int64))
		if !cached {
			procExecCountVal = int64(0)
		}

		lastExecutionTimeVal := row[lastExecutionTime]
		planCreationTimeVal := row[planCreationTime]

		totalElapsedTimeVal := float64(totalElapsedTimeDiffsMicrosecond[i]) / 1_000_000
		if count, ok := executionCountVal.(int64); !ok || count == 0 || totalElapsedTimeVal == 0 {
			continue
		}

		s.logger.Debug(fmt.Sprintf("QueryHash: %v, PlanHash: %v, DataRow: %v", queryHashVal, queryPlanHashVal, row))

		if !resourcesAdded {
			resources = s.setupResourceBuilder(s.lb.NewResourceBuilder(), row).Emit()
			resourcesAdded = true
		}
		s.lb.RecordDbServerTopQueryEvent(
			context.Background(),
			timestamp,
			totalWorkerTimeInSecVal,
			queryTextVal.(string),
			databaseNameVal,
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
			dbSystemNameVal,
			procExecCountVal.(int64),
			row[storedProcedureID],
			row[storedProcedureName],
			lastExecutionTimeVal,
			planCreationTimeVal,
		)
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
func (s *sqlServerScraperHelper) cacheAndDiff(queryHash, queryPlanHash, procedureID, column string, val int64) (bool, int64) {
	if val < 0 {
		return false, 0
	}

	key := queryHash + "-" + queryPlanHash + "-" + column
	if procedureID != "0" { // procedureID is '0' when not a stored procedure
		key = procedureID + "-" + key
	}
	cached, ok := s.cache.Get(key)
	s.cache.Add(key, val)
	if !ok {
		return false, val
	}

	if val > cached {
		return true, val - cached
	}
	return true, 0
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
	pq := make(priorityqueue.PriorityQueue[sqlquery.StringMap, int64], len(rows))
	for i, row := range rows {
		value := values[i]
		pq[i] = &priorityqueue.QueueItem[sqlquery.StringMap, int64]{
			Value:    row,
			Priority: value,
			Index:    i,
		}
	}
	heap.Init(&pq)

	for pq.Len() > 0 && len(results) < int(maximum) {
		item := heap.Pop(&pq).(*priorityqueue.QueueItem[sqlquery.StringMap, int64])
		results = append(results, item.Value)
	}
	return results
}

func (s *sqlServerScraperHelper) recordWorkerThreadMetrics(ctx context.Context) error {
	const activeThreads = "active_threads"
	const availableThreads = "available_threads"
	const requestsWaitingForThreads = "requests_waiting_for_threads"
	const totalThreads = "total_threads"
	const waitingForCPUThreads = "waiting_for_cpu_threads"

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
		rb := s.setupResourceBuilder(s.mb.NewResourceBuilder(), row)

		val, err := retrieveInt(row, requestsWaitingForThreads)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse %s for row %d: %w", requestsWaitingForThreads, i, err))
		} else {
			s.mb.RecordSqlserverWorkerRequestCountDataPoint(now, val.(int64), metadata.AttributeRequestStateWaiting)
		}

		val, err = retrieveInt(row, activeThreads)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse %s for row %d: %w", activeThreads, i, err))
		} else {
			s.mb.RecordSqlserverWorkerThreadCountDataPoint(now, val.(int64), metadata.AttributeWorkerStateActive)
		}

		val, err = retrieveInt(row, availableThreads)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse %s for row %d: %w", availableThreads, i, err))
		} else {
			s.mb.RecordSqlserverWorkerThreadCountDataPoint(now, val.(int64), metadata.AttributeWorkerStateAvailable)
		}

		val, err = retrieveInt(row, totalThreads)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse %s for row %d: %w", totalThreads, i, err))
		} else {
			s.mb.RecordSqlserverWorkerThreadCountDataPoint(now, val.(int64), metadata.AttributeWorkerStateMaximum)
		}

		val, err = retrieveInt(row, waitingForCPUThreads)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to parse %s for row %d: %w", waitingForCPUThreads, i, err))
		} else {
			s.mb.RecordSqlserverWorkerThreadCountDataPoint(now, val.(int64), metadata.AttributeWorkerStateWaitingForCPU)
		}

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	return errors.Join(errs...)
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

// SQLServer:SQL Errors counter instance names.
const (
	dbOfflineErrors      = "DB Offline Errors"
	infoErrors           = "Info Errors"
	killConnectionErrors = "Kill Connection Errors"
	userErrors           = "User Errors"
)

// errorCategoryAttr maps a SQLServer:SQL Errors counter instance to the
// sqlserver.error.category attribute value. The returned ok is false for
// instances that have no mapping (for example the "_Total" instance, which is
// intentionally skipped since callers can sum the per-category points).
func errorCategoryAttr(instance string) (category metadata.AttributeSqlserverErrorCategory, ok bool) {
	switch instance {
	case dbOfflineErrors:
		return metadata.AttributeSqlserverErrorCategoryDbOffline, true
	case infoErrors:
		return metadata.AttributeSqlserverErrorCategoryInfo, true
	case killConnectionErrors:
		return metadata.AttributeSqlserverErrorCategoryKillConnection, true
	case userErrors:
		return metadata.AttributeSqlserverErrorCategoryUser, true
	default:
		return 0, false
	}
}

func (s *sqlServerScraperHelper) recordDatabaseSampleQuery(ctx context.Context) (pcommon.Resource, error) {
	const blockingSessionID = "blocking_session_id"
	const blockingStartTime = "blocking_start_time"
	const clientAddress = "client_address"
	const clientAppName = "client_app_name"
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
	const sessionDurationMillisecond = "session_duration"
	const sessionID = "session_id"
	const sessionStartTime = "session_start_time"
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
	// stored procedure columns
	const storedProcedureID = "procedure_id"
	const storedProcedureName = "procedure_name"

	rows, err := s.client.QueryRows(
		ctx,
		sql.Named("top", s.config.MaxRowsPerQuery),
	)
	resources := pcommon.NewResource()
	if err != nil {
		if !errors.Is(err, sqlquery.ErrNullValueWarning) {
			return resources, fmt.Errorf("sqlServerScraperHelper failed getting log rows: %w", err)
		}
		// in case the sql returned rows contains null value, we just log a warning and continue
		s.logger.Warn("problems encountered getting log rows", zap.Error(err))
	}

	activeSessionIDs := make(map[int64]struct{})
	blockingSessionIDs := make(map[int64]struct{})
	for _, row := range rows {
		sessionVal, parseErr := retrieveInt(row, sessionID)
		if parseErr == nil {
			activeSessionIDs[sessionVal.(int64)] = struct{}{}
		}

		blockingVal, parseErr := retrieveInt(row, blockingSessionID)
		if parseErr == nil && blockingVal.(int64) > 0 {
			blockingSessionIDs[blockingVal.(int64)] = struct{}{}
		}
	}

	missingBlockingSessionIDs := make(map[int64]struct{})
	for blockerSessionID := range blockingSessionIDs {
		if _, ok := activeSessionIDs[blockerSessionID]; !ok {
			missingBlockingSessionIDs[blockerSessionID] = struct{}{}
		}
	}

	if len(missingBlockingSessionIDs) > 0 && s.db != nil {
		idleBlockingQuery := fmt.Sprintf(
			getSQLServerIdleBlockingSessionsQuery(),
			formatSQLServerSessionIDsParam(missingBlockingSessionIDs),
		)
		idleBlockingClient := s.clientProviderFunc(
			sqlquery.DbWrapper{Db: s.db},
			idleBlockingQuery,
			s.logger,
			s.telemetry,
		)

		idleRows, idleErr := idleBlockingClient.QueryRows(
			ctx,
			sql.Named("top", s.config.MaxRowsPerQuery),
		)
		if idleErr != nil {
			s.logger.Warn("problems encountered getting idle blocker log rows", zap.Error(idleErr))
		}
		if idleErr == nil || errors.Is(idleErr, sqlquery.ErrNullValueWarning) {
			for _, idleRow := range idleRows {
				idleSessionVal, parseErr := retrieveInt(idleRow, sessionID)
				if parseErr == nil {
					if _, ok := missingBlockingSessionIDs[idleSessionVal.(int64)]; ok {
						rows = append(rows, idleRow)
					}
				} else {
					s.logger.Debug("failed to parse idle blocker session id", zap.String("column", sessionID), zap.String("value", idleRow[sessionID]), zap.Error(parseErr))
				}
			}
		}
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
			obfuscated, err := s.obfuscator.obfuscateSQLString(statement)
			if err != nil {
				s.logger.Error(fmt.Sprintf("failed to obfuscate SQL statement: %v", statement))
				return "", nil
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
		sessionDurationSecondVal := s.retrieveValue(row, sessionDurationMillisecond, &errs, retrieveIntAndConvert(func(i int64) any {
			return float64(i) / 1000.0
		})).(float64)
		totalElapsedTimeSecondVal := s.retrieveValue(row, totalElapsedTimeMillisecond, &errs, retrieveIntAndConvert(func(i int64) any {
			return float64(i) / 1000.0
		})).(float64)
		transactionIDVal := s.retrieveValue(row, transactionID, &errs, retrieveInt).(int64)
		transactionIsolationLevelVal := s.retrieveValue(row, transactionIsolationLevel, &errs, retrieveInt).(int64)
		usernameVal := row[username]
		waitResourceVal := row[waitResource]
		waitTimeMillisecondVal := s.retrieveValue(row, waitTimeMillisecond, &errs, retrieveInt).(int64)
		waitTimeSecondVal := float64(waitTimeMillisecondVal) / 1000.0
		resourceTypeVal, resourceIDVal := parseWaitResource(waitResourceVal)
		blockingStartTimeVal := row[blockingStartTime]
		clientAppNameVal := row[clientAppName]
		sessionStartTimeVal := row[sessionStartTime]
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
		if s.logger.Level() == zap.DebugLevel && row[storedProcedureID] != "0" {
			s.logger.Debug("Stored proc data", zap.String("id", row[storedProcedureID]), zap.String("name", row[storedProcedureName]))
		}
		s.lb.RecordDbServerQuerySampleEvent(
			contextFromQuery,
			timestamp, clientAddressVal, clientPortVal,
			dbNamespaceVal, queryTextVal, dbSystemNameVal,
			networkPeerAddressVal, networkPeerPortVal,
			blockSessionIDVal, blockingStartTimeVal, clientAppNameVal, contextInfoVal,
			commandVal, cpuTimeSecondVal,
			deadlockPriorityVal, estimatedCompletionTimeSecondVal,
			lockTimeoutSecondVal, logicalReadsVal,
			openTransactionCountVal, percentCompleteVal, queryHashVal, queryPlanHashVal,
			queryStartVal, readsVal,
			requestStatusVal, resourceIDVal, resourceTypeVal, rowCountVal,
			sessionDurationSecondVal, sessionStartTimeVal, sessionIDVal, sessionStatusVal,
			totalElapsedTimeSecondVal, transactionIDVal, transactionIsolationLevelVal,
			waitResourceVal, waitTimeSecondVal, waitTypeVal, writesVal, usernameVal,
			row[storedProcedureID], row[storedProcedureName],
		)

		if !resourcesAdded {
			resources = s.setupResourceBuilder(s.lb.NewResourceBuilder(), row).Emit()
			resourcesAdded = true
		}
	}
	return resources, errors.Join(errs...)
}
