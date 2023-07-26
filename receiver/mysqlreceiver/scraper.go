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
	rb        *metadata.ResourceBuilder
	mb        *metadata.MetricsBuilder

	// Feature gates regarding resource attributes
	renameCommands bool
}

func newMySQLScraper(
	settings receiver.CreateSettings,
	config *Config,
) *mySQLScraper {
	ms := &mySQLScraper{
		logger: settings.Logger,
		config: config,
		rb:     metadata.NewResourceBuilder(config.MetricsBuilderConfig.ResourceAttributes),
		mb:     metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
	}

	return ms
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

	// collect performance event statements metrics.
	m.scrapeStatementEventsStats(now, errs)
	// collect lock table events metrics
	m.scrapeTableLockWaitEventStats(now, errs)

	// collect global status metrics.
	m.scrapeGlobalStats(now, errs)

	// colect replicas status metrics.
	m.scrapeReplicaStatusStats(now)

	m.rb.SetMysqlInstanceEndpoint(m.config.Endpoint)
	m.mb.EmitForResource(metadata.WithResource(m.rb.Emit()))

	return m.mb.Emit(), errs.Combine()
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
		case "Innodb_buffer_pool_pages_misc":
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
		case "Com_insert":
			addPartialIfError(errs, m.mb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandInsert))
		case "Com_select":
			addPartialIfError(errs, m.mb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandSelect))
		case "Com_update":
			addPartialIfError(errs, m.mb.RecordMysqlCommandsDataPoint(now, v, metadata.AttributeCommandUpdate))

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
		}
	}
}

func (m *mySQLScraper) scrapeTableIoWaitsStats(now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	tableIoWaitsStats, err := m.sqlclient.getTableIoWaitsStats()
	if err != nil {
		m.logger.Error("Failed to fetch table io_waits stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := 0; i < len(tableIoWaitsStats); i++ {
		s := tableIoWaitsStats[i]
		// counts
		m.mb.RecordMysqlTableIoWaitCountDataPoint(now, s.countDelete, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema)
		m.mb.RecordMysqlTableIoWaitCountDataPoint(now, s.countFetch, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema)
		m.mb.RecordMysqlTableIoWaitCountDataPoint(now, s.countInsert, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema)
		m.mb.RecordMysqlTableIoWaitCountDataPoint(now, s.countUpdate, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema)

		// times
		m.mb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeDelete/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema,
		)
		m.mb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeFetch/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema,
		)
		m.mb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeInsert/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema,
		)
		m.mb.RecordMysqlTableIoWaitTimeDataPoint(
			now, s.timeUpdate/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema,
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

	for i := 0; i < len(indexIoWaitsStats); i++ {
		s := indexIoWaitsStats[i]
		// counts
		m.mb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countDelete, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema, s.index)
		m.mb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countFetch, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema, s.index)
		m.mb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countInsert, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema, s.index)
		m.mb.RecordMysqlIndexIoWaitCountDataPoint(now, s.countUpdate, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema, s.index)

		// times
		m.mb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeDelete/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsDelete, s.name, s.schema, s.index,
		)
		m.mb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeFetch/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsFetch, s.name, s.schema, s.index,
		)
		m.mb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeInsert/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsInsert, s.name, s.schema, s.index,
		)
		m.mb.RecordMysqlIndexIoWaitTimeDataPoint(
			now, s.timeUpdate/picosecondsInNanoseconds, metadata.AttributeIoWaitsOperationsUpdate, s.name, s.schema, s.index,
		)
	}
}

func (m *mySQLScraper) scrapeStatementEventsStats(now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	statementEventsStats, err := m.sqlclient.getStatementEventsStats()
	if err != nil {
		m.logger.Error("Failed to fetch index io_waits stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := 0; i < len(statementEventsStats); i++ {
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

		m.mb.RecordMysqlStatementEventWaitTimeDataPoint(now, s.sumTimerWait/picosecondsInNanoseconds, s.schema, s.digest, s.digestText)
	}
}

func (m *mySQLScraper) scrapeTableLockWaitEventStats(now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	tableLockWaitEventStats, err := m.sqlclient.getTableLockWaitEventStats()
	if err != nil {
		m.logger.Error("Failed to fetch index io_waits stats", zap.Error(err))
		errs.AddPartial(8, err)
		return
	}

	for i := 0; i < len(tableLockWaitEventStats); i++ {
		s := tableLockWaitEventStats[i]
		// read data points
		m.mb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadNormal, s.schema, s.name, metadata.AttributeReadLockTypeNormal)
		m.mb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadWithSharedLocks, s.schema, s.name, metadata.AttributeReadLockTypeWithSharedLocks)
		m.mb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadHighPriority, s.schema, s.name, metadata.AttributeReadLockTypeHighPriority)
		m.mb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadNoInsert, s.schema, s.name, metadata.AttributeReadLockTypeNoInsert)
		m.mb.RecordMysqlTableLockWaitReadCountDataPoint(now, s.countReadExternal, s.schema, s.name, metadata.AttributeReadLockTypeExternal)

		// read time data points
		m.mb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadNormal/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeReadLockTypeNormal)
		m.mb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadWithSharedLocks/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeReadLockTypeWithSharedLocks)
		m.mb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadHighPriority/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeReadLockTypeHighPriority)
		m.mb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadNoInsert/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeReadLockTypeNoInsert)
		m.mb.RecordMysqlTableLockWaitReadTimeDataPoint(now, s.sumTimerReadExternal/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeReadLockTypeExternal)

		// write data points
		m.mb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteAllowWrite, s.schema, s.name, metadata.AttributeWriteLockTypeAllowWrite)
		m.mb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteConcurrentInsert, s.schema, s.name, metadata.AttributeWriteLockTypeConcurrentInsert)
		m.mb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteLowPriority, s.schema, s.name, metadata.AttributeWriteLockTypeLowPriority)
		m.mb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteNormal, s.schema, s.name, metadata.AttributeWriteLockTypeNormal)
		m.mb.RecordMysqlTableLockWaitWriteCountDataPoint(now, s.countWriteExternal, s.schema, s.name, metadata.AttributeWriteLockTypeExternal)

		// write time data points
		m.mb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteAllowWrite/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeWriteLockTypeAllowWrite)
		m.mb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteConcurrentInsert/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeWriteLockTypeConcurrentInsert)
		m.mb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteLowPriority/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeWriteLockTypeLowPriority)
		m.mb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteNormal/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeWriteLockTypeNormal)
		m.mb.RecordMysqlTableLockWaitWriteTimeDataPoint(now, s.sumTimerWriteExternal/picosecondsInNanoseconds, s.schema, s.name, metadata.AttributeWriteLockTypeExternal)
	}
}

func (m *mySQLScraper) scrapeReplicaStatusStats(now pcommon.Timestamp) {
	replicaStatusStats, err := m.sqlclient.getReplicaStatusStats()
	if err != nil {
		m.logger.Info("Failed to fetch replica status stats", zap.Error(err))
		return
	}

	for i := 0; i < len(replicaStatusStats); i++ {
		s := replicaStatusStats[i]

		val, _ := s.secondsBehindSource.Value()
		if val != nil {
			m.mb.RecordMysqlReplicaTimeBehindSourceDataPoint(now, val.(int64))
		}

		m.mb.RecordMysqlReplicaSQLDelayDataPoint(now, s.sqlDelay)
	}
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
