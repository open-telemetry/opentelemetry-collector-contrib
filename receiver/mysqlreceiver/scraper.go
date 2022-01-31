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

	// metric initialization
	md := pdata.NewMetrics()
	ilm := md.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otel/mysql")
	now := pdata.NewTimestampFromTime(time.Now())

	// collect innodb metrics.
	innodbStats, err := m.sqlclient.getInnodbStats()
	if err != nil {
		m.logger.Error("Failed to fetch InnoDB stats", zap.Error(err))
	}

	for k, v := range innodbStats {
		if k != "buffer_pool_size" {
			continue
		}
		if f, ok := m.parseFloat(k, v); ok {
			m.mb.RecordMysqlBufferPoolSizeDataPoint(now, f, "total")
		}
	}

	// collect global status metrics.
	globalStats, err := m.sqlclient.getGlobalStats()
	if err != nil {
		m.logger.Error("Failed to fetch global stats", zap.Error(err))
		return pdata.Metrics{}, err
	}

	for k, v := range globalStats {
		switch k {

		// buffer_pool_pages
		case "Innodb_buffer_pool_pages_data":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlBufferPoolPagesDataPoint(now, f, "data")
			}
		case "Innodb_buffer_pool_pages_dirty":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlBufferPoolPagesDataPoint(now, f, "dirty")
			}
		case "Innodb_buffer_pool_pages_flushed":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlBufferPoolPagesDataPoint(now, f, "flushed")
			}
		case "Innodb_buffer_pool_pages_free":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlBufferPoolPagesDataPoint(now, f, "free")
			}
		case "Innodb_buffer_pool_pages_misc":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlBufferPoolPagesDataPoint(now, f, "misc")
			}
		case "Innodb_buffer_pool_pages_total":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlBufferPoolPagesDataPoint(now, f, "total")
			}

		// buffer_pool_operations
		case "Innodb_buffer_pool_read_ahead_rnd":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "read_ahead_rnd")
			}
		case "Innodb_buffer_pool_read_ahead":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "read_ahead")
			}
		case "Innodb_buffer_pool_read_ahead_evicted":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "read_ahead_evicted")
			}
		case "Innodb_buffer_pool_read_requests":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "read_requests")
			}
		case "Innodb_buffer_pool_reads":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "reads")
			}
		case "Innodb_buffer_pool_wait_free":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "wait_free")
			}
		case "Innodb_buffer_pool_write_requests":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlBufferPoolOperationsDataPoint(now, i, "write_requests")
			}

		// buffer_pool_size
		case "Innodb_buffer_pool_bytes_data":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlBufferPoolSizeDataPoint(now, f, "data")
			}
		case "Innodb_buffer_pool_bytes_dirty":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlBufferPoolSizeDataPoint(now, f, "dirty")
			}

		// commands
		case "Com_stmt_execute":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "execute")
			}
		case "Com_stmt_close":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "close")
			}
		case "Com_stmt_fetch":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "fetch")
			}
		case "Com_stmt_prepare":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "prepare")
			}
		case "Com_stmt_reset":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "reset")
			}
		case "Com_stmt_send_long_data":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlCommandsDataPoint(now, i, "send_long_data")
			}

		// handlers
		case "Handler_commit":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "commit")
			}
		case "Handler_delete":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "delete")
			}
		case "Handler_discover":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "discover")
			}
		case "Handler_external_lock":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "lock")
			}
		case "Handler_mrr_init":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "mrr_init")
			}
		case "Handler_prepare":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "prepare")
			}
		case "Handler_read_first":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_first")
			}
		case "Handler_read_key":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_key")
			}
		case "Handler_read_last":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_last")
			}
		case "Handler_read_next":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_next")
			}
		case "Handler_read_prev":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_prev")
			}
		case "Handler_read_rnd":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_rnd")
			}
		case "Handler_read_rnd_next":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "read_rnd_next")
			}
		case "Handler_rollback":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "rollback")
			}
		case "Handler_savepoint":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "savepoint")
			}
		case "Handler_savepoint_rollback":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "savepoint_rollback")
			}
		case "Handler_update":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "update")
			}
		case "Handler_write":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlHandlersDataPoint(now, i, "write")
			}

		// double_writes
		case "Innodb_dblwr_pages_written":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlDoubleWritesDataPoint(now, i, "written")
			}
		case "Innodb_dblwr_writes":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlDoubleWritesDataPoint(now, i, "writes")
			}

		// log_operations
		case "Innodb_log_waits":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlLogOperationsDataPoint(now, i, "waits")
			}
		case "Innodb_log_write_requests":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlLogOperationsDataPoint(now, i, "requests")
			}
		case "Innodb_log_writes":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlLogOperationsDataPoint(now, i, "writes")
			}

		// operations
		case "Innodb_data_fsyncs":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlOperationsDataPoint(now, i, "fsyncs")
			}
		case "Innodb_data_reads":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlOperationsDataPoint(now, i, "reads")
			}
		case "Innodb_data_writes":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlOperationsDataPoint(now, i, "writes")
			}

		// page_operations
		case "Innodb_pages_created":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlPageOperationsDataPoint(now, i, "created")
			}
		case "Innodb_pages_read":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlPageOperationsDataPoint(now, i, "read")
			}
		case "Innodb_pages_written":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlPageOperationsDataPoint(now, i, "written")
			}

		// row_locks
		case "Innodb_row_lock_waits":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlRowLocksDataPoint(now, i, "waits")
			}
		case "Innodb_row_lock_time":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlRowLocksDataPoint(now, i, "time")
			}

		// row_operations
		case "Innodb_rows_deleted":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlRowOperationsDataPoint(now, i, "deleted")
			}
		case "Innodb_rows_inserted":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlRowOperationsDataPoint(now, i, "inserted")
			}
		case "Innodb_rows_read":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlRowOperationsDataPoint(now, i, "read")
			}
		case "Innodb_rows_updated":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlRowOperationsDataPoint(now, i, "updated")
			}

		// locks
		case "Table_locks_immediate":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlLocksDataPoint(now, i, "immediate")
			}
		case "Table_locks_waited":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlLocksDataPoint(now, i, "waited")
			}

		// sorts
		case "Sort_merge_passes":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlSortsDataPoint(now, i, "merge_passes")
			}
		case "Sort_range":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlSortsDataPoint(now, i, "range")
			}
		case "Sort_rows":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlSortsDataPoint(now, i, "rows")
			}
		case "Sort_scan":
			if i, ok := m.parseInt(k, v); ok {
				m.mb.RecordMysqlSortsDataPoint(now, i, "scan")
			}

		// threads
		case "Threads_cached":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlThreadsDataPoint(now, f, "cached")
			}
		case "Threads_connected":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlThreadsDataPoint(now, f, "connected")
			}
		case "Threads_created":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlThreadsDataPoint(now, f, "created")
			}
		case "Threads_running":
			if f, ok := m.parseFloat(k, v); ok {
				m.mb.RecordMysqlThreadsDataPoint(now, f, "running")
			}
		}
	}

	m.mb.Emit(ilm.Metrics())
	return md, nil
}

// parseFloat converts string to float64.
func (m *mySQLScraper) parseFloat(key, value string) (float64, bool) {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		m.logInvalid("float", key, value)
		return 0, false
	}
	return f, true
}

// parseInt converts string to int64.
func (m *mySQLScraper) parseInt(key, value string) (int64, bool) {
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		m.logInvalid("int", key, value)
		return 0, false
	}
	return i, true
}

func (m *mySQLScraper) logInvalid(expectedType, key, value string) {
	m.logger.Info(
		"invalid value",
		zap.String("expectedType", expectedType),
		zap.String("key", key),
		zap.String("value", value),
	)
}
