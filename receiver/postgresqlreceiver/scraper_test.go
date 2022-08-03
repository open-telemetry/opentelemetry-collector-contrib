// Copyright  The OpenTelemetry Authors
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

package postgresqlreceiver

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

func TestUnsuccessfulScrape(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "fake:11111"

	scraper := newPostgreSQLScraper(componenttest.NewNopReceiverCreateSettings(), cfg, &defaultClientFactory{})
	actualMetrics, err := scraper.scrape(context.Background())
	require.Error(t, err)

	require.NoError(t, scrapertest.CompareMetrics(pmetric.NewMetrics(), actualMetrics))
}

func TestScraper(t *testing.T) {
	factory := new(mockClientFactory)
	factory.initMocks([]string{"otel"})

	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{"otel"}
	scraper := newPostgreSQLScraper(componenttest.NewNopReceiverCreateSettings(), cfg, factory)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "otel", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
}

func TestScraperNoDatabaseSingle(t *testing.T) {
	factory := new(mockClientFactory)
	factory.initMocks([]string{"otel"})

	cfg := createDefaultConfig().(*Config)
	scraper := newPostgreSQLScraper(componenttest.NewNopReceiverCreateSettings(), cfg, factory)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "otel", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
}

func TestScraperNoDatabaseMultiple(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel", "open", "telemetry"})

	cfg := createDefaultConfig().(*Config)
	scraper := newPostgreSQLScraper(componenttest.NewNopReceiverCreateSettings(), cfg, &factory)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "multiple", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
}

type mockClientFactory struct{ mock.Mock }
type mockClient struct{ mock.Mock }

var _ client = &mockClient{}

func (m *mockClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockClient) getCommitsAndRollbacks(_ context.Context, databases []string) ([]MetricStat, error) {
	args := m.Called(databases)
	return args.Get(0).([]MetricStat), args.Error(1)
}

func (m *mockClient) getBackends(_ context.Context, databases []string) ([]MetricStat, error) {
	args := m.Called(databases)
	return args.Get(0).([]MetricStat), args.Error(1)
}

func (m *mockClient) getDatabaseSize(_ context.Context, databases []string) ([]MetricStat, error) {
	args := m.Called(databases)
	return args.Get(0).([]MetricStat), args.Error(1)
}

func (m *mockClient) getDatabaseTableMetrics(_ context.Context) ([]TableMetrics, error) {
	args := m.Called()
	return args.Get(0).([]TableMetrics), args.Error(1)
}

func (m *mockClient) getBlocksReadByTable(_ context.Context) ([]MetricStat, error) {
	args := m.Called()
	return args.Get(0).([]MetricStat), args.Error(1)
}

func (m *mockClient) listDatabases(_ context.Context) ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockClient) getVersion(ctx context.Context) (*version.Version, error) {
	args := m.Called(ctx)
	return args.Get(0).(*version.Version), args.Error(1)
}

func (m *mockClient) getBackgroundWriterStats(ctx context.Context) ([]MetricStat, error) {
	args := m.Called(ctx)
	return args.Get(0).([]MetricStat), args.Error(1)
}

func (m *mockClient) getIndexStats(ctx context.Context, database string) (*IndexStat, error) {
	args := m.Called(ctx, database)
	return args.Get(0).(*IndexStat), args.Error(1)
}

func (m *mockClient) getQueryStats(ctx context.Context, v version.Version) ([]QueryStat, error) {
	args := m.Called(ctx, v)
	return args.Get(0).([]QueryStat), args.Error(1)
}

func (m *mockClient) getReplicationDelay(_ context.Context) (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockClient) getReplicationStats(ctx context.Context) ([]replicationStats, error) {
	args := m.Called()
	return args.Get(0).([]replicationStats), args.Error(1)
}

func (m *mockClient) getMaxConnections(_ context.Context) (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockClient) getWALStats(c context.Context) (*walStats, error) {
	args := m.Called(c)
	return args.Get(0).(*walStats), args.Error(1)
}

func (m *mockClientFactory) getClient(c *Config, database string) (client, error) {
	args := m.Called(database)
	return args.Get(0).(client), args.Error(1)
}

func (m *mockClientFactory) initMocks(databases []string) {
	listClient := new(mockClient)
	listClient.initMocks("", databases, 0)
	m.On("getClient", "").Return(listClient, nil)

	for index, db := range databases {
		client := new(mockClient)
		client.initMocks(db, databases, index)
		m.On("getClient", db).Return(client, nil)
	}
}

func (m *mockClient) initMocks(database string, databases []string, index int) {
	m.On("Close").Return(nil)
	m.On("getVersion", mock.Anything).Return(version.NewVersion("10.21"))

	if database == "" {
		m.On("listDatabases").Return(databases, nil)

		commitsAndRollbacks := []MetricStat{}
		dbSize := []MetricStat{}
		backends := []MetricStat{}

		for idx, db := range databases {
			commitsAndRollbacks = append(commitsAndRollbacks, MetricStat{
				database: db,
				stats: map[string]string{
					"xact_commit":   fmt.Sprintf("%d", idx+1),
					"xact_rollback": fmt.Sprintf("%d", idx+2),
				},
			})
			dbSize = append(dbSize, MetricStat{
				database: db,
				stats:    map[string]string{"db_size": fmt.Sprintf("%d", idx+4)},
			})
			backends = append(backends, MetricStat{
				database: db,
				stats:    map[string]string{"count": fmt.Sprintf("%d", idx+3)},
			})
		}

		m.On("getCommitsAndRollbacks", databases).Return(commitsAndRollbacks, nil)
		m.On("getDatabaseSize", databases).Return(dbSize, nil)
		m.On("getBackends", databases).Return(backends, nil)
		m.On("getReplicationDelay").Return(int64(200), nil)
		m.On("getMaxConnections").Return(int64(100), nil)
		m.On("getWALStats", mock.Anything).Return(&walStats{
			age: 6799,
		}, nil)
		m.On("getBackgroundWriterStats", mock.Anything).Return([]MetricStat{
			{
				stats: map[string]string{
					"buffers_allocated":         "6",
					"checkpoint_req":            "8",
					"checkpoint_scheduled":      "12",
					"checkpoint_duration_write": "3",
					"checkpoint_duration_sync":  "3",
					"bg_writes":                 "44",
					"backend_writes":            "22",
					"buffers_written_fsync":     "12",
					"buffers_checkpoints":       "44",
					"maxwritten_count":          "1111",
				},
			},
		}, nil)
		m.On("getReplicationStats", mock.Anything).Return([]replicationStats{
			{
				flushLag:  int64(500),
				replayLag: int64(300),
				writeLag:  int64(421),
				client:    "198.64.23.1",
			},
		}, nil)
	} else {
		table1 := "public.table1"
		table2 := "public.table2"
		tableMetrics := []TableMetrics{}
		tableMetrics = append(tableMetrics, TableMetrics{
			database:    database,
			table:       table1,
			live:        int64(index + 7),
			dead:        int64(index + 8),
			inserts:     int64(89),
			upd:         int64(index + 40),
			del:         int64(index + 41),
			hotUpd:      int64(1),
			size:        int64(1024),
			vacuumCount: int64(2),
		})

		tableMetrics = append(tableMetrics, TableMetrics{
			database:    database,
			table:       table2,
			live:        int64(index + 7),
			dead:        int64(index + 8),
			inserts:     int64(89),
			upd:         int64(index + 40),
			del:         int64(index + 41),
			hotUpd:      int64(1),
			size:        int64(1024),
			vacuumCount: int64(2),
		})
		m.On("getDatabaseTableMetrics").Return(tableMetrics, nil)

		blocksMetrics := []MetricStat{}
		blocksMetrics = append(blocksMetrics, MetricStat{
			database: database,
			table:    "public.table1",
			stats: map[string]string{
				"heap_read":  fmt.Sprintf("%d", index+19),
				"heap_hit":   fmt.Sprintf("%d", index+20),
				"idx_read":   fmt.Sprintf("%d", index+21),
				"idx_hit":    fmt.Sprintf("%d", index+22),
				"toast_read": fmt.Sprintf("%d", index+23),
				"toast_hit":  fmt.Sprintf("%d", index+24),
				"tidx_read":  fmt.Sprintf("%d", index+25),
				"tidx_hit":   fmt.Sprintf("%d", index+26),
			},
		})

		blocksMetrics = append(blocksMetrics, MetricStat{
			database: database,
			table:    "public.table2",
			stats: map[string]string{
				"heap_read":  fmt.Sprintf("%d", index+27),
				"heap_hit":   fmt.Sprintf("%d", index+28),
				"idx_read":   fmt.Sprintf("%d", index+29),
				"idx_hit":    fmt.Sprintf("%d", index+30),
				"toast_read": fmt.Sprintf("%d", index+31),
				"toast_hit":  fmt.Sprintf("%d", index+32),
				"tidx_read":  fmt.Sprintf("%d", index+33),
				"tidx_hit":   fmt.Sprintf("%d", index+34),
			},
		})
		m.On("getBlocksReadByTable").Return(blocksMetrics, nil)

		index1 := "sequelize_pkey"
		index2 := "notifications_pkey"
		m.On("getIndexStats", mock.Anything, database).Return(&IndexStat{
			database: database,
			indexStats: map[string]indexStatHolder{
				index1: {
					table: table1,
					size:  1024,
					scans: 3,
					index: index1,
				},
				index2: {
					table: table2,
					size:  2048,
					scans: 67,
					index: index2,
				},
			},
		}, nil)
	}
}
