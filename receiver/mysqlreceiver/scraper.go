// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"context"
	"errors"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
)

type mySQLScraper struct {
	sqlclient client
	logger    *zap.Logger
	config    *Config
	mb        *metadata.MetricsBuilder
}

func newMySQLScraper(
	logger *zap.Logger,
	config *Config,
) *mySQLScraper {
	return &mySQLScraper{
		logger: logger,
		config: config,
		mb:     metadata.NewMetricsBuilder(config.Metrics),
	}
}

// start starts the scraper by initializing the db client connection.
func (m *mySQLScraper) start(_ context.Context, host component.Host) error {
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

	var errors scrapererror.ScrapeErrors
	for k, v := range innodbStats {
		if k != "buffer_pool_size" {
			continue
		}
		m.mb.ParseMysqlBufferPoolLimitDataPoint(now, v, errors)
	}

	// collect global status metrics.
	globalStats, err := m.sqlclient.getGlobalStats()
	if err != nil {
		m.logger.Error("Failed to fetch global stats", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	m.recordDataPages(now, globalStats, errors)
	m.recordDataUsage(now, globalStats, errors)

	for k, v := range globalStats {
		switch k {

		// buffer_pool.pages
		case "Innodb_buffer_pool_pages_data":
			m.mb.ParseMysqlBufferPoolPagesDataPoint(now, v, errors, "data")
		case "Innodb_buffer_pool_pages_free":
			m.mb.ParseMysqlBufferPoolPagesDataPoint(now, v, errors, "free")
		case "Innodb_buffer_pool_pages_misc":
			m.mb.ParseMysqlBufferPoolPagesDataPoint(now, v, errors, "misc")

		// buffer_pool.page_flushes
		case "Innodb_buffer_pool_pages_flushed":
			m.mb.ParseMysqlBufferPoolPageFlushesDataPoint(now, v, errors)

		// buffer_pool.operations
		case "Innodb_buffer_pool_read_ahead_rnd":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, "read_ahead_rnd")
		case "Innodb_buffer_pool_read_ahead":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, "read_ahead")
		case "Innodb_buffer_pool_read_ahead_evicted":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, "read_ahead_evicted")
		case "Innodb_buffer_pool_read_requests":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, "read_requests")
		case "Innodb_buffer_pool_reads":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, "reads")
		case "Innodb_buffer_pool_wait_free":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, "wait_free")
		case "Innodb_buffer_pool_write_requests":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, "write_requests")

		// commands
		case "Com_stmt_execute":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, "execute")
		case "Com_stmt_close":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, "close")
		case "Com_stmt_fetch":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, "fetch")
		case "Com_stmt_prepare":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, "prepare")
		case "Com_stmt_reset":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, "reset")
		case "Com_stmt_send_long_data":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, "send_long_data")

		// handlers
		case "Handler_commit":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "commit")
		case "Handler_delete":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "delete")
		case "Handler_discover":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "discover")
		case "Handler_external_lock":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "lock")
		case "Handler_mrr_init":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "mrr_init")
		case "Handler_prepare":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "prepare")
		case "Handler_read_first":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "read_first")
		case "Handler_read_key":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "read_key")
		case "Handler_read_last":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "read_last")
		case "Handler_read_next":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "read_next")
		case "Handler_read_prev":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "read_prev")
		case "Handler_read_rnd":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "read_rnd")
		case "Handler_read_rnd_next":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "read_rnd_next")
		case "Handler_rollback":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "rollback")
		case "Handler_savepoint":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "savepoint")
		case "Handler_savepoint_rollback":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "savepoint_rollback")
		case "Handler_update":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "update")
		case "Handler_write":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, "write")

		// double_writes
		case "Innodb_dblwr_pages_written":
			m.mb.ParseMysqlDoubleWritesDataPoint(now, v, errors, "written")
		case "Innodb_dblwr_writes":
			m.mb.ParseMysqlDoubleWritesDataPoint(now, v, errors, "writes")

		// log_operations
		case "Innodb_log_waits":
			m.mb.ParseMysqlLogOperationsDataPoint(now, v, errors, "waits")
		case "Innodb_log_write_requests":
			m.mb.ParseMysqlLogOperationsDataPoint(now, v, errors, "requests")
		case "Innodb_log_writes":
			m.mb.ParseMysqlLogOperationsDataPoint(now, v, errors, "writes")

		// operations
		case "Innodb_data_fsyncs":
			m.mb.ParseMysqlOperationsDataPoint(now, v, errors, "fsyncs")
		case "Innodb_data_reads":
			m.mb.ParseMysqlOperationsDataPoint(now, v, errors, "reads")
		case "Innodb_data_writes":
			m.mb.ParseMysqlOperationsDataPoint(now, v, errors, "writes")

		// page_operations
		case "Innodb_pages_created":
			m.mb.ParseMysqlPageOperationsDataPoint(now, v, errors, "created")
		case "Innodb_pages_read":
			m.mb.ParseMysqlPageOperationsDataPoint(now, v, errors, "read")
		case "Innodb_pages_written":
			m.mb.ParseMysqlPageOperationsDataPoint(now, v, errors, "written")

		// row_locks
		case "Innodb_row_lock_waits":
			m.mb.ParseMysqlRowLocksDataPoint(now, v, errors, "waits")
		case "Innodb_row_lock_time":
			m.mb.ParseMysqlRowLocksDataPoint(now, v, errors, "time")

		// row_operations
		case "Innodb_rows_deleted":
			m.mb.ParseMysqlRowOperationsDataPoint(now, v, errors, "deleted")
		case "Innodb_rows_inserted":
			m.mb.ParseMysqlRowOperationsDataPoint(now, v, errors, "inserted")
		case "Innodb_rows_read":
			m.mb.ParseMysqlRowOperationsDataPoint(now, v, errors, "read")
		case "Innodb_rows_updated":
			m.mb.ParseMysqlRowOperationsDataPoint(now, v, errors, "updated")

		// locks
		case "Table_locks_immediate":
			m.mb.ParseMysqlLocksDataPoint(now, v, errors, "immediate")
		case "Table_locks_waited":
			m.mb.ParseMysqlLocksDataPoint(now, v, errors, "waited")

		// sorts
		case "Sort_merge_passes":
			m.mb.ParseMysqlSortsDataPoint(now, v, errors, "merge_passes")
		case "Sort_range":
			m.mb.ParseMysqlSortsDataPoint(now, v, errors, "range")
		case "Sort_rows":
			m.mb.ParseMysqlSortsDataPoint(now, v, errors, "rows")
		case "Sort_scan":
			m.mb.ParseMysqlSortsDataPoint(now, v, errors, "scan")

		// threads
		case "Threads_cached":
			m.mb.ParseMysqlThreadsDataPoint(now, v, errors, "cached")
		case "Threads_connected":
			m.mb.ParseMysqlThreadsDataPoint(now, v, errors, "connected")
		case "Threads_created":
			m.mb.ParseMysqlThreadsDataPoint(now, v, errors, "created")
		case "Threads_running":
			m.mb.ParseMysqlThreadsDataPoint(now, v, errors, "running")
		}
	}

	return m.mb.Emit(), errors.Combine()
}

func (m *mySQLScraper) recordDataPages(now pcommon.Timestamp, globalStats map[string]string, errors scrapererror.ScrapeErrors) {
	dirty, err := parseInt(globalStats["Innodb_buffer_pool_pages_dirty"])
	if err != nil {
		errors.AddPartial(2, err) // we need dirty to calculate free, so 2 data points lost here
		return
	}
	m.mb.RecordMysqlBufferPoolDataPagesDataPoint(now, dirty, "dirty")

	data, err := parseInt(globalStats["Innodb_buffer_pool_pages_data"])
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	m.mb.RecordMysqlBufferPoolDataPagesDataPoint(now, data-dirty, "clean")
}

func (m *mySQLScraper) recordDataUsage(now pcommon.Timestamp, globalStats map[string]string, errors scrapererror.ScrapeErrors) {
	dirty, err := parseInt(globalStats["Innodb_buffer_pool_bytes_dirty"])
	if err != nil {
		errors.AddPartial(2, err) // we need dirty to calculate free, so 2 data points lost here
		return
	}
	m.mb.RecordMysqlBufferPoolUsageDataPoint(now, dirty, "dirty")

	data, err := parseInt(globalStats["Innodb_buffer_pool_bytes_data"])
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	m.mb.RecordMysqlBufferPoolUsageDataPoint(now, data-dirty, "clean")
}

// parseInt converts string to int64.
func parseInt(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}
