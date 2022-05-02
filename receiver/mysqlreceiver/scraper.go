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
		addPartialIfError(errors, m.mb.RecordMysqlBufferPoolLimitDataPoint(now, v))
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
			addPartialIfError(errors, m.mb.RecordMysqlBufferPoolPagesDataPoint(now, v, "data"))
		case "Innodb_buffer_pool_pages_free":
			addPartialIfError(errors, m.mb.RecordMysqlBufferPoolPagesDataPoint(now, v, "free"))
		case "Innodb_buffer_pool_pages_misc":
			addPartialIfError(errors, m.mb.RecordMysqlBufferPoolPagesDataPoint(now, v, "misc"))

		// buffer_pool.page_flushes
		case "Innodb_buffer_pool_pages_flushed":
			addPartialIfError(errors, m.mb.RecordMysqlBufferPoolPageFlushesDataPoint(now, v))

		// buffer_pool.operations
		case "Innodb_buffer_pool_read_ahead_rnd":
			addPartialIfError(errors, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v, "read_ahead_rnd"))
		case "Innodb_buffer_pool_read_ahead":
			addPartialIfError(errors, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v, "read_ahead"))
		case "Innodb_buffer_pool_read_ahead_evicted":
			addPartialIfError(errors, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v, "read_ahead_evicted"))
		case "Innodb_buffer_pool_read_requests":
			addPartialIfError(errors, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v, "read_requests"))
		case "Innodb_buffer_pool_reads":
			addPartialIfError(errors, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v, "reads"))
		case "Innodb_buffer_pool_wait_free":
			addPartialIfError(errors, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v, "wait_free"))
		case "Innodb_buffer_pool_write_requests":
			addPartialIfError(errors, m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, v, "write_requests"))

		// commands
		case "Com_stmt_execute":
			addPartialIfError(errors, m.mb.RecordMysqlCommandsDataPoint(now, v, "execute"))
		case "Com_stmt_close":
			addPartialIfError(errors, m.mb.RecordMysqlCommandsDataPoint(now, v, "close"))
		case "Com_stmt_fetch":
			addPartialIfError(errors, m.mb.RecordMysqlCommandsDataPoint(now, v, "fetch"))
		case "Com_stmt_prepare":
			addPartialIfError(errors, m.mb.RecordMysqlCommandsDataPoint(now, v, "prepare"))
		case "Com_stmt_reset":
			addPartialIfError(errors, m.mb.RecordMysqlCommandsDataPoint(now, v, "reset"))
		case "Com_stmt_send_long_data":
			addPartialIfError(errors, m.mb.RecordMysqlCommandsDataPoint(now, v, "send_long_data"))

		// handlers
		case "Handler_commit":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "commit"))
		case "Handler_delete":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "delete"))
		case "Handler_discover":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "discover"))
		case "Handler_external_lock":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "external_lock"))
		case "Handler_mrr_init":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "mrr_init"))
		case "Handler_prepare":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "prepare"))
		case "Handler_read_first":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "read_first"))
		case "Handler_read_key":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "read_key"))
		case "Handler_read_last":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "read_last"))
		case "Handler_read_next":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "read_next"))
		case "Handler_read_prev":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "read_prev"))
		case "Handler_read_rnd":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "read_rnd"))
		case "Handler_read_rnd_next":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "read_rnd_next"))
		case "Handler_rollback":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "rollback"))
		case "Handler_savepoint":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "savepoint"))
		case "Handler_savepoint_rollback":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "savepoint_rollback"))
		case "Handler_update":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "update"))
		case "Handler_write":
			addPartialIfError(errors, m.mb.RecordMysqlHandlersDataPoint(now, v, "write"))

		// double_writes
		case "Innodb_dblwr_pages_written":
			addPartialIfError(errors, m.mb.RecordMysqlDoubleWritesDataPoint(now, v, "pages_written"))
		case "Innodb_dblwr_writes":
			addPartialIfError(errors, m.mb.RecordMysqlDoubleWritesDataPoint(now, v, "writes"))

		// log_operations
		case "Innodb_log_waits":
			addPartialIfError(errors, m.mb.RecordMysqlLogOperationsDataPoint(now, v, "waits"))
		case "Innodb_log_write_requests":
			addPartialIfError(errors, m.mb.RecordMysqlLogOperationsDataPoint(now, v, "write_requests"))
		case "Innodb_log_writes":
			addPartialIfError(errors, m.mb.RecordMysqlLogOperationsDataPoint(now, v, "writes"))

		// operations
		case "Innodb_data_fsyncs":
			addPartialIfError(errors, m.mb.RecordMysqlOperationsDataPoint(now, v, "fsyncs"))
		case "Innodb_data_reads":
			addPartialIfError(errors, m.mb.RecordMysqlOperationsDataPoint(now, v, "reads"))
		case "Innodb_data_writes":
			addPartialIfError(errors, m.mb.RecordMysqlOperationsDataPoint(now, v, "writes"))

		// page_operations
		case "Innodb_pages_created":
			addPartialIfError(errors, m.mb.RecordMysqlPageOperationsDataPoint(now, v, "created"))
		case "Innodb_pages_read":
			addPartialIfError(errors, m.mb.RecordMysqlPageOperationsDataPoint(now, v, "read"))
		case "Innodb_pages_written":
			addPartialIfError(errors, m.mb.RecordMysqlPageOperationsDataPoint(now, v, "written"))

		// row_locks
		case "Innodb_row_lock_waits":
			addPartialIfError(errors, m.mb.RecordMysqlRowLocksDataPoint(now, v, "waits"))
		case "Innodb_row_lock_time":
			addPartialIfError(errors, m.mb.RecordMysqlRowLocksDataPoint(now, v, "time"))

		// row_operations
		case "Innodb_rows_deleted":
			addPartialIfError(errors, m.mb.RecordMysqlRowOperationsDataPoint(now, v, "deleted"))
		case "Innodb_rows_inserted":
			addPartialIfError(errors, m.mb.RecordMysqlRowOperationsDataPoint(now, v, "inserted"))
		case "Innodb_rows_read":
			addPartialIfError(errors, m.mb.RecordMysqlRowOperationsDataPoint(now, v, "read"))
		case "Innodb_rows_updated":
			addPartialIfError(errors, m.mb.RecordMysqlRowOperationsDataPoint(now, v, "updated"))

		// locks
		case "Table_locks_immediate":
			addPartialIfError(errors, m.mb.RecordMysqlLocksDataPoint(now, v, "immediate"))
		case "Table_locks_waited":
			addPartialIfError(errors, m.mb.RecordMysqlLocksDataPoint(now, v, "waited"))

		// sorts
		case "Sort_merge_passes":
			addPartialIfError(errors, m.mb.RecordMysqlSortsDataPoint(now, v, "merge_passes"))
		case "Sort_range":
			addPartialIfError(errors, m.mb.RecordMysqlSortsDataPoint(now, v, "range"))
		case "Sort_rows":
			addPartialIfError(errors, m.mb.RecordMysqlSortsDataPoint(now, v, "rows"))
		case "Sort_scan":
			addPartialIfError(errors, m.mb.RecordMysqlSortsDataPoint(now, v, "scan"))

		// threads
		case "Threads_cached":
			addPartialIfError(errors, m.mb.RecordMysqlThreadsDataPoint(now, v, "cached"))
		case "Threads_connected":
			addPartialIfError(errors, m.mb.RecordMysqlThreadsDataPoint(now, v, "connected"))
		case "Threads_created":
			addPartialIfError(errors, m.mb.RecordMysqlThreadsDataPoint(now, v, "created"))
		case "Threads_running":
			addPartialIfError(errors, m.mb.RecordMysqlThreadsDataPoint(now, v, "running"))
		}
	}

	return m.mb.Emit(), errors.Combine()
}

func addPartialIfError(errors scrapererror.ScrapeErrors, err error) {
	if err != nil {
		errors.AddPartial(1, err)
	}
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
