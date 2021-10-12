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

package mysqlreceiver

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
)

type mySQLScraper struct {
	client   client
	stopOnce sync.Once

	logger *zap.Logger
	config *Config
}

func newMySQLScraper(
	logger *zap.Logger,
	config *Config,
) *mySQLScraper {
	return &mySQLScraper{
		logger: logger,
		config: config,
	}
}

// start starts the scraper by initializing the db client connection.
func (m *mySQLScraper) start(_ context.Context, host component.Host) error {
	client, err := newMySQLClient(mySQLConfig{
		username: m.config.Username,
		password: m.config.Password,
		database: m.config.Database,
		endpoint: m.config.Endpoint,
	})
	if err != nil {
		return err
	}
	m.client = client

	return nil
}

// shutdown closes the db connection
func (m *mySQLScraper) shutdown(context.Context) error {
	var err error
	m.stopOnce.Do(func() {
		err = m.client.Close()
	})
	return err
}

// initMetric initializes a metric with a metadata label.
func initMetric(ms pdata.MetricSlice, mi metadata.MetricIntf) pdata.Metric {
	m := ms.AppendEmpty()
	mi.Init(m)
	return m
}

// addToDoubleMetric adds and labels a double gauge datapoint to a metricslice.
func addToDoubleMetric(metric pdata.NumberDataPointSlice, labels pdata.AttributeMap, value float64, ts pdata.Timestamp) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(ts)
	dataPoint.SetDoubleVal(value)
	if labels.Len() > 0 {
		labels.CopyTo(dataPoint.Attributes())
	}
}

// addToIntMetric adds and labels a int sum datapoint to metricslice.
func addToIntMetric(metric pdata.NumberDataPointSlice, labels pdata.AttributeMap, value int64, ts pdata.Timestamp) {
	dataPoint := metric.AppendEmpty()
	dataPoint.SetTimestamp(ts)
	dataPoint.SetIntVal(value)
	if labels.Len() > 0 {
		labels.CopyTo(dataPoint.Attributes())
	}
}

// scrape scrapes the mysql db metric stats, transforms them and labels them into a metric slices.
func (m *mySQLScraper) scrape(context.Context) (pdata.ResourceMetricsSlice, error) {

	if m.client == nil {
		return pdata.ResourceMetricsSlice{}, errors.New("failed to connect to http client")
	}

	// metric initialization
	rms := pdata.NewResourceMetricsSlice()
	ilm := rms.AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName("otel/mysql")
	now := pdata.NewTimestampFromTime(time.Now())

	bufferPoolPages := initMetric(ilm.Metrics(), metadata.M.MysqlBufferPoolPages).Sum().DataPoints()
	bufferPoolOperations := initMetric(ilm.Metrics(), metadata.M.MysqlBufferPoolOperations).Sum().DataPoints()
	bufferPoolSize := initMetric(ilm.Metrics(), metadata.M.MysqlBufferPoolSize).Sum().DataPoints()
	commands := initMetric(ilm.Metrics(), metadata.M.MysqlCommands).Sum().DataPoints()
	handlers := initMetric(ilm.Metrics(), metadata.M.MysqlHandlers).Sum().DataPoints()
	doubleWrites := initMetric(ilm.Metrics(), metadata.M.MysqlDoubleWrites).Sum().DataPoints()
	logOperations := initMetric(ilm.Metrics(), metadata.M.MysqlLogOperations).Sum().DataPoints()
	operations := initMetric(ilm.Metrics(), metadata.M.MysqlOperations).Sum().DataPoints()
	pageOperations := initMetric(ilm.Metrics(), metadata.M.MysqlPageOperations).Sum().DataPoints()
	rowLocks := initMetric(ilm.Metrics(), metadata.M.MysqlRowLocks).Sum().DataPoints()
	rowOperations := initMetric(ilm.Metrics(), metadata.M.MysqlRowOperations).Sum().DataPoints()
	locks := initMetric(ilm.Metrics(), metadata.M.MysqlLocks).Sum().DataPoints()
	sorts := initMetric(ilm.Metrics(), metadata.M.MysqlSorts).Sum().DataPoints()
	threads := initMetric(ilm.Metrics(), metadata.M.MysqlThreads).Sum().DataPoints()

	// collect innodb metrics.
	innodbStats, err := m.client.getInnodbStats()
	if err != nil {
		m.logger.Error("Failed to fetch InnoDB stats", zap.Error(err))
	}

	for k, v := range innodbStats {
		if k != "buffer_pool_size" {
			continue
		}
		labels := pdata.NewAttributeMap()
		if f, ok := m.parseFloat(k, v); ok {
			labels.Insert(metadata.L.BufferPoolSize, pdata.NewAttributeValueString("total"))
			addToDoubleMetric(bufferPoolSize, labels, f, now)
		}
	}

	// collect global status metrics.
	globalStats, err := m.client.getGlobalStats()
	if err != nil {
		m.logger.Error("Failed to fetch global stats", zap.Error(err))
		return pdata.ResourceMetricsSlice{}, err
	}

	for k, v := range globalStats {
		labels := pdata.NewAttributeMap()
		switch k {

		// buffer_pool_pages
		case "Innodb_buffer_pool_pages_data":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.BufferPoolPages, pdata.NewAttributeValueString("data"))
				addToDoubleMetric(bufferPoolPages, labels, f, now)
			}
		case "Innodb_buffer_pool_pages_dirty":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.BufferPoolPages, pdata.NewAttributeValueString("dirty"))
				addToDoubleMetric(bufferPoolPages, labels, f, now)
			}
		case "Innodb_buffer_pool_pages_flushed":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.BufferPoolPages, pdata.NewAttributeValueString("flushed"))
				addToDoubleMetric(bufferPoolPages, labels, f, now)
			}
		case "Innodb_buffer_pool_pages_free":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.BufferPoolPages, pdata.NewAttributeValueString("free"))
				addToDoubleMetric(bufferPoolPages, labels, f, now)
			}
		case "Innodb_buffer_pool_pages_misc":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.BufferPoolPages, pdata.NewAttributeValueString("misc"))
				addToDoubleMetric(bufferPoolPages, labels, f, now)
			}
		case "Innodb_buffer_pool_pages_total":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.BufferPoolPages, pdata.NewAttributeValueString("total"))
				addToDoubleMetric(bufferPoolPages, labels, f, now)
			}

		// buffer_pool_operations
		case "Innodb_buffer_pool_read_ahead_rnd":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.BufferPoolOperations, pdata.NewAttributeValueString("read_ahead_rnd"))
				addToIntMetric(bufferPoolOperations, labels, i, now)
			}
		case "Innodb_buffer_pool_read_ahead":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.BufferPoolOperations, pdata.NewAttributeValueString("read_ahead"))
				addToIntMetric(bufferPoolOperations, labels, i, now)
			}
		case "Innodb_buffer_pool_read_ahead_evicted":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.BufferPoolOperations, pdata.NewAttributeValueString("read_ahead_evicted"))
				addToIntMetric(bufferPoolOperations, labels, i, now)
			}
		case "Innodb_buffer_pool_read_requests":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.BufferPoolOperations, pdata.NewAttributeValueString("read_requests"))
				addToIntMetric(bufferPoolOperations, labels, i, now)
			}
		case "Innodb_buffer_pool_reads":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.BufferPoolOperations, pdata.NewAttributeValueString("reads"))
				addToIntMetric(bufferPoolOperations, labels, i, now)
			}
		case "Innodb_buffer_pool_wait_free":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.BufferPoolOperations, pdata.NewAttributeValueString("wait_free"))
				addToIntMetric(bufferPoolOperations, labels, i, now)
			}
		case "Innodb_buffer_pool_write_requests":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.BufferPoolOperations, pdata.NewAttributeValueString("write_requests"))
				addToIntMetric(bufferPoolOperations, labels, i, now)
			}

		// buffer_pool_size
		case "Innodb_buffer_pool_bytes_data":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.BufferPoolSize, pdata.NewAttributeValueString("data"))
				addToDoubleMetric(bufferPoolSize, labels, f, now)
			}
		case "Innodb_buffer_pool_bytes_dirty":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.BufferPoolSize, pdata.NewAttributeValueString("dirty"))
				addToDoubleMetric(bufferPoolSize, labels, f, now)
			}

		// commands
		case "Com_stmt_execute":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Command, pdata.NewAttributeValueString("execute"))
				addToIntMetric(commands, labels, i, now)
			}
		case "Com_stmt_close":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Command, pdata.NewAttributeValueString("close"))
				addToIntMetric(commands, labels, i, now)
			}
		case "Com_stmt_fetch":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Command, pdata.NewAttributeValueString("fetch"))
				addToIntMetric(commands, labels, i, now)
			}
		case "Com_stmt_prepare":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Command, pdata.NewAttributeValueString("prepare"))
				addToIntMetric(commands, labels, i, now)
			}
		case "Com_stmt_reset":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Command, pdata.NewAttributeValueString("reset"))
				addToIntMetric(commands, labels, i, now)
			}
		case "Com_stmt_send_long_data":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Command, pdata.NewAttributeValueString("send_long_data"))
				addToIntMetric(commands, labels, i, now)
			}

		// handlers
		case "Handler_commit":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("commit"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_delete":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("delete"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_discover":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("discover"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_external_lock":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("lock"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_mrr_init":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("mrr_init"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_prepare":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("prepare"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_read_first":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("read_first"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_read_key":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("read_key"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_read_last":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("read_last"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_read_next":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("read_next"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_read_prev":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("read_prev"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_read_rnd":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("read_rnd"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_read_rnd_next":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("read_rnd_next"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_rollback":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("rollback"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_savepoint":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("savepoint"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_savepoint_rollback":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("savepoint_rollback"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_update":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("update"))
				addToIntMetric(handlers, labels, i, now)
			}
		case "Handler_write":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Handler, pdata.NewAttributeValueString("write"))
				addToIntMetric(handlers, labels, i, now)
			}

		// double_writes
		case "Innodb_dblwr_pages_written":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.DoubleWrites, pdata.NewAttributeValueString("written"))
				addToIntMetric(doubleWrites, labels, i, now)
			}
		case "Innodb_dblwr_writes":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.DoubleWrites, pdata.NewAttributeValueString("writes"))
				addToIntMetric(doubleWrites, labels, i, now)
			}

		// log_operations
		case "Innodb_log_waits":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.LogOperations, pdata.NewAttributeValueString("waits"))
				addToIntMetric(logOperations, labels, i, now)
			}
		case "Innodb_log_write_requests":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.LogOperations, pdata.NewAttributeValueString("requests"))
				addToIntMetric(logOperations, labels, i, now)
			}
		case "Innodb_log_writes":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.LogOperations, pdata.NewAttributeValueString("writes"))
				addToIntMetric(logOperations, labels, i, now)
			}

		// operations
		case "Innodb_data_fsyncs":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Operations, pdata.NewAttributeValueString("fsyncs"))
				addToIntMetric(operations, labels, i, now)
			}
		case "Innodb_data_reads":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Operations, pdata.NewAttributeValueString("reads"))
				addToIntMetric(operations, labels, i, now)
			}
		case "Innodb_data_writes":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Operations, pdata.NewAttributeValueString("writes"))
				addToIntMetric(operations, labels, i, now)
			}

		// page_operations
		case "Innodb_pages_created":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.PageOperations, pdata.NewAttributeValueString("created"))
				addToIntMetric(pageOperations, labels, i, now)
			}
		case "Innodb_pages_read":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.PageOperations, pdata.NewAttributeValueString("read"))
				addToIntMetric(pageOperations, labels, i, now)
			}
		case "Innodb_pages_written":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.PageOperations, pdata.NewAttributeValueString("written"))
				addToIntMetric(pageOperations, labels, i, now)
			}

		// row_locks
		case "Innodb_row_lock_waits":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.RowLocks, pdata.NewAttributeValueString("waits"))
				addToIntMetric(rowLocks, labels, i, now)
			}
		case "Innodb_row_lock_time":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.RowLocks, pdata.NewAttributeValueString("time"))
				addToIntMetric(rowLocks, labels, i, now)
			}

		// row_operations
		case "Innodb_rows_deleted":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.RowOperations, pdata.NewAttributeValueString("deleted"))
				addToIntMetric(rowOperations, labels, i, now)
			}
		case "Innodb_rows_inserted":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.RowOperations, pdata.NewAttributeValueString("inserted"))
				addToIntMetric(rowOperations, labels, i, now)
			}
		case "Innodb_rows_read":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.RowOperations, pdata.NewAttributeValueString("read"))
				addToIntMetric(rowOperations, labels, i, now)
			}
		case "Innodb_rows_updated":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.RowOperations, pdata.NewAttributeValueString("updated"))
				addToIntMetric(rowOperations, labels, i, now)
			}

		// locks
		case "Table_locks_immediate":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Locks, pdata.NewAttributeValueString("immediate"))
				addToIntMetric(locks, labels, i, now)
			}
		case "Table_locks_waited":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Locks, pdata.NewAttributeValueString("waited"))
				addToIntMetric(locks, labels, i, now)
			}

		// sorts
		case "Sort_merge_passes":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Sorts, pdata.NewAttributeValueString("merge_passes"))
				addToIntMetric(sorts, labels, i, now)
			}
		case "Sort_range":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Sorts, pdata.NewAttributeValueString("range"))
				addToIntMetric(sorts, labels, i, now)
			}
		case "Sort_rows":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Sorts, pdata.NewAttributeValueString("rows"))
				addToIntMetric(sorts, labels, i, now)
			}
		case "Sort_scan":
			if i, ok := m.parseInt(k, v); ok {
				labels.Insert(metadata.L.Sorts, pdata.NewAttributeValueString("scan"))
				addToIntMetric(sorts, labels, i, now)
			}

		// threads
		case "Threads_cached":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.Threads, pdata.NewAttributeValueString("cached"))
				addToDoubleMetric(threads, labels, f, now)
			}
		case "Threads_connected":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.Threads, pdata.NewAttributeValueString("connected"))
				addToDoubleMetric(threads, labels, f, now)
			}
		case "Threads_created":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.Threads, pdata.NewAttributeValueString("created"))
				addToDoubleMetric(threads, labels, f, now)
			}
		case "Threads_running":
			if f, ok := m.parseFloat(k, v); ok {
				labels.Insert(metadata.L.Threads, pdata.NewAttributeValueString("running"))
				addToDoubleMetric(threads, labels, f, now)
			}
		}
	}
	return rms, nil
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
