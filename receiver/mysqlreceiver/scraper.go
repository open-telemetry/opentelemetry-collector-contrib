// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"container/heap"
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/priorityqueue"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
)

type mySQLScraper struct {
	sqlclient              client
	logger                 *zap.Logger
	config                 *Config
	mb                     *metadata.MetricsBuilder
	lb                     *metadata.LogsBuilder
	cache                  *lru.Cache[string, int64]
	queryPlanCache         *expirable.LRU[string, string]
	obfuscator             *obfuscator
	lastExecutionTimestamp time.Time

	// detectedVersion is the database product and version detected at Connect time.
	// It is set once during start() and used to stamp scope attributes on emitted logs.
	detectedVersion dbVersion

	// Feature gates regarding resource attributes
	renameCommands bool
}

func newMySQLScraper(
	settings receiver.Settings,
	config *Config,
	cache *lru.Cache[string, int64],
	queryPlanCache *expirable.LRU[string, string],
) *mySQLScraper {
	return &mySQLScraper{
		logger:                 settings.Logger,
		config:                 config,
		mb:                     metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		lb:                     metadata.NewLogsBuilder(config.LogsBuilderConfig, settings),
		cache:                  cache,
		queryPlanCache:         queryPlanCache,
		obfuscator:             newObfuscator(),
		lastExecutionTimestamp: time.Unix(0, 0),
	}
}

// start starts the scraper by initializing the db client connection.
func (m *mySQLScraper) start(_ context.Context, _ component.Host) error {
	sqlclient, err := newMySQLClient(m.config)
	if err != nil {
		return err
	}

	err = sqlclient.Connect()
	if err != nil {
		_ = sqlclient.Close()
		return err
	}
	m.sqlclient = sqlclient
	m.detectedVersion = m.sqlclient.getDBVersion()
	m.logDetectedVersion(m.detectedVersion)
	return nil
}

// logDetectedVersion logs the detected database product and version, including
// capability flags and an end-of-life warning for MySQL <8.
func (m *mySQLScraper) logDetectedVersion(dbVer dbVersion) {
	if dbVer.version == nil {
		m.logger.Warn("database version could not be detected at startup; receiver will use MySQL <8/MariaDB fallback behavior for its entire lifetime",
			zap.Bool("supports_query_sample_text", false),
		)
		return
	}
	m.logger.Info("detected database version",
		zap.String("product", dbVer.productString()),
		zap.String("version", dbVer.version.String()),
		zap.Bool("supports_query_sample_text", dbVer.supportsQuerySampleText()),
	)
	if dbVer.product == dbProductMySQL && dbVer.version.Segments()[0] < 8 {
		m.logger.Warn("detected MySQL version is past end-of-life and may not be supported by this receiver in a future release",
			zap.String("version", dbVer.version.String()),
		)
	}
}

// setScopeAttributes stamps db.version and db.product onto the instrumentation
// scope of every ScopeLogs in logs. These are set at the scope level (not the
// resource or record level) so that they are available to downstream processors
// and exporters without affecting the data model — users who don't need them
// can ignore them, and those who do can access them via
// instrumentation_scope.attributes["db.version"] in OTTL.
func (m *mySQLScraper) setScopeAttributes(logs plog.Logs) {
	if m.detectedVersion.version == nil {
		return
	}
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		sls := logs.ResourceLogs().At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			attrs := sls.At(j).Scope().Attributes()
			attrs.PutStr("db.version", m.detectedVersion.version.String())
			attrs.PutStr("db.product", m.detectedVersion.productString())
		}
	}
}

// shutdown closes the db connection
func (m *mySQLScraper) shutdown(context.Context) error {
	if m.sqlclient == nil {
		return nil
	}
	return m.sqlclient.Close()
}

// scrape scrapes the mysql db metric stats, transforms them and labels them into a metric slices.
func (m *mySQLScraper) scrape(context.Context) (pmetric.Metrics, error) {
	if m.sqlclient == nil {
		return pmetric.Metrics{}, errors.New("failed to connect to http client")
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	// collect innodb metrics.
	innodbStats, innoErr := m.sqlclient.getInnodbStats()
	if innoErr != nil {
		m.logger.Error("Failed to fetch InnoDB stats", zap.Error(innoErr))
	}

	errs := &scrapererror.ScrapeErrors{}
	for k, v := range innodbStats {
		if k != "buffer_pool_size" {
			continue
		}
		addPartialIfError(errs, m.mb.RecordMysqlBufferPoolLimitDataPoint(now, v))
	}

	// collect io_waits metrics.
	m.scrapeTableIoWaitsStats(now, errs)
	m.scrapeIndexIoWaitsStats(now, errs)

	// collect table size metrics.

	m.scrapeTableStats(now, errs)

	// collect performance event statements metrics.
	m.scrapeStatementEventsStats(now, errs)
	// collect lock table events metrics
	m.scrapeTableLockWaitEventStats(now, errs)

	// collect global status metrics.
	m.scrapeGlobalStats(now, errs)

	// collect replicas status metrics.
	m.scrapeReplicaStatusStats(now)

	rb := m.mb.NewResourceBuilder()
	rb.SetMysqlInstanceEndpoint(m.config.Endpoint)
	m.mb.EmitForResource(metadata.WithResource(rb.Emit()))

	return m.mb.Emit(), errs.Combine()
}

// emitLogsWithScopeAttrs emits accumulated log records, stamps the resource
// endpoint, applies scope-level version attributes, and returns the result.
func (m *mySQLScraper) emitLogsWithScopeAttrs(errs *scrapererror.ScrapeErrors) (plog.Logs, error) {
	rb := m.lb.NewResourceBuilder()
	rb.SetMysqlInstanceEndpoint(m.config.Endpoint)
	logs := m.lb.Emit(metadata.WithLogsResource(rb.Emit()))
	m.setScopeAttributes(logs)
	return logs, errs.Combine()
}

func (m *mySQLScraper) scrapeTopQueryFunc(_ context.Context) (plog.Logs, error) {
	if m.sqlclient == nil {
		return plog.NewLogs(), errors.New("failed to connect to MySQL client")
	}

	errs := &scrapererror.ScrapeErrors{}

	now := pcommon.NewTimestampFromTime(time.Now())

	if int(math.Ceil(time.Since(m.lastExecutionTimestamp).Seconds())) < int(m.config.TopQueryCollection.CollectionInterval.Seconds()) {
		m.logger.Debug("Skipping top queries scrape, not enough time has passed since last execution")
	} else {
		m.scrapeTopQueries(now, errs)
	}
	return m.emitLogsWithScopeAttrs(errs)
}

func (m *mySQLScraper) scrapeQuerySampleFunc(ctx context.Context) (plog.Logs, error) {
	if m.sqlclient == nil {
		return plog.NewLogs(), errors.New("failed to connect to MySQL client")
	}

	errs := &scrapererror.ScrapeErrors{}

	now := pcommon.NewTimestampFromTime(time.Now())

	m.scrapeQuerySamples(ctx, now, errs)
	return m.emitLogsWithScopeAttrs(errs)
}

func (m *mySQLScraper) scrapeGlobalStats(now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	globalStats, err := m.sqlclient.getGlobalStats()
	if err != nil {
		m.logger.Error("Failed to fetch global stats", zap.Error(err))
		errs.AddPartial(66, err)
		return
	}

	m.recordDataPages(now, globalStats, errs)
	m.recordDataUsage(now, globalStats, errs)

	for k, v := range globalStats {
		switch k {
		// bytes transmission
		case "Bytes_received":
			addPartialIfError(errs, m.mb.RecordMysqlClientNetworkIoDataPoint(now, v, metadata.AttributeDirectionReceived))
		case "Bytes_sent":
			addPartialIfError(errs, m.mb.RecordMysqlClientNetworkIoDataPoint(now, v, metadata.AttributeDirectionSent))

		// buffer_pool.pages
		case "Innodb_buffer_pool_pages_data":
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolPagesDataPoint(now, v,
				metadata.AttributeBufferPoolPagesData))
		case "Innodb_buffer_pool_pages_free":
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolPagesDataPoint(now, v,
				metadata.AttributeBufferPoolPagesFree))
		case "Innodb_buffer_pool_pages_total":
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolPagesDataPoint(now, v,
				metadata.AttributeBufferPoolPagesTotal))
		case "Innodb_buffer_pool_pages_misc":
			_, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				m.logger.Warn("Innodb_buffer_pool_pages_misc reports an out-of-bounds value and will be ignored. See https://bugs.mysql.com/bug.php?id=59550.")
				continue
			}
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolPagesDataPoint(now, v,
				metadata.AttributeBufferPoolPagesMisc))
		// buffer_pool.page_flushes
		case "Innodb_buffer_pool_pages_flushed":
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolPageFlushesDataPoint(now, v))

		// buffer_pool.operations
		case "Innodb_buffer_pool_read_ahead_rnd":
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsReadAheadRnd))
		case "Innodb_buffer_pool_read_ahead":
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsReadAhead))
		case "Innodb_buffer_pool_read_ahead_evicted":
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsReadAheadEvicted))
		case "Innodb_buffer_pool_read_requests":
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsReadRequests))
		case "Innodb_buffer_pool_reads":
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsReads))
		case "Innodb_buffer_pool_wait_free":
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsWaitFree))
		case "Innodb_buffer_pool_write_requests":
			addPartialIfError(errs, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsWriteRequests))

		// connection.errors
		case "Connection_errors_accept":
			addPartialIfError(errs, m.mb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorAccept))
		case "Connection_errors_internal":
			addPartialIfError(errs, m.mb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorInternal))
		case "Connection_errors_max_connections":
			addPartialIfError(errs, m.mb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorMaxConnections))
		case "Connection_errors_peer_address":
			addPartialIfError(errs, m.mb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorPeerAddress))
		case "Connection_errors_select":
			addPartialIfError(errs, m.mb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorSelect))
		case "Connection_errors_tcpwrap":
			addPartialIfError(errs, m.mb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorTcpwrap))
		case "Aborted_clients":
			addPartialIfError(errs, m.mb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorAbortedClients))
		case "Aborted_connects":
			addPartialIfError(errs, m.mb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorAborted))
		case "Locked_connects":
			addPartialIfError(errs, m.mb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorLocked))

		// connection
		case "Connections":
			addPartialIfError(errs, m.mb.RecordMysqlConnectionCountDataPoint(now, v))
		case "Max_used_connections":
			addPartialIfError(errs, m.mb.RecordMysqlMaxUsedConnectionsDataPoint(now, v))

		// prepared_statements_commands
		case "Com_stmt_execute":
			addPartialIfError(errs, m.mb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandExecute))
		case "Com_stmt_close":
			addPartialIfError(errs, m.mb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandClose))
		case "Com_stmt_fetch":
			addPartialIfError(errs, m.mb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandFetch))
		case "Com_stmt_prepare":
			addPartialIfError(errs, m.mb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandPrepare))
		case "Com_stmt_reset":
			addPartialIfError(errs, m.mb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandReset))
		case "Com_stmt_send_long_data":
			addPartialIfError(errs, m.mb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandSendLongData))

		// commands
		case "Com_delete":
			addPartialIfError(errs, m.mb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandDelete))
		case "Com_delete_multi":
			addPartialIfError(errs, m.mb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandDeleteMulti))
		case "Com_insert":
			addPartialIfError(errs, m.mb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandInsert))
		case "Com_select":
			addPartialIfError(errs, m.mb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandSelect))
		case "Com_update":
			addPartialIfError(errs, m.mb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandUpdate))
		case "Com_update_multi":
			addPartialIfError(errs, m.mb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandUpdateMulti))

		// created tmps
		case "Created_tmp_disk_tables":
			addPartialIfError(errs, m.mb.RecordMysqlTmpResourcesDataPoint(now, v, metadata.AttributeTmpResourceDiskTables))
		case "Created_tmp_files":
			addPartialIfError(errs, m.mb.RecordMysqlTmpResourcesDataPoint(now, v, metadata.AttributeTmpResourceFiles))
		case "Created_tmp_tables":
			addPartialIfError(errs, m.mb.RecordMysqlTmpResourcesDataPoint(now, v, metadata.AttributeTmpResourceTables))

		// handlers
		case "Handler_commit":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerCommit))
		case "Handler_delete":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerDelete))
		case "Handler_discover":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerDiscover))
		case "Handler_external_lock":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerExternalLock))
		case "Handler_mrr_init":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerMrrInit))
		case "Handler_prepare":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerPrepare))
		case "Handler_read_first":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadFirst))
		case "Handler_read_key":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadKey))
		case "Handler_read_last":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadLast))
		case "Handler_read_next":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadNext))
		case "Handler_read_prev":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadPrev))
		case "Handler_read_rnd":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadRnd))
		case "Handler_read_rnd_next":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadRndNext))
		case "Handler_rollback":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerRollback))
		case "Handler_savepoint":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerSavepoint))
		case "Handler_savepoint_rollback":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerSavepointRollback))
		case "Handler_update":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerUpdate))
		case "Handler_write":
			addPartialIfError(errs, m.mb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerWrite))

		// double_writes
		case "Innodb_dblwr_pages_written":
			addPartialIfError(errs, m.mb.RecordMysqlDoubleWritesDataPoint(now, v, metadata.AttributeDoubleWritesPagesWritten))
		case "Innodb_dblwr_writes":
			addPartialIfError(errs, m.mb.RecordMysqlDoubleWritesDataPoint(now, v, metadata.AttributeDoubleWritesWrites))

		// log_operations
		case "Innodb_log_waits":
			addPartialIfError(errs, m.mb.RecordMysqlLogOperationsDataPoint(now, v, metadata.AttributeLogOperationsWaits))
		case "Innodb_log_write_requests":
			addPartialIfError(errs, m.mb.RecordMysqlLogOperationsDataPoint(now, v, metadata.AttributeLogOperationsWriteRequests))
		case "Innodb_log_writes":
			addPartialIfError(errs, m.mb.RecordMysqlLogOperationsDataPoint(now, v, metadata.AttributeLogOperationsWrites))
		case "Innodb_os_log_fsyncs":
			addPartialIfError(errs, m.mb.RecordMysqlLogOperationsDataPoint(now, v, metadata.AttributeLogOperationsFsyncs))

		// operations
		case "Innodb_data_fsyncs":
			addPartialIfError(errs, m.mb.RecordMysqlOperationsDataPoint(now, v, metadata.AttributeOperationsFsyncs))
		case "Innodb_data_reads":
			addPartialIfError(errs, m.mb.RecordMysqlOperationsDataPoint(now, v, metadata.AttributeOperationsReads))
		case "Innodb_data_writes":
			addPartialIfError(errs, m.mb.RecordMysqlOperationsDataPoint(now, v, metadata.AttributeOperationsWrites))

		// page_operations
		case "Innodb_pages_created":
			addPartialIfError(errs, m.mb.RecordMysqlPageOperationsDataPoint(now, v, metadata.AttributePageOperationsCreated))
		case "Innodb_pages_read":
			addPartialIfError(errs, m.mb.RecordMysqlPageOperationsDataPoint(now, v,
				metadata.AttributePageOperationsRead))
		case "Innodb_pages_written":
			addPartialIfError(errs, m.mb.RecordMysqlPageOperationsDataPoint(now, v,
				metadata.AttributePageOperationsWritten))

		// row_locks
		case "Innodb_row_lock_waits":
			addPartialIfError(errs, m.mb.RecordMysqlRowLocksDataPoint(now, v, metadata.AttributeRowLocksWaits))
		case "Innodb_row_lock_time":
			addPartialIfError(errs, m.mb.RecordMysqlRowLocksDataPoint(now, v, metadata.AttributeRowLocksTime))

		// row_operations
		case "Innodb_rows_deleted":
			addPartialIfError(errs, m.mb.RecordMysqlRowOperationsDataPoint(now, v, metadata.AttributeRowOperationsDeleted))
		case "Innodb_rows_inserted":
			addPartialIfError(errs, m.mb.RecordMysqlRowOperationsDataPoint(now, v, metadata.AttributeRowOperationsInserted))
		case "Innodb_rows_read":
			addPartialIfError(errs, m.mb.RecordMysqlRowOperationsDataPoint(now, v,
				metadata.AttributeRowOperationsRead))
		case "Innodb_rows_updated":
			addPartialIfError(errs, m.mb.RecordMysqlRowOperationsDataPoint(now, v,
				metadata.AttributeRowOperationsUpdated))

		// locks
		case "Table_locks_immediate":
			addPartialIfError(errs, m.mb.RecordMysqlLocksDataPoint(now, v, metadata.AttributeLocksImmediate))
		case "Table_locks_waited":
			addPartialIfError(errs, m.mb.RecordMysqlLocksDataPoint(now, v, metadata.AttributeLocksWaited))

		// joins
		case "Select_full_join":
			addPartialIfError(errs, m.mb.RecordMysqlJoinsDataPoint(now, v, metadata.AttributeJoinKindFull))
		case "Select_full_range_join":
			addPartialIfError(errs, m.mb.RecordMysqlJoinsDataPoint(now, v, metadata.AttributeJoinKindFullRange))
		case "Select_range":
			addPartialIfError(errs, m.mb.RecordMysqlJoinsDataPoint(now, v, metadata.AttributeJoinKindRange))
		case "Select_range_check":
			addPartialIfError(errs, m.mb.RecordMysqlJoinsDataPoint(now, v, metadata.AttributeJoinKindRangeCheck))
		case "Select_scan":
			addPartialIfError(errs, m.mb.RecordMysqlJoinsDataPoint(now, v, metadata.AttributeJoinKindScan))

		// open cache
		case "Table_open_cache_hits":
			addPartialIfError(errs, m.mb.RecordMysqlTableOpenCacheDataPoint(now, v, metadata.AttributeCacheStatusHit))
		case "Table_open_cache_misses":
			addPartialIfError(errs, m.mb.RecordMysqlTableOpenCacheDataPoint(now, v, metadata.AttributeCacheStatusMiss))
		case "Table_open_cache_overflows":
			addPartialIfError(errs, m.mb.RecordMysqlTableOpenCacheDataPoint(now, v, metadata.AttributeCacheStatusOverflow))

		// queries
		case "Queries":
			addPartialIfError(errs, m.mb.RecordMysqlQueryCountDataPoint(now, v))
		case "Questions":
			addPartialIfError(errs, m.mb.RecordMysqlQueryClientCountDataPoint(now, v))
		case "Slow_queries":
			addPartialIfError(errs, m.mb.RecordMysqlQuerySlowCountDataPoint(now, v))

		// sorts
		case "Sort_merge_passes":
			addPartialIfError(errs, m.mb.RecordMysqlSortsDataPoint(now, v, metadata.AttributeSortsMergePasses))
		case "Sort_range":
			addPartialIfError(errs, m.mb.RecordMysqlSortsDataPoint(now, v, metadata.AttributeSortsRange))
		case "Sort_rows":
			addPartialIfError(errs, m.mb.RecordMysqlSortsDataPoint(now, v, metadata.AttributeSortsRows))
		case "Sort_scan":
			addPartialIfError(errs, m.mb.RecordMysqlSortsDataPoint(now, v, metadata.AttributeSortsScan))

		// threads
		case "Threads_cached":
			addPartialIfError(errs, m.mb.RecordMysqlThreadsDataPoint(now, v, metadata.AttributeThreadsCached))
		case "Threads_connected":
			addPartialIfError(errs, m.mb.RecordMysqlThreadsDataPoint(now, v, metadata.AttributeThreadsConnected))
		case "Threads_created":
			addPartialIfError(errs, m.mb.RecordMysqlThreadsDataPoint(now, v, metadata.AttributeThreadsCreated))
		case "Threads_running":
			addPartialIfError(errs, m.mb.RecordMysqlThreadsDataPoint(now, v, metadata.AttributeThreadsRunning))

		// opened resources
		case "Opened_files":
			addPartialIfError(errs, m.mb.RecordMysqlOpenedResourcesDataPoint(now, v, metadata.AttributeOpenedResourcesFile))
		case "Opened_tables":
			addPartialIfError(errs, m.mb.RecordMysqlOpenedResourcesDataPoint(now, v, metadata.AttributeOpenedResourcesTable))
		case "Opened_table_definitions":
			addPartialIfError(errs, m.mb.RecordMysqlOpenedResourcesDataPoint(now, v, metadata.AttributeOpenedResourcesTableDefinition))

		// mysqlx_worker_threads
		case "Mysqlx_worker_threads":
			addPartialIfError(errs, m.mb.RecordMysqlMysqlxWorkerThreadsDataPoint(now, v, metadata.AttributeMysqlxThreadsAvailable))
		case "Mysqlx_worker_threads_active":
			addPartialIfError(errs, m.mb.RecordMysqlMysqlxWorkerThreadsDataPoint(now, v, metadata.AttributeMysqlxThreadsActive))

		// mysqlx_connections
		case "Mysqlx_connections_accepted":
			addPartialIfError(errs, m.mb.RecordMysqlMysqlxConnectionsDataPoint(now, v, metadata.AttributeConnectionStatusAccepted))
		case "Mysqlx_connections_closed":
			addPartialIfError(errs, m.mb.RecordMysqlMysqlxConnectionsDataPoint(now, v, metadata.AttributeConnectionStatusClosed))
		case "Mysqlx_connections_rejected":
			addPartialIfError(errs, m.mb.RecordMysqlMysqlxConnectionsDataPoint(now, v, metadata.AttributeConnectionStatusRejected))

		// uptime
		case "Uptime":
			addPartialIfError(errs, m.mb.RecordMysqlUptimeDataPoint(now, v))

		// page size
		case "Innodb_page_size":
			addPartialIfError(errs, m.mb.RecordMysqlPageSizeDataPoint(now, v))
		}
	}
}

func (m *mySQLScraper) scrapeTableStats(now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	tableStats, err := m.sqlclient.getTableStats()
	if err != nil {
		m.logger.Error("Failed to fetch table size stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := range tableStats {
		s := tableStats[i]
		// counts
		m.mb.RecordMysqlTableRowsDataPoint(now, s.rows, s.name, s.schema)
		m.mb.RecordMysqlTableAverageRowLengthDataPoint(now, s.averageRowLength, s.name, s.schema)
		m.mb.RecordMysqlTableSizeDataPoint(now, s.dataLength, s.name, s.schema, metadata.AttributeTableSizeTypeData)
		m.mb.RecordMysqlTableSizeDataPoint(now, s.indexLength, s.name, s.schema, metadata.AttributeTableSizeTypeIndex)
	}
}

func (m *mySQLScraper) scrapeTableIoWaitsStats(now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	tableIoWaitsStats, err := m.sqlclient.getTableIoWaitsStats()
	if err != nil {
		m.logger.Error("Failed to fetch table io_waits stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := range tableIoWaitsStats {
		s := tableIoWaitsStats[i]
		// counts
		m.mb.RecordMysqlTableIoWaitCountDataPoint(now, s.countDelete, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema)
		m.mb.RecordMysqlTableIoWaitCountDataPoint(now, s.countFetch, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema)
		m.mb.RecordMysqlTableIoWaitCountDataPoint(now, s.countInsert, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema)
		m.mb.RecordMysqlTableIoWaitCountDataPoint(now, s.countUpdate, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema)

		// times
		m.mb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeDelete, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema,
		)
		m.mb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeFetch, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema,
		)
		m.mb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeInsert, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema,
		)
		m.mb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeUpdate, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema,
		)
	}
}

func (m *mySQLScraper) scrapeIndexIoWaitsStats(now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	indexIoWaitsStats, err := m.sqlclient.getIndexIoWaitsStats()
	if err != nil {
		m.logger.Error("Failed to fetch index io_waits stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := range indexIoWaitsStats {
		s := indexIoWaitsStats[i]
		// counts
		m.mb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countDelete, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema, s.index)
		m.mb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countFetch, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema, s.index)
		m.mb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countInsert, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema, s.index)
		m.mb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countUpdate, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema, s.index)

		// times
		m.mb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeDelete, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema, s.index,
		)
		m.mb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeFetch, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema, s.index,
		)
		m.mb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeInsert, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema, s.index,
		)
		m.mb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeUpdate, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema, s.index,
		)
	}
}

func (m *mySQLScraper) scrapeStatementEventsStats(now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	statementEventsStats, err := m.sqlclient.getStatementEventsStats()
	if err != nil {
		m.logger.Error("Failed to fetch statement events stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := range statementEventsStats {
		s := statementEventsStats[i]
		m.mb.RecordMysqlStatementEventCountDataPoint(now, s.countCreatedTmpDiskTables, s.schema, s.digest, s.digestText, metadata.AttributeEventStateCreatedTmpDiskTables)
		m.mb.RecordMysqlStatementEventCountDataPoint(now, s.countCreatedTmpTables, s.schema, s.digest, s.digestText, metadata.AttributeEventStateCreatedTmpTables)
		m.mb.RecordMysqlStatementEventCountDataPoint(now, s.countErrors, s.schema, s.digest, s.digestText, metadata.AttributeEventStateErrors)
		m.mb.RecordMysqlStatementEventCountDataPoint(now, s.countNoIndexUsed, s.schema, s.digest, s.digestText, metadata.AttributeEventStateNoIndexUsed)
		m.mb.RecordMysqlStatementEventCountDataPoint(now, s.countRowsAffected, s.schema, s.digest, s.digestText, metadata.AttributeEventStateRowsAffected)
		m.mb.RecordMysqlStatementEventCountDataPoint(now, s.countRowsExamined, s.schema, s.digest, s.digestText, metadata.AttributeEventStateRowsExamined)
		m.mb.RecordMysqlStatementEventCountDataPoint(now, s.countRowsSent, s.schema, s.digest, s.digestText, metadata.AttributeEventStateRowsSent)
		m.mb.RecordMysqlStatementEventCountDataPoint(now, s.countSortMergePasses, s.schema, s.digest, s.digestText, metadata.AttributeEventStateSortMergePasses)
		m.mb.RecordMysqlStatementEventCountDataPoint(now, s.countSortRows, s.schema, s.digest, s.digestText, metadata.AttributeEventStateSortRows)
		m.mb.RecordMysqlStatementEventCountDataPoint(now, s.countWarnings, s.schema, s.digest, s.digestText, metadata.AttributeEventStateWarnings)

		m.mb.RecordMysqlStatementEventWaitTimeDataPoint(now, s.sumTimerWait, s.schema, s.digest, s.digestText)
	}
}

func (m *mySQLScraper) scrapeTableLockWaitEventStats(now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	tableLockWaitEventStats, err := m.sqlclient.getTableLockWaitEventStats()
	if err != nil {
		m.logger.Error("Failed to fetch index io_waits stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := range tableLockWaitEventStats {
		s := tableLockWaitEventStats[i]
		// read data points
		m.mb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadNormal, s.schema, s.name, metadata.AttributeReadLockTypeNormal)
		m.mb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadWithSharedLocks, s.schema, s.name, metadata.AttributeReadLockTypeWithSharedLocks)
		m.mb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadHighPriority, s.schema, s.name, metadata.AttributeReadLockTypeHighPriority)
		m.mb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadNoInsert, s.schema, s.name, metadata.AttributeReadLockTypeNoInsert)
		m.mb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadExternal, s.schema, s.name, metadata.AttributeReadLockTypeExternal)

		// read time data points
		m.mb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadNormal, s.schema, s.name, metadata.AttributeReadLockTypeNormal)
		m.mb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadWithSharedLocks, s.schema, s.name, metadata.AttributeReadLockTypeWithSharedLocks)
		m.mb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadHighPriority, s.schema, s.name, metadata.AttributeReadLockTypeHighPriority)
		m.mb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadNoInsert, s.schema, s.name, metadata.AttributeReadLockTypeNoInsert)
		m.mb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadExternal, s.schema, s.name, metadata.AttributeReadLockTypeExternal)

		// write data points
		m.mb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteAllowWrite, s.schema, s.name, metadata.AttributeWriteLockTypeAllowWrite)
		m.mb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteConcurrentInsert, s.schema, s.name, metadata.AttributeWriteLockTypeConcurrentInsert)
		m.mb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteLowPriority, s.schema, s.name, metadata.AttributeWriteLockTypeLowPriority)
		m.mb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteNormal, s.schema, s.name, metadata.AttributeWriteLockTypeNormal)
		m.mb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteExternal, s.schema, s.name, metadata.AttributeWriteLockTypeExternal)

		// write time data points
		m.mb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteAllowWrite, s.schema, s.name, metadata.AttributeWriteLockTypeAllowWrite)
		m.mb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteConcurrentInsert, s.schema, s.name, metadata.AttributeWriteLockTypeConcurrentInsert)
		m.mb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteLowPriority, s.schema, s.name, metadata.AttributeWriteLockTypeLowPriority)
		m.mb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteNormal, s.schema, s.name, metadata.AttributeWriteLockTypeNormal)
		m.mb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteExternal, s.schema, s.name, metadata.AttributeWriteLockTypeExternal)
	}
}

func (m *mySQLScraper) scrapeReplicaStatusStats(now pcommon.Timestamp) {
	replicaStatusStats, err := m.sqlclient.getReplicaStatusStats(m.detectedVersion.supportsReplicaStatus())
	if err != nil {
		m.logger.Info("Failed to fetch replica status stats", zap.Error(err))
		return
	}

	for i := range replicaStatusStats {
		s := replicaStatusStats[i]

		val, _ := s.secondsBehindSource.Value()
		if val != nil {
			m.mb.RecordMysqlReplicaTimeBehindSourceDataPoint(now, val.(int64))
		}

		m.mb.RecordMysqlReplicaSQLDelayDataPoint(now, s.sqlDelay)
	}
}

func (m *mySQLScraper) scrapeTopQueries(now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	queries, err := m.sqlclient.getTopQueries(m.config.TopQueryCollection.MaxQuerySampleCount, m.config.TopQueryCollection.LookbackTime, m.detectedVersion.supportsQuerySampleText())
	if err != nil {
		m.logger.Error("Failed to fetch top queries", zap.Error(err))
		errs.AddPartial(1, err)
		return
	}

	sumTimerWaitInPicoSecondsDiff := make([]int64, len(queries))
	for i, q := range queries {
		if cached, diff := m.cacheAndDiff(q.schemaName, q.digest, "sum_timer_wait", q.sumTimerWaitInPicoSeconds); cached && diff > 0 {
			sumTimerWaitInPicoSecondsDiff[i] = diff
		}
	}

	// sort the rows based on the sumTimerWaitInPicoSecondsDiff in descending order,
	// only report first T(T=topQueryCount) rows.
	queries = sortTopQueries(queries, sumTimerWaitInPicoSecondsDiff, m.config.TopQueryCollection.TopQueryCount)

	// sort the totalElapsedTimeDiffs in descending order as well
	sort.Slice(sumTimerWaitInPicoSecondsDiff, func(i, j int) bool { return sumTimerWaitInPicoSecondsDiff[i] > sumTimerWaitInPicoSecondsDiff[j] })

	m.lastExecutionTimestamp = now.AsTime()

	for i, q := range queries {
		// skip the rest queries due to desc order
		if sumTimerWaitInPicoSecondsDiff[i] == 0 {
			break
		}

		sumTimerWaitVal := float64(sumTimerWaitInPicoSecondsDiff[i]) / 1_000_000_000_000.0 // convert to seconds

		cached, countStarVal := m.cacheAndDiff(q.schemaName, q.digest, "count_star", q.countStar)
		if !cached {
			countStarVal = 0
		}

		obfuscatedQuery, err := m.obfuscator.obfuscateSQLString(q.digestText)
		if err != nil {
			m.logger.Error("Failed to obfuscate query", zap.Error(err))
		}

		var queryPlan string
		// querySampleText is "" when the fallback template was used (MySQL <8 / MariaDB).
		// Skip EXPLAIN in that case — there is no sample statement to explain.
		if q.querySampleText != "" {
			cacheKey := q.schemaName + "-" + q.digest
			var ok bool
			if queryPlan, ok = m.queryPlanCache.Get(cacheKey); !ok {
				// attempt to explain the query
				queryPlan = m.sqlclient.explainQuery(q.digestText, q.querySampleText, q.schemaName, q.digest, m.logger)
				if queryPlan == "" {
					m.logger.Debug("query plan not available", zap.String("digest", q.digest), zap.String("digest_text", q.digestText))
				} else {
					// Obfuscate the plan
					queryPlan, err = m.obfuscator.obfuscatePlan(queryPlan)
					if err != nil {
						// Obfuscation returned an error, log it. We cannot publish the unobfuscated plan as it may contain sensitive data.
						// queryPlan is intentionally left as "" so the cache entry below suppresses
						// repeated EXPLAIN attempts for this digest — EXPLAIN succeeds or fails
						// consistently for a given query shape, so retrying every scrape cycle
						// provides no benefit and adds unnecessary load.
						m.logger.Error("Failed to obfuscate query plan", zap.Error(err))
					}
				}
				// Cache the result (including "" for unavailable/failed plans) so that
				// scrapeQuerySamples can reuse it and EXPLAIN is not re-issued every cycle.
				m.queryPlanCache.Add(cacheKey, queryPlan)
			}
		}

		queryPlanHash := ""
		if queryPlan != "" {
			queryPlanHash = q.digest
		}

		m.lb.RecordDbServerTopQueryEvent(
			context.Background(),
			now,
			metadata.AttributeDbSystemNameMysql,
			obfuscatedQuery,
			queryPlan,
			queryPlanHash,
			q.digest,
			countStarVal,
			sumTimerWaitVal,
		)
	}
}

func (m *mySQLScraper) scrapeQuerySamples(_ context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	samples, err := m.sqlclient.getQuerySamples(m.config.QuerySampleCollection.MaxRowsPerQuery, m.detectedVersion.supportsProcesslist())
	if err != nil {
		m.logger.Error("Failed to fetch query samples", zap.Error(err))
		errs.AddPartial(1, err)
		return
	}

	for i := range samples {
		sample := &samples[i]
		clientAddress := sample.processlistHost
		clientPort := int64(sample.clientPort)
		networkPeerAddress := clientAddress
		networkPeerPort := clientPort

		obfuscatedQuery, obfErr := m.obfuscator.obfuscateSQLString(sample.sqlText)
		if obfErr != nil {
			m.logger.Error("Failed to obfuscate query", zap.Error(obfErr))
		}

		// Use context.Background() as the default (not the scraper ctx) so that log
		// records carry empty trace/span IDs when no application traceparent is present.
		// This prevents the collector's own internal scrape span from being stamped onto
		// query-sample records. If the sample carries a W3C traceparent, extract the
		// application's trace context from it.
		recordCtx := context.Background()
		if sample.traceparent != "" {
			var tpErr error
			recordCtx, tpErr = contextWithTraceparent(sample.traceparent)
			if tpErr != nil {
				m.logger.Warn("Invalid traceparent; omitting trace context", zap.String("presented-traceparent", sample.traceparent), zap.String("db.query.digest", sample.digest), zap.Error(tpErr))
			}
		}

		m.lb.RecordDbServerQuerySampleEvent(
			recordCtx,
			now,
			metadata.AttributeDbSystemNameMysql,
			sample.threadID,
			sample.processlistUser,
			sample.processlistDB,
			sample.processlistCommand,
			sample.processlistState,
			obfuscatedQuery,
			sample.digest,
			sample.digest,
			sample.eventID,
			sample.waitEvent,
			sample.sessionStatus,
			sample.sessionID,
			sample.statementTimerWait,
			sample.waitTime,
			clientAddress,
			clientPort,
			networkPeerAddress,
			networkPeerPort,
		)
	}
}

// contextWithTraceparent extracts a W3C TraceContext traceparent from the given
// string and returns a new context.Background()-based context carrying the
// resulting span context. On failure (invalid or absent traceparent), returns
// an undecorated context.Background() and a non-nil error.
func contextWithTraceparent(traceparent string) (context.Context, error) {
	newCtx := propagation.TraceContext{}.Extract(context.Background(), propagation.MapCarrier{
		"traceparent": traceparent,
	})
	if trace.SpanContextFromContext(newCtx).IsValid() {
		return newCtx, nil
	}
	return context.Background(), errors.New("no valid span context extracted from traceparent")
}

func addPartialIfError(errors *scrapererror.ScrapeErrors, err error) {
	if err != nil {
		errors.AddPartial(1, err)
	}
}

func (m *mySQLScraper) recordDataPages(now pcommon.Timestamp, globalStats map[string]string, errors *scrapererror.ScrapeErrors) {
	dirty, err := parseInt(globalStats["Innodb_buffer_pool_pages_dirty"])
	if err != nil {
		errors.AddPartial(2, err) // we need dirty to calculate free, so 2 data points lost here
		return
	}
	m.mb.RecordMysqlBufferPoolDataPagesDataPoint(now, dirty, metadata.AttributeBufferPoolDataDirty)

	data, err := parseInt(globalStats["Innodb_buffer_pool_pages_data"])
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	m.mb.RecordMysqlBufferPoolDataPagesDataPoint(now, data-dirty, metadata.AttributeBufferPoolDataClean)
}

func (m *mySQLScraper) recordDataUsage(now pcommon.Timestamp, globalStats map[string]string, errors *scrapererror.ScrapeErrors) {
	dirty, err := parseInt(globalStats["Innodb_buffer_pool_bytes_dirty"])
	if err != nil {
		errors.AddPartial(2, err) // we need dirty to calculate free, so 2 data points lost here
		return
	}
	m.mb.RecordMysqlBufferPoolUsageDataPoint(now, dirty, metadata.AttributeBufferPoolDataDirty)

	data, err := parseInt(globalStats["Innodb_buffer_pool_bytes_data"])
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	m.mb.RecordMysqlBufferPoolUsageDataPoint(now, data-dirty, metadata.AttributeBufferPoolDataClean)
}

// parseInt converts string to int64.
func parseInt(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

// cacheAndDiff store row(in int) with schema name and digest variables
// (1) returns true if the key is cached before
// (2) returns positive value if the value is larger than the cached value
func (m *mySQLScraper) cacheAndDiff(schemaName, digest, column string, val int64) (bool, int64) {
	if val < 0 {
		return false, 0
	}

	key := schemaName + "-" + digest + "-" + column

	cached, ok := m.cache.Get(key)
	if !ok {
		m.cache.Add(key, val)
		return false, val
	}

	if val > cached {
		m.cache.Add(key, val)
		return true, val - cached
	}

	// val < cached means the DB counter was reset (e.g. after a DB restart).
	// Treat the current value as the full diff since the reset and refresh the cache.
	if val < cached {
		m.cache.Add(key, val)
		return true, val
	}

	return true, 0
}

// sortTopQueries sorts the top queries based on the `values` slice in descending order and returns the first M(M=maximum) queries
// Input: (row: [query1, query2, query3], values: [100, 10, 1000], maximum: 2
// Expected Output: (row: [query3, query1])
func sortTopQueries(queries []topQuery, values []int64, maximum uint64) []topQuery {
	results := make([]topQuery, 0)

	if len(queries) == 0 ||
		len(values) == 0 ||
		len(queries) != len(values) ||
		maximum <= 0 {
		return []topQuery{}
	}
	pq := make(priorityqueue.PriorityQueue[topQuery, int64], len(queries))
	for i, q := range queries {
		value := values[i]
		pq[i] = &priorityqueue.QueueItem[topQuery, int64]{
			Value:    q,
			Priority: value,
			Index:    i,
		}
	}
	heap.Init(&pq)

	for pq.Len() > 0 && len(results) < int(maximum) {
		item := heap.Pop(&pq).(*priorityqueue.QueueItem[topQuery, int64])
		results = append(results, item.Value)
	}
	return results
}
