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
	"go.opentelemetry.io/collector/model/pdata"
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
func (m *mySQLScraper) scrape(context.Context) (pdata.Metrics, error) {
	if m.sqlclient == nil {
		return pdata.Metrics{}, errors.New("failed to connect to http client")
	}

	now := pdata.NewTimestampFromTime(time.Now())

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
		m.mb.ParseMysqlBufferPoolLimitDataPoint(now, v, errors, m.logger)
	}

	// collect global status metrics.
	globalStats, err := m.sqlclient.getGlobalStats()
	if err != nil {
		m.logger.Error("Failed to fetch global stats", zap.Error(err))
		return pdata.Metrics{}, err
	}

	m.recordDataPages(now, globalStats, errors)
	m.recordDataUsage(now, globalStats, errors)

	for k, v := range globalStats {
		switch k {

		// buffer_pool.pages
		case "Innodb_buffer_pool_pages_data":
			m.mb.ParseMysqlBufferPoolPagesDataPoint(now, v, errors, m.logger, "data")
		case "Innodb_buffer_pool_pages_free":
			m.mb.ParseMysqlBufferPoolPagesDataPoint(now, v, errors, m.logger, "free")
		case "Innodb_buffer_pool_pages_misc":
			m.mb.ParseMysqlBufferPoolPagesDataPoint(now, v, errors, m.logger, "misc")

		// buffer_pool.page_flushes
		case "Innodb_buffer_pool_pages_flushed":
			m.mb.ParseMysqlBufferPoolPageFlushesDataPoint(now, v, errors, m.logger)

		// buffer_pool.operations
		case "Innodb_buffer_pool_read_ahead_rnd":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, m.logger, "read_ahead_rnd")
		case "Innodb_buffer_pool_read_ahead":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, m.logger, "read_ahead")
		case "Innodb_buffer_pool_read_ahead_evicted":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, m.logger, "read_ahead_evicted")
		case "Innodb_buffer_pool_read_requests":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, m.logger, "read_requests")
		case "Innodb_buffer_pool_reads":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, m.logger, "reads")
		case "Innodb_buffer_pool_wait_free":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, m.logger, "wait_free")
		case "Innodb_buffer_pool_write_requests":
			m.mb.ParseMysqlBufferPoolOperationsDataPoint(now, v, errors, m.logger, "write_requests")

		// commands
		case "Com_stmt_execute":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, m.logger, "execute")
		case "Com_stmt_close":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, m.logger, "close")
		case "Com_stmt_fetch":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, m.logger, "fetch")
		case "Com_stmt_prepare":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, m.logger, "prepare")
		case "Com_stmt_reset":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, m.logger, "reset")
		case "Com_stmt_send_long_data":
			m.mb.ParseMysqlCommandsDataPoint(now, v, errors, m.logger, "send_long_data")

		// handlers
		case "Handler_commit":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "commit")
		case "Handler_delete":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "delete")
		case "Handler_discover":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "discover")
		case "Handler_external_lock":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "lock")
		case "Handler_mrr_init":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "mrr_init")
		case "Handler_prepare":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "prepare")
		case "Handler_read_first":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "read_first")
		case "Handler_read_key":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "read_key")
		case "Handler_read_last":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "read_last")
		case "Handler_read_next":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "read_next")
		case "Handler_read_prev":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "read_prev")
		case "Handler_read_rnd":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "read_rnd")
		case "Handler_read_rnd_next":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "read_rnd_next")
		case "Handler_rollback":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "rollback")
		case "Handler_savepoint":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "savepoint")
		case "Handler_savepoint_rollback":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "savepoint_rollback")
		case "Handler_update":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "update")
		case "Handler_write":
			m.mb.ParseMysqlHandlersDataPoint(now, v, errors, m.logger, "write")

		// double_writes
		case "Innodb_dblwr_pages_written":
			m.mb.ParseMysqlDoubleWritesDataPoint(now, v, errors, m.logger, "written")
		case "Innodb_dblwr_writes":
			m.mb.ParseMysqlDoubleWritesDataPoint(now, v, errors, m.logger, "writes")

		// log_operations
		case "Innodb_log_waits":
			m.mb.ParseMysqlLogOperationsDataPoint(now, v, errors, m.logger, "waits")
		case "Innodb_log_write_requests":
			m.mb.ParseMysqlLogOperationsDataPoint(now, v, errors, m.logger, "requests")
		case "Innodb_log_writes":
			m.mb.ParseMysqlLogOperationsDataPoint(now, v, errors, m.logger, "writes")

		// operations
		case "Innodb_data_fsyncs":
			m.mb.ParseMysqlOperationsDataPoint(now, v, errors, m.logger, "fsyncs")
		case "Innodb_data_reads":
			m.mb.ParseMysqlOperationsDataPoint(now, v, errors, m.logger, "reads")
		case "Innodb_data_writes":
			m.mb.ParseMysqlOperationsDataPoint(now, v, errors, m.logger, "writes")

		// page_operations
		case "Innodb_pages_created":
			m.mb.ParseMysqlPageOperationsDataPoint(now, v, errors, m.logger, "created")
		case "Innodb_pages_read":
			m.mb.ParseMysqlPageOperationsDataPoint(now, v, errors, m.logger, "read")
		case "Innodb_pages_written":
			m.mb.ParseMysqlPageOperationsDataPoint(now, v, errors, m.logger, "written")

		// row_locks
		case "Innodb_row_lock_waits":
			m.mb.ParseMysqlRowLocksDataPoint(now, v, errors, m.logger, "waits")
		case "Innodb_row_lock_time":
			m.mb.ParseMysqlRowLocksDataPoint(now, v, errors, m.logger, "time")

		// row_operations
		case "Innodb_rows_deleted":
			m.mb.ParseMysqlRowOperationsDataPoint(now, v, errors, m.logger, "deleted")
		case "Innodb_rows_inserted":
			m.mb.ParseMysqlRowOperationsDataPoint(now, v, errors, m.logger, "inserted")
		case "Innodb_rows_read":
			m.mb.ParseMysqlRowOperationsDataPoint(now, v, errors, m.logger, "read")
		case "Innodb_rows_updated":
			m.mb.ParseMysqlRowOperationsDataPoint(now, v, errors, m.logger, "updated")

		// locks
		case "Table_locks_immediate":
			m.mb.ParseMysqlLocksDataPoint(now, v, errors, m.logger, "immediate")
		case "Table_locks_waited":
			m.mb.ParseMysqlLocksDataPoint(now, v, errors, m.logger, "waited")

		// sorts
		case "Sort_merge_passes":
			m.mb.ParseMysqlSortsDataPoint(now, v, errors, m.logger, "merge_passes")
		case "Sort_range":
			m.mb.ParseMysqlSortsDataPoint(now, v, errors, m.logger, "range")
		case "Sort_rows":
			m.mb.ParseMysqlSortsDataPoint(now, v, errors, m.logger, "rows")
		case "Sort_scan":
			m.mb.ParseMysqlSortsDataPoint(now, v, errors, m.logger, "scan")

		// threads
		case "Threads_cached":
			m.mb.ParseMysqlThreadsDataPoint(now, v, errors, m.logger, "cached")
		case "Threads_connected":
			m.mb.ParseMysqlThreadsDataPoint(now, v, errors, m.logger, "connected")
		case "Threads_created":
			m.mb.ParseMysqlThreadsDataPoint(now, v, errors, m.logger, "created")
		case "Threads_running":
			m.mb.ParseMysqlThreadsDataPoint(now, v, errors, m.logger, "running")
		}
	}

	return m.mb.Emit(), errors.Combine()
}

func (m *mySQLScraper) recordDataPages(now pdata.Timestamp, globalStats map[string]string, errors scrapererror.ScrapeErrors) {
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

func (m *mySQLScraper) recordDataUsage(now pdata.Timestamp, globalStats map[string]string, errors scrapererror.ScrapeErrors) {
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
