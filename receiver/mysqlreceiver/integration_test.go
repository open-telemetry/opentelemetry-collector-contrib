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

//go:build integration
// +build integration

package mysqlreceiver

import (
	"context"
	"fmt"
	"net"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
)

func TestMysqlIntegration(t *testing.T) {
	t.Run("Running mysql version 5.7", func(t *testing.T) {
		container := getContainer(t, containerRequest5_7)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Endpoint = net.JoinHostPort(hostname, "3307")
		cfg.Username = "otel"
		cfg.Password = "otel"

		consumer := new(consumertest.MetricsSink)
		settings := componenttest.NewNopReceiverCreateSettings()
		rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err, "failed creating metrics receiver")
		require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
		require.Eventuallyf(t, func() bool {
			return len(consumer.AllMetrics()) > 0
		}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")

		md := consumer.AllMetrics()[0]
		require.Equal(t, 1, md.ResourceMetrics().Len())
		ilms := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics()
		require.Equal(t, 1, ilms.Len())
		metrics := ilms.At(0).Metrics()
		require.NoError(t, rcvr.Shutdown(context.Background()))

		validateResult(t, metrics)
	})

	t.Run("Running mysql version 8.0", func(t *testing.T) {
		container := getContainer(t, containerRequest8_0)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Endpoint = net.JoinHostPort(hostname, "3306")
		cfg.Username = "otel"
		cfg.Password = "otel"

		consumer := new(consumertest.MetricsSink)
		settings := componenttest.NewNopReceiverCreateSettings()
		rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err, "failed creating metrics receiver")
		require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
		require.Eventuallyf(t, func() bool {
			return len(consumer.AllMetrics()) > 0
		}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")

		md := consumer.AllMetrics()[0]
		require.Equal(t, 1, md.ResourceMetrics().Len())
		ilms := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics()
		require.Equal(t, 1, ilms.Len())
		metrics := ilms.At(0).Metrics()
		require.NoError(t, rcvr.Shutdown(context.Background()))

		validateResult(t, metrics)
	})
}

var (
	containerRequest5_7 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    path.Join(".", "testdata", "integration"),
			Dockerfile: "Dockerfile.mysql.5_7",
		},
		ExposedPorts: []string{"3307:3306"},
		WaitingFor: wait.ForListeningPort("3306").
			WithStartupTimeout(2 * time.Minute),
	}
	containerRequest8_0 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    path.Join(".", "testdata", "integration"),
			Dockerfile: "Dockerfile.mysql.8_0",
		},
		ExposedPorts: []string{"3306:3306"},
		WaitingFor: wait.ForListeningPort("3306").
			WithStartupTimeout(2 * time.Minute),
	}
)

func getContainer(t *testing.T, req testcontainers.ContainerRequest) testcontainers.Container {
	require.NoError(t, req.Validate())
	container, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)

	code, err := container.Exec(context.Background(), []string{"/setup.sh"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	return container
}

func validateResult(t *testing.T, metrics pdata.MetricSlice) {
	require.Equal(t, len(metadata.M.Names()), metrics.Len())

	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		switch m.Name() {
		case metadata.M.MysqlBufferPoolPages.Name():
			dps := m.Gauge().DataPoints()
			require.Equal(t, 6, dps.Len())
			bufferPoolPagesMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.BufferPoolPages)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				bufferPoolPagesMetrics[label] = true
			}
			require.Equal(t, map[string]bool{
				"mysql.buffer_pool_pages :data":    true,
				"mysql.buffer_pool_pages :dirty":   true,
				"mysql.buffer_pool_pages :flushed": true,
				"mysql.buffer_pool_pages :free":    true,
				"mysql.buffer_pool_pages :misc":    true,
				"mysql.buffer_pool_pages :total":   true,
			}, bufferPoolPagesMetrics)
		case metadata.M.MysqlBufferPoolOperations.Name():
			dps := m.Sum().DataPoints()
			require.True(t, m.Sum().IsMonotonic())
			require.Equal(t, 7, dps.Len())
			bufferPoolOperationsMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.BufferPoolOperations)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				bufferPoolOperationsMetrics[label] = true
			}
			require.Equal(t, 7, len(bufferPoolOperationsMetrics))
			require.Equal(t, map[string]bool{
				"mysql.buffer_pool_operations :read_ahead":         true,
				"mysql.buffer_pool_operations :read_ahead_evicted": true,
				"mysql.buffer_pool_operations :read_ahead_rnd":     true,
				"mysql.buffer_pool_operations :read_requests":      true,
				"mysql.buffer_pool_operations :reads":              true,
				"mysql.buffer_pool_operations :wait_free":          true,
				"mysql.buffer_pool_operations :write_requests":     true,
			}, bufferPoolOperationsMetrics)
		case metadata.M.MysqlBufferPoolSize.Name():
			dps := m.Gauge().DataPoints()
			require.Equal(t, 3, dps.Len())
			bufferPoolSizeMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.BufferPoolSize)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				bufferPoolSizeMetrics[label] = true
			}
			require.Equal(t, 3, len(bufferPoolSizeMetrics))
			require.Equal(t, map[string]bool{
				"mysql.buffer_pool_size :data":  true,
				"mysql.buffer_pool_size :dirty": true,
				"mysql.buffer_pool_size :size":  true,
			}, bufferPoolSizeMetrics)
		case metadata.M.MysqlCommands.Name():
			dps := m.Sum().DataPoints()
			require.True(t, m.Sum().IsMonotonic())
			require.Equal(t, 6, dps.Len())
			commandsMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.Command)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				commandsMetrics[label] = true
			}
			require.Equal(t, 6, len(commandsMetrics))
			require.Equal(t, map[string]bool{
				"mysql.commands :close":          true,
				"mysql.commands :execute":        true,
				"mysql.commands :fetch":          true,
				"mysql.commands :prepare":        true,
				"mysql.commands :reset":          true,
				"mysql.commands :send_long_data": true,
			}, commandsMetrics)
		case metadata.M.MysqlHandlers.Name():
			dps := m.Sum().DataPoints()
			require.True(t, m.Sum().IsMonotonic())
			require.Equal(t, 18, dps.Len())
			handlersMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.Handler)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				handlersMetrics[label] = true
			}
			require.Equal(t, 18, len(handlersMetrics))
			require.Equal(t, map[string]bool{
				"mysql.handlers :commit":             true,
				"mysql.handlers :delete":             true,
				"mysql.handlers :discover":           true,
				"mysql.handlers :lock":               true,
				"mysql.handlers :mrr_init":           true,
				"mysql.handlers :prepare":            true,
				"mysql.handlers :read_first":         true,
				"mysql.handlers :read_key":           true,
				"mysql.handlers :read_last":          true,
				"mysql.handlers :read_next":          true,
				"mysql.handlers :read_prev":          true,
				"mysql.handlers :read_rnd":           true,
				"mysql.handlers :read_rnd_next":      true,
				"mysql.handlers :rollback":           true,
				"mysql.handlers :savepoint":          true,
				"mysql.handlers :savepoint_rollback": true,
				"mysql.handlers :update":             true,
				"mysql.handlers :write":              true,
			}, handlersMetrics)
		case metadata.M.MysqlDoubleWrites.Name():
			dps := m.Sum().DataPoints()
			require.True(t, m.Sum().IsMonotonic())
			require.Equal(t, 2, dps.Len())
			doubleWritesMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.DoubleWrites)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				doubleWritesMetrics[label] = true
			}
			require.Equal(t, 2, len(doubleWritesMetrics))
			require.Equal(t, map[string]bool{
				"mysql.double_writes :writes":  true,
				"mysql.double_writes :written": true,
			}, doubleWritesMetrics)
		case metadata.M.MysqlLogOperations.Name():
			dps := m.Sum().DataPoints()
			require.True(t, m.Sum().IsMonotonic())
			require.Equal(t, 3, dps.Len())
			logOperationsMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.LogOperations)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				logOperationsMetrics[label] = true
			}
			require.Equal(t, 3, len(logOperationsMetrics))
			require.Equal(t, map[string]bool{
				"mysql.log_operations :requests": true,
				"mysql.log_operations :waits":    true,
				"mysql.log_operations :writes":   true,
			}, logOperationsMetrics)
		case metadata.M.MysqlOperations.Name():
			dps := m.Sum().DataPoints()
			require.True(t, m.Sum().IsMonotonic())
			require.Equal(t, 3, dps.Len())
			operationsMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.Operations)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				operationsMetrics[label] = true
			}
			require.Equal(t, 3, len(operationsMetrics))
			require.Equal(t, map[string]bool{
				"mysql.operations :fsyncs": true,
				"mysql.operations :reads":  true,
				"mysql.operations :writes": true,
			}, operationsMetrics)
		case metadata.M.MysqlPageOperations.Name():
			dps := m.Sum().DataPoints()
			require.True(t, m.Sum().IsMonotonic())
			require.Equal(t, 3, dps.Len())
			pageOperationsMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.PageOperations)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				pageOperationsMetrics[label] = true
			}
			require.Equal(t, 3, len(pageOperationsMetrics))
			require.Equal(t, map[string]bool{
				"mysql.page_operations :created": true,
				"mysql.page_operations :read":    true,
				"mysql.page_operations :written": true,
			}, pageOperationsMetrics)
		case metadata.M.MysqlRowLocks.Name():
			dps := m.Sum().DataPoints()
			require.True(t, m.Sum().IsMonotonic())
			require.Equal(t, 2, dps.Len())
			rowLocksMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.RowLocks)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				rowLocksMetrics[label] = true
			}
			require.Equal(t, 2, len(rowLocksMetrics))
			require.Equal(t, map[string]bool{
				"mysql.row_locks :time":  true,
				"mysql.row_locks :waits": true,
			}, rowLocksMetrics)
		case metadata.M.MysqlRowOperations.Name():
			dps := m.Sum().DataPoints()
			require.True(t, m.Sum().IsMonotonic())
			require.Equal(t, 4, dps.Len())
			rowOperationsMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.RowOperations)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				rowOperationsMetrics[label] = true
			}
			require.Equal(t, 4, len(rowOperationsMetrics))
			require.Equal(t, map[string]bool{
				"mysql.row_operations :deleted":  true,
				"mysql.row_operations :inserted": true,
				"mysql.row_operations :read":     true,
				"mysql.row_operations :updated":  true,
			}, rowOperationsMetrics)
		case metadata.M.MysqlLocks.Name():
			dps := m.Sum().DataPoints()
			require.True(t, m.Sum().IsMonotonic())
			require.Equal(t, 2, dps.Len())
			locksMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.Locks)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				locksMetrics[label] = true
			}
			require.Equal(t, 2, len(locksMetrics))
			require.Equal(t, map[string]bool{
				"mysql.locks :immediate": true,
				"mysql.locks :waited":    true,
			}, locksMetrics)
		case metadata.M.MysqlSorts.Name():
			dps := m.Sum().DataPoints()
			require.True(t, m.Sum().IsMonotonic())
			require.Equal(t, 4, dps.Len())
			sortsMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.Sorts)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				sortsMetrics[label] = true
			}
			require.Equal(t, 4, len(sortsMetrics))
			require.Equal(t, map[string]bool{
				"mysql.sorts :merge_passes": true,
				"mysql.sorts :range":        true,
				"mysql.sorts :rows":         true,
				"mysql.sorts :scan":         true,
			}, sortsMetrics)
		case metadata.M.MysqlThreads.Name():
			dps := m.Gauge().DataPoints()
			require.Equal(t, 4, dps.Len())
			threadsMetrics := map[string]bool{}
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				value_label, _ := dp.Attributes().Get(metadata.L.Threads)
				label := fmt.Sprintf("%s :%s", m.Name(), value_label.AsString())
				threadsMetrics[label] = true
			}
			require.Equal(t, 4, len(threadsMetrics))
			require.Equal(t, map[string]bool{
				"mysql.threads :cached":    true,
				"mysql.threads :connected": true,
				"mysql.threads :created":   true,
				"mysql.threads :running":   true,
			}, threadsMetrics)
		}
	}
}

func TestMySQLStartStop(t *testing.T) {
	container := getContainer(t, containerRequest8_0)
	defer func() {
		require.NoError(t, container.Terminate(context.Background()))
	}()
	hostname, err := container.Host(context.Background())
	require.NoError(t, err)

	sc := newMySQLScraper(zap.NewNop(), &Config{
		Username: "otel",
		Password: "otel",
		Endpoint: fmt.Sprintf("%s:3306", hostname),
	})

	// scraper is connected
	err = sc.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// scraper is closed
	err = sc.shutdown(context.Background())
	require.NoError(t, err)

	// scraper scapes and produces an error because db is closed
	_, err = sc.scrape(context.Background())
	require.Error(t, err)
	require.EqualError(t, err, "sql: database is closed")
}
