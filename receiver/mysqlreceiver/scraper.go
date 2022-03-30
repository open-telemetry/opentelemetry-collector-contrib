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
		if i, err := parseInt(v); err != nil {
			errors.AddPartial(1, err)
		} else {
			m.mb.RecordMysqlBufferPoolLimitDataPoint(now, i)
		}
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
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlBufferPoolPagesDataPoint(now, i, "data")
			}
		case "Innodb_buffer_pool_pages_free":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlBufferPoolPagesDataPoint(now, i, "free")
			}
		case "Innodb_buffer_pool_pages_misc":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlBufferPoolPagesDataPoint(now, i, "misc")
			}

		// buffer_pool.page_flushes
		case "Innodb_buffer_pool_pages_flushed":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlBufferPoolPageFlushesDataPoint(now, i)
			}

		// buffer_pool.operations
		case "Innodb_buffer_pool_read_ahead_rnd":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "read_ahead_rnd")
			}
		case "Innodb_buffer_pool_read_ahead":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "read_ahead")
			}
		case "Innodb_buffer_pool_read_ahead_evicted":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "read_ahead_evicted")
			}
		case "Innodb_buffer_pool_read_requests":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "read_requests")
			}
		case "Innodb_buffer_pool_reads":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "reads")
			}
		case "Innodb_buffer_pool_wait_free":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "wait_free")
			}
		case "Innodb_buffer_pool_write_requests":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "write_requests")
			}

		// commands
		case "Com_stmt_execute":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "execute")
			}
		case "Com_stmt_close":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "close")
			}
		case "Com_stmt_fetch":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "fetch")
			}
		case "Com_stmt_prepare":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "prepare")
			}
		case "Com_stmt_reset":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "reset")
			}
		case "Com_stmt_send_long_data":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "send_long_data")
			}

		// handlers
		case "Handler_commit":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "commit")
			}
		case "Handler_delete":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "delete")
			}
		case "Handler_discover":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "discover")
			}
		case "Handler_external_lock":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "lock")
			}
		case "Handler_mrr_init":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "mrr_init")
			}
		case "Handler_prepare":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "prepare")
			}
		case "Handler_read_first":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_first")
			}
		case "Handler_read_key":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_key")
			}
		case "Handler_read_last":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_last")
			}
		case "Handler_read_next":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_next")
			}
		case "Handler_read_prev":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_prev")
			}
		case "Handler_read_rnd":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_rnd")
			}
		case "Handler_read_rnd_next":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_rnd_next")
			}
		case "Handler_rollback":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "rollback")
			}
		case "Handler_savepoint":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "savepoint")
			}
		case "Handler_savepoint_rollback":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "savepoint_rollback")
			}
		case "Handler_update":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "update")
			}
		case "Handler_write":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "write")
			}

		// double_writes
		case "Innodb_dblwr_pages_written":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlDoubleWritesDataPoint(now, i, "written")
			}
		case "Innodb_dblwr_writes":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlDoubleWritesDataPoint(now, i, "writes")
			}

		// log_operations
		case "Innodb_log_waits":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlLogOperationsDataPoint(now, i, "waits")
			}
		case "Innodb_log_write_requests":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlLogOperationsDataPoint(now, i, "requests")
			}
		case "Innodb_log_writes":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlLogOperationsDataPoint(now, i, "writes")
			}

		// operations
		case "Innodb_data_fsyncs":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlOperationsDataPoint(now, i, "fsyncs")
			}
		case "Innodb_data_reads":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlOperationsDataPoint(now, i, "reads")
			}
		case "Innodb_data_writes":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlOperationsDataPoint(now, i, "writes")
			}

		// page_operations
		case "Innodb_pages_created":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlPageOperationsDataPoint(now, i, "created")
			}
		case "Innodb_pages_read":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlPageOperationsDataPoint(now, i, "read")
			}
		case "Innodb_pages_written":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlPageOperationsDataPoint(now, i, "written")
			}

		// row_locks
		case "Innodb_row_lock_waits":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlRowLocksDataPoint(now, i, "waits")
			}
		case "Innodb_row_lock_time":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlRowLocksDataPoint(now, i, "time")
			}

		// row_operations
		case "Innodb_rows_deleted":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlRowOperationsDataPoint(now, i, "deleted")
			}
		case "Innodb_rows_inserted":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlRowOperationsDataPoint(now, i, "inserted")
			}
		case "Innodb_rows_read":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlRowOperationsDataPoint(now, i, "read")
			}
		case "Innodb_rows_updated":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlRowOperationsDataPoint(now, i, "updated")
			}

		// locks
		case "Table_locks_immediate":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlLocksDataPoint(now, i, "immediate")
			}
		case "Table_locks_waited":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlLocksDataPoint(now, i, "waited")
			}

		// sorts
		case "Sort_merge_passes":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlSortsDataPoint(now, i, "merge_passes")
			}
		case "Sort_range":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlSortsDataPoint(now, i, "range")
			}
		case "Sort_rows":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlSortsDataPoint(now, i, "rows")
			}
		case "Sort_scan":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlSortsDataPoint(now, i, "scan")
			}

		// threads
		case "Threads_cached":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlThreadsDataPoint(now, i, "cached")
			}
		case "Threads_connected":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlThreadsDataPoint(now, i, "connected")
			}
		case "Threads_created":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlThreadsDataPoint(now, i, "created")
			}
		case "Threads_running":
			if i, err := parseInt(v); err != nil {
				errors.AddPartial(1, err)
			} else {
				m.mb.RecordMysqlThreadsDataPoint(now, i, "running")
			}
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
