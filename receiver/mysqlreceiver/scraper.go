// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"context"
	"errors"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
)

const (
	picosecondsInNanoseconds int64 = 1000
)

type mySQLScraper struct {
	sqlclient client
	logger    *zap.Logger
	config    *Config
	mb        *metadata.MetricsBuilder

	// Feature gates regarding resource attributes
	renameCommands bool
}

func newMySQLScraper(
	settings receiver.CreateSettings,
	config *Config,
) *mySQLScraper {
	return &mySQLScraper{
		logger: settings.Logger,
		config: config,
		mb:     metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
	}
}

// start starts the scraper by initializing the db client connection.
func (m *mySQLScraper) start(_ context.Context, _ component.Host) error {
	sqlclient := newMySQLClient(m.config)

	err := sqlclient.Connect()
	if err != nil {
		return err
	}
	m.sqlclient = sqlclient

	return nil
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

	rb := m.mb.NewResourceBuilder()
	rb.SetMysqlInstanceEndpoint(m.config.Endpoint)
	rmb := m.mb.ResourceMetricsBuilder(rb.Emit())

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
		addPartialIfError(errs, rmb.RecordMysqlBufferPoolLimitDataPoint(now, v))
	}

	// collect io_waits metrics.
	m.scrapeTableIoWaitsStats(rmb, now, errs)
	m.scrapeIndexIoWaitsStats(rmb, now, errs)

	// collect performance event statements metrics.
	m.scrapeStatementEventsStats(rmb, now, errs)
	// collect lock table events metrics
	m.scrapeTableLockWaitEventStats(rmb, now, errs)

	// collect global status metrics.
	m.scrapeGlobalStats(rmb, now, errs)

	// colect replicas status metrics.
	m.scrapeReplicaStatusStats(rmb, now)

	return m.mb.Emit(), errs.Combine()
}

func (m *mySQLScraper) scrapeGlobalStats(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	globalStats, err := m.sqlclient.getGlobalStats()
	if err != nil {
		m.logger.Error("Failed to fetch global stats", zap.Error(err))
		errs.AddPartial(66, err)
		return
	}

	m.recordDataPages(rmb, now, globalStats, errs)
	m.recordDataUsage(rmb, now, globalStats, errs)

	for k, v := range globalStats {
		switch k {
		// bytes transmission
		case "Bytes_received":
			addPartialIfError(errs, rmb.RecordMysqlClientNetworkIoDataPoint(now, v, metadata.AttributeDirectionReceived))
		case "Bytes_sent":
			addPartialIfError(errs, rmb.RecordMysqlClientNetworkIoDataPoint(now, v, metadata.AttributeDirectionSent))

		// buffer_pool.pages
		case "Innodb_buffer_pool_pages_data":
			addPartialIfError(errs, rmb.RecordMysqlBufferPoolPagesDataPoint(now, v,
				metadata.AttributeBufferPoolPagesData))
		case "Innodb_buffer_pool_pages_free":
			addPartialIfError(errs, rmb.RecordMysqlBufferPoolPagesDataPoint(now, v,
				metadata.AttributeBufferPoolPagesFree))
		case "Innodb_buffer_pool_pages_misc":
			addPartialIfError(errs, rmb.RecordMysqlBufferPoolPagesDataPoint(now, v,
				metadata.AttributeBufferPoolPagesMisc))

		// buffer_pool.page_flushes
		case "Innodb_buffer_pool_pages_flushed":
			addPartialIfError(errs, rmb.RecordMysqlBufferPoolPageFlushesDataPoint(now, v))

		// buffer_pool.operations
		case "Innodb_buffer_pool_read_ahead_rnd":
			addPartialIfError(errs, rmb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsReadAheadRnd))
		case "Innodb_buffer_pool_read_ahead":
			addPartialIfError(errs, rmb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsReadAhead))
		case "Innodb_buffer_pool_read_ahead_evicted":
			addPartialIfError(errs, rmb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsReadAheadEvicted))
		case "Innodb_buffer_pool_read_requests":
			addPartialIfError(errs, rmb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsReadRequests))
		case "Innodb_buffer_pool_reads":
			addPartialIfError(errs, rmb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsReads))
		case "Innodb_buffer_pool_wait_free":
			addPartialIfError(errs, rmb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsWaitFree))
		case "Innodb_buffer_pool_write_requests":
			addPartialIfError(errs, rmb.RecordMysqlBufferPoolOperationsDataPoint(now, v,
				metadata.AttributeBufferPoolOperationsWriteRequests))

		// connection.errors
		case "Connection_errors_accept":
			addPartialIfError(errs, rmb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorAccept))
		case "Connection_errors_internal":
			addPartialIfError(errs, rmb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorInternal))
		case "Connection_errors_max_connections":
			addPartialIfError(errs, rmb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorMaxConnections))
		case "Connection_errors_peer_address":
			addPartialIfError(errs, rmb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorPeerAddress))
		case "Connection_errors_select":
			addPartialIfError(errs, rmb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorSelect))
		case "Connection_errors_tcpwrap":
			addPartialIfError(errs, rmb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorTcpwrap))
		case "Aborted_clients":
			addPartialIfError(errs, rmb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorAbortedClients))
		case "Aborted_connects":
			addPartialIfError(errs, rmb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorAborted))
		case "Locked_connects":
			addPartialIfError(errs, rmb.RecordMysqlConnectionErrorsDataPoint(now, v,
				metadata.AttributeConnectionErrorLocked))

		// connection
		case "Connections":
			addPartialIfError(errs, rmb.RecordMysqlConnectionCountDataPoint(now, v))

		// prepared_statements_commands
		case "Com_stmt_execute":
			addPartialIfError(errs, rmb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandExecute))
		case "Com_stmt_close":
			addPartialIfError(errs, rmb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandClose))
		case "Com_stmt_fetch":
			addPartialIfError(errs, rmb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandFetch))
		case "Com_stmt_prepare":
			addPartialIfError(errs, rmb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandPrepare))
		case "Com_stmt_reset":
			addPartialIfError(errs, rmb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandReset))
		case "Com_stmt_send_long_data":
			addPartialIfError(errs, rmb.RecordMysqlPreparedStatementsDataPoint(now, v,
				metadata.AttributePreparedStatementsCommandSendLongData))

		// commands
		case "Com_delete":
			addPartialIfError(errs, rmb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandDelete))
		case "Com_insert":
			addPartialIfError(errs, rmb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandInsert))
		case "Com_select":
			addPartialIfError(errs, rmb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandSelect))
		case "Com_update":
			addPartialIfError(errs, rmb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandUpdate))

		// created tmps
		case "Created_tmp_disk_tables":
			addPartialIfError(errs, rmb.RecordMysqlTmpResourcesDataPoint(now, v, metadata.AttributeTmpResourceDiskTables))
		case "Created_tmp_files":
			addPartialIfError(errs, rmb.RecordMysqlTmpResourcesDataPoint(now, v, metadata.AttributeTmpResourceFiles))
		case "Created_tmp_tables":
			addPartialIfError(errs, rmb.RecordMysqlTmpResourcesDataPoint(now, v, metadata.AttributeTmpResourceTables))

		// handlers
		case "Handler_commit":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerCommit))
		case "Handler_delete":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerDelete))
		case "Handler_discover":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerDiscover))
		case "Handler_external_lock":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerExternalLock))
		case "Handler_mrr_init":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerMrrInit))
		case "Handler_prepare":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerPrepare))
		case "Handler_read_first":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadFirst))
		case "Handler_read_key":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadKey))
		case "Handler_read_last":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadLast))
		case "Handler_read_next":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadNext))
		case "Handler_read_prev":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadPrev))
		case "Handler_read_rnd":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadRnd))
		case "Handler_read_rnd_next":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerReadRndNext))
		case "Handler_rollback":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerRollback))
		case "Handler_savepoint":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerSavepoint))
		case "Handler_savepoint_rollback":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerSavepointRollback))
		case "Handler_update":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerUpdate))
		case "Handler_write":
			addPartialIfError(errs, rmb.RecordMysqlHandlersDataPoint(now, v, metadata.AttributeHandlerWrite))

		// double_writes
		case "Innodb_dblwr_pages_written":
			addPartialIfError(errs, rmb.RecordMysqlDoubleWritesDataPoint(now, v, metadata.AttributeDoubleWritesPagesWritten))
		case "Innodb_dblwr_writes":
			addPartialIfError(errs, rmb.RecordMysqlDoubleWritesDataPoint(now, v, metadata.AttributeDoubleWritesWrites))

		// log_operations
		case "Innodb_log_waits":
			addPartialIfError(errs, rmb.RecordMysqlLogOperationsDataPoint(now, v, metadata.AttributeLogOperationsWaits))
		case "Innodb_log_write_requests":
			addPartialIfError(errs, rmb.RecordMysqlLogOperationsDataPoint(now, v, metadata.AttributeLogOperationsWriteRequests))
		case "Innodb_log_writes":
			addPartialIfError(errs, rmb.RecordMysqlLogOperationsDataPoint(now, v, metadata.AttributeLogOperationsWrites))

		// operations
		case "Innodb_data_fsyncs":
			addPartialIfError(errs, rmb.RecordMysqlOperationsDataPoint(now, v, metadata.AttributeOperationsFsyncs))
		case "Innodb_data_reads":
			addPartialIfError(errs, rmb.RecordMysqlOperationsDataPoint(now, v, metadata.AttributeOperationsReads))
		case "Innodb_data_writes":
			addPartialIfError(errs, rmb.RecordMysqlOperationsDataPoint(now, v, metadata.AttributeOperationsWrites))

		// page_operations
		case "Innodb_pages_created":
			addPartialIfError(errs, rmb.RecordMysqlPageOperationsDataPoint(now, v, metadata.AttributePageOperationsCreated))
		case "Innodb_pages_read":
			addPartialIfError(errs, rmb.RecordMysqlPageOperationsDataPoint(now, v,
				metadata.AttributePageOperationsRead))
		case "Innodb_pages_written":
			addPartialIfError(errs, rmb.RecordMysqlPageOperationsDataPoint(now, v,
				metadata.AttributePageOperationsWritten))

		// row_locks
		case "Innodb_row_lock_waits":
			addPartialIfError(errs, rmb.RecordMysqlRowLocksDataPoint(now, v, metadata.AttributeRowLocksWaits))
		case "Innodb_row_lock_time":
			addPartialIfError(errs, rmb.RecordMysqlRowLocksDataPoint(now, v, metadata.AttributeRowLocksTime))

		// row_operations
		case "Innodb_rows_deleted":
			addPartialIfError(errs, rmb.RecordMysqlRowOperationsDataPoint(now, v, metadata.AttributeRowOperationsDeleted))
		case "Innodb_rows_inserted":
			addPartialIfError(errs, rmb.RecordMysqlRowOperationsDataPoint(now, v, metadata.AttributeRowOperationsInserted))
		case "Innodb_rows_read":
			addPartialIfError(errs, rmb.RecordMysqlRowOperationsDataPoint(now, v,
				metadata.AttributeRowOperationsRead))
		case "Innodb_rows_updated":
			addPartialIfError(errs, rmb.RecordMysqlRowOperationsDataPoint(now, v,
				metadata.AttributeRowOperationsUpdated))

		// locks
		case "Table_locks_immediate":
			addPartialIfError(errs, rmb.RecordMysqlLocksDataPoint(now, v, metadata.AttributeLocksImmediate))
		case "Table_locks_waited":
			addPartialIfError(errs, rmb.RecordMysqlLocksDataPoint(now, v, metadata.AttributeLocksWaited))

		// joins
		case "Select_full_join":
			addPartialIfError(errs, rmb.RecordMysqlJoinsDataPoint(now, v, metadata.AttributeJoinKindFull))
		case "Select_full_range_join":
			addPartialIfError(errs, rmb.RecordMysqlJoinsDataPoint(now, v, metadata.AttributeJoinKindFullRange))
		case "Select_range":
			addPartialIfError(errs, rmb.RecordMysqlJoinsDataPoint(now, v, metadata.AttributeJoinKindRange))
		case "Select_range_check":
			addPartialIfError(errs, rmb.RecordMysqlJoinsDataPoint(now, v, metadata.AttributeJoinKindRangeCheck))
		case "Select_scan":
			addPartialIfError(errs, rmb.RecordMysqlJoinsDataPoint(now, v, metadata.AttributeJoinKindScan))

		// open cache
		case "Table_open_cache_hits":
			addPartialIfError(errs, rmb.RecordMysqlTableOpenCacheDataPoint(now, v, metadata.AttributeCacheStatusHit))
		case "Table_open_cache_misses":
			addPartialIfError(errs, rmb.RecordMysqlTableOpenCacheDataPoint(now, v, metadata.AttributeCacheStatusMiss))
		case "Table_open_cache_overflows":
			addPartialIfError(errs, rmb.RecordMysqlTableOpenCacheDataPoint(now, v, metadata.AttributeCacheStatusOverflow))

		// queries
		case "Queries":
			addPartialIfError(errs, rmb.RecordMysqlQueryCountDataPoint(now, v))
		case "Questions":
			addPartialIfError(errs, rmb.RecordMysqlQueryClientCountDataPoint(now, v))
		case "Slow_queries":
			addPartialIfError(errs, rmb.RecordMysqlQuerySlowCountDataPoint(now, v))

		// sorts
		case "Sort_merge_passes":
			addPartialIfError(errs, rmb.RecordMysqlSortsDataPoint(now, v, metadata.AttributeSortsMergePasses))
		case "Sort_range":
			addPartialIfError(errs, rmb.RecordMysqlSortsDataPoint(now, v, metadata.AttributeSortsRange))
		case "Sort_rows":
			addPartialIfError(errs, rmb.RecordMysqlSortsDataPoint(now, v, metadata.AttributeSortsRows))
		case "Sort_scan":
			addPartialIfError(errs, rmb.RecordMysqlSortsDataPoint(now, v, metadata.AttributeSortsScan))

		// threads
		case "Threads_cached":
			addPartialIfError(errs, rmb.RecordMysqlThreadsDataPoint(now, v, metadata.AttributeThreadsCached))
		case "Threads_connected":
			addPartialIfError(errs, rmb.RecordMysqlThreadsDataPoint(now, v, metadata.AttributeThreadsConnected))
		case "Threads_created":
			addPartialIfError(errs, rmb.RecordMysqlThreadsDataPoint(now, v, metadata.AttributeThreadsCreated))
		case "Threads_running":
			addPartialIfError(errs, rmb.RecordMysqlThreadsDataPoint(now, v, metadata.AttributeThreadsRunning))

		// opened resources
		case "Opened_files":
			addPartialIfError(errs, rmb.RecordMysqlOpenedResourcesDataPoint(now, v, metadata.AttributeOpenedResourcesFile))
		case "Opened_tables":
			addPartialIfError(errs, rmb.RecordMysqlOpenedResourcesDataPoint(now, v, metadata.AttributeOpenedResourcesTable))
		case "Opened_table_definitions":
			addPartialIfError(errs, rmb.RecordMysqlOpenedResourcesDataPoint(now, v, metadata.AttributeOpenedResourcesTableDefinition))

		// mysqlx_worker_threads
		case "Mysqlx_worker_threads":
			addPartialIfError(errs, rmb.RecordMysqlMysqlxWorkerThreadsDataPoint(now, v, metadata.AttributeMysqlxThreadsAvailable))
		case "Mysqlx_worker_threads_active":
			addPartialIfError(errs, rmb.RecordMysqlMysqlxWorkerThreadsDataPoint(now, v, metadata.AttributeMysqlxThreadsActive))

		// mysqlx_connections
		case "Mysqlx_connections_accepted":
			addPartialIfError(errs, rmb.RecordMysqlMysqlxConnectionsDataPoint(now, v, metadata.AttributeConnectionStatusAccepted))
		case "Mysqlx_connections_closed":
			addPartialIfError(errs, rmb.RecordMysqlMysqlxConnectionsDataPoint(now, v, metadata.AttributeConnectionStatusClosed))
		case "Mysqlx_connections_rejected":
			addPartialIfError(errs, rmb.RecordMysqlMysqlxConnectionsDataPoint(now, v, metadata.AttributeConnectionStatusRejected))

		// uptime
		case "Uptime":
			addPartialIfError(errs, rmb.RecordMysqlUptimeDataPoint(now, v))
		}
	}
}

func (m *mySQLScraper) scrapeTableIoWaitsStats(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	tableIoWaitsStats, err := m.sqlclient.getTableIoWaitsStats()
	if err != nil {
		m.logger.Error("Failed to fetch table io_waits stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := 0; i < len(tableIoWaitsStats); i++ {
		s := tableIoWaitsStats[i]
		// counts
		rmb.RecordMysqlTableIoWaitCountDataPoint(now, s.countDelete, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema)
		rmb.RecordMysqlTableIoWaitCountDataPoint(now, s.countFetch, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema)
		rmb.RecordMysqlTableIoWaitCountDataPoint(now, s.countInsert, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema)
		rmb.RecordMysqlTableIoWaitCountDataPoint(now, s.countUpdate, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema)

		// times
		rmb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeDelete/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema,
		)
		rmb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeFetch/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema,
		)
		rmb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeInsert/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema,
		)
		rmb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeUpdate/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema,
		)
	}
}

func (m *mySQLScraper) scrapeIndexIoWaitsStats(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	indexIoWaitsStats, err := m.sqlclient.getIndexIoWaitsStats()
	if err != nil {
		m.logger.Error("Failed to fetch index io_waits stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := 0; i < len(indexIoWaitsStats); i++ {
		s := indexIoWaitsStats[i]
		// counts
		rmb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countDelete, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema, s.index)
		rmb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countFetch, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema, s.index)
		rmb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countInsert, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema, s.index)
		rmb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countUpdate, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema, s.index)

		// times
		rmb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeDelete/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema, s.index,
		)
		rmb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeFetch/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema, s.index,
		)
		rmb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeInsert/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema, s.index,
		)
		rmb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeUpdate/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema, s.index,
		)
	}
}

func (m *mySQLScraper) scrapeStatementEventsStats(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	statementEventsStats, err := m.sqlclient.getStatementEventsStats()
	if err != nil {
		m.logger.Error("Failed to fetch index io_waits stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := 0; i < len(statementEventsStats); i++ {
		s := statementEventsStats[i]
		rmb.RecordMysqlStatementEventCountDataPoint(now, s.countCreatedTmpDiskTables, s.schema, s.digest, s.digestText, metadata.AttributeEventStateCreatedTmpDiskTables)
		rmb.RecordMysqlStatementEventCountDataPoint(now, s.countCreatedTmpTables, s.schema, s.digest, s.digestText, metadata.AttributeEventStateCreatedTmpTables)
		rmb.RecordMysqlStatementEventCountDataPoint(now, s.countErrors, s.schema, s.digest, s.digestText, metadata.AttributeEventStateErrors)
		rmb.RecordMysqlStatementEventCountDataPoint(now, s.countNoIndexUsed, s.schema, s.digest, s.digestText, metadata.AttributeEventStateNoIndexUsed)
		rmb.RecordMysqlStatementEventCountDataPoint(now, s.countRowsAffected, s.schema, s.digest, s.digestText, metadata.AttributeEventStateRowsAffected)
		rmb.RecordMysqlStatementEventCountDataPoint(now, s.countRowsExamined, s.schema, s.digest, s.digestText, metadata.AttributeEventStateRowsExamined)
		rmb.RecordMysqlStatementEventCountDataPoint(now, s.countRowsSent, s.schema, s.digest, s.digestText, metadata.AttributeEventStateRowsSent)
		rmb.RecordMysqlStatementEventCountDataPoint(now, s.countSortMergePasses, s.schema, s.digest, s.digestText, metadata.AttributeEventStateSortMergePasses)
		rmb.RecordMysqlStatementEventCountDataPoint(now, s.countSortRows, s.schema, s.digest, s.digestText, metadata.AttributeEventStateSortRows)
		rmb.RecordMysqlStatementEventCountDataPoint(now, s.countWarnings, s.schema, s.digest, s.digestText, metadata.AttributeEventStateWarnings)

		rmb.RecordMysqlStatementEventWaitTimeDataPoint(now, s.sumTimerWait/picosecondsInNanoseconds, s.schema, s.digest, s.digestText)
	}
}

func (m *mySQLScraper) scrapeTableLockWaitEventStats(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	tableLockWaitEventStats, err := m.sqlclient.getTableLockWaitEventStats()
	if err != nil {
		m.logger.Error("Failed to fetch index io_waits stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := 0; i < len(tableLockWaitEventStats); i++ {
		s := tableLockWaitEventStats[i]
		// read data points
		rmb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadNormal, s.schema, s.name, metadata.AttributeReadLockTypeNormal)
		rmb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadWithSharedLocks, s.schema, s.name, metadata.AttributeReadLockTypeWithSharedLocks)
		rmb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadHighPriority, s.schema, s.name, metadata.AttributeReadLockTypeHighPriority)
		rmb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadNoInsert, s.schema, s.name, metadata.AttributeReadLockTypeNoInsert)
		rmb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadExternal, s.schema, s.name, metadata.AttributeReadLockTypeExternal)

		// read time data points
		rmb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadNormal/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeReadLockTypeNormal)
		rmb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadWithSharedLocks/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeReadLockTypeWithSharedLocks)
		rmb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadHighPriority/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeReadLockTypeHighPriority)
		rmb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadNoInsert/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeReadLockTypeNoInsert)
		rmb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadExternal/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeReadLockTypeExternal)

		// write data points
		rmb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteAllowWrite, s.schema, s.name, metadata.AttributeWriteLockTypeAllowWrite)
		rmb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteConcurrentInsert, s.schema, s.name, metadata.AttributeWriteLockTypeConcurrentInsert)
		rmb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteLowPriority, s.schema, s.name, metadata.AttributeWriteLockTypeLowPriority)
		rmb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteNormal, s.schema, s.name, metadata.AttributeWriteLockTypeNormal)
		rmb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteExternal, s.schema, s.name, metadata.AttributeWriteLockTypeExternal)

		// write time data points
		rmb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteAllowWrite/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeWriteLockTypeAllowWrite)
		rmb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteConcurrentInsert/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeWriteLockTypeConcurrentInsert)
		rmb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteLowPriority/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeWriteLockTypeLowPriority)
		rmb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteNormal/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeWriteLockTypeNormal)
		rmb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteExternal/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeWriteLockTypeExternal)
	}
}

func (m *mySQLScraper) scrapeReplicaStatusStats(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp) {
	replicaStatusStats, err := m.sqlclient.getReplicaStatusStats()
	if err != nil {
		m.logger.Info("Failed to fetch replica status stats", zap.Error(err))
		return
	}

	for i := 0; i < len(replicaStatusStats); i++ {
		s := replicaStatusStats[i]

		val, _ := s.secondsBehindSource.Value()
		if val != nil {
			rmb.RecordMysqlReplicaTimeBehindSourceDataPoint(now, val.(int64))
		}

		rmb.RecordMysqlReplicaSQLDelayDataPoint(now, s.sqlDelay)
	}
}

func addPartialIfError(errors *scrapererror.ScrapeErrors, err error) {
	if err != nil {
		errors.AddPartial(1, err)
	}
}

func (m *mySQLScraper) recordDataPages(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, globalStats map[string]string, errors *scrapererror.ScrapeErrors) {
	dirty, err := parseInt(globalStats["Innodb_buffer_pool_pages_dirty"])
	if err != nil {
		errors.AddPartial(2, err) // we need dirty to calculate free, so 2 data points lost here
		return
	}
	rmb.RecordMysqlBufferPoolDataPagesDataPoint(now, dirty, metadata.AttributeBufferPoolDataDirty)

	data, err := parseInt(globalStats["Innodb_buffer_pool_pages_data"])
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	rmb.RecordMysqlBufferPoolDataPagesDataPoint(now, data-dirty, metadata.AttributeBufferPoolDataClean)
}

func (m *mySQLScraper) recordDataUsage(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, globalStats map[string]string, errors *scrapererror.ScrapeErrors) {
	dirty, err := parseInt(globalStats["Innodb_buffer_pool_bytes_dirty"])
	if err != nil {
		errors.AddPartial(2, err) // we need dirty to calculate free, so 2 data points lost here
		return
	}
	rmb.RecordMysqlBufferPoolUsageDataPoint(now, dirty, metadata.AttributeBufferPoolDataDirty)

	data, err := parseInt(globalStats["Innodb_buffer_pool_bytes_data"])
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	rmb.RecordMysqlBufferPoolUsageDataPoint(now, data-dirty, metadata.AttributeBufferPoolDataClean)
}

// parseInt converts string to int64.
func parseInt(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}
