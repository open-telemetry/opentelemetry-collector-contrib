// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestUnsuccessfulScrape(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "fake:11111"

	scraper := newPostgreSQLScraper(receivertest.NewNopCreateSettings(), cfg, &defaultClientFactory{})

	actualMetrics, err := scraper.scrape(context.Background())
	require.Error(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(pmetric.NewMetrics(), actualMetrics))
}

func TestScraper(t *testing.T) {
	factory := new(mockClientFactory)
	factory.initMocks([]string{"otel"})

	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{"otel"}
	scraper := newPostgreSQLScraper(receivertest.NewNopCreateSettings(), cfg, factory)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "otel", "expected.yaml")
	golden.WriteMetrics(t, expectedFile, actualMetrics)
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScraperNoDatabaseSingle(t *testing.T) {
	factory := new(mockClientFactory)
	factory.initMocks([]string{"otel"})

	cfg := createDefaultConfig().(*Config)
	scraper := newPostgreSQLScraper(receivertest.NewNopCreateSettings(), cfg, factory)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "otel", "expected.yaml")
	golden.WriteMetrics(t, expectedFile, actualMetrics)
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScraperNoDatabaseMultiple(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel", "open", "telemetry"})

	cfg := createDefaultConfig().(*Config)
	scraper := newPostgreSQLScraper(receivertest.NewNopCreateSettings(), cfg, &factory)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "multiple", "expected.yaml")
	golden.WriteMetrics(t, expectedFile, actualMetrics)
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScraperWithResourceAttributeFeatureGate(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel", "open", "telemetry"})

	cfg := createDefaultConfig().(*Config)
	scraper := newPostgreSQLScraper(receivertest.NewNopCreateSettings(), cfg, &factory)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "multiple", "expected_with_resource.yaml")
	golden.WriteMetrics(t, expectedFile, actualMetrics)
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScraperWithResourceAttributeFeatureGateSingle(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel"})

	cfg := createDefaultConfig().(*Config)
	scraper := newPostgreSQLScraper(receivertest.NewNopCreateSettings(), cfg, &factory)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "otel", "expected_with_resource.yaml")
	golden.WriteMetrics(t, expectedFile, actualMetrics)
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

type mockClientFactory struct{ mock.Mock }
type mockClient struct{ mock.Mock }

var _ client = &mockClient{}

func (m *mockClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockClient) getDatabaseStats(_ context.Context, databases []string) (map[databaseName]databaseStats, error) {
	args := m.Called(databases)
	return args.Get(0).(map[databaseName]databaseStats), args.Error(1)
}

func (m *mockClient) getBackends(_ context.Context, databases []string) (map[databaseName]int64, error) {
	args := m.Called(databases)
	return args.Get(0).(map[databaseName]int64), args.Error(1)
}

func (m *mockClient) getDatabaseSize(_ context.Context, databases []string) (map[databaseName]int64, error) {
	args := m.Called(databases)
	return args.Get(0).(map[databaseName]int64), args.Error(1)
}

func (m *mockClient) getDatabaseTableMetrics(ctx context.Context, database string) (map[tableIdentifier]tableStats, error) {
	args := m.Called(ctx, database)
	return args.Get(0).(map[tableIdentifier]tableStats), args.Error(1)
}

func (m *mockClient) getBlocksReadByTable(ctx context.Context, database string) (map[tableIdentifier]tableIOStats, error) {
	args := m.Called(ctx, database)
	return args.Get(0).(map[tableIdentifier]tableIOStats), args.Error(1)
}

func (m *mockClient) getIndexStats(ctx context.Context, database string) (map[indexIdentifer]indexStat, error) {
	args := m.Called(ctx, database)
	return args.Get(0).(map[indexIdentifer]indexStat), args.Error(1)
}

func (m *mockClient) getBGWriterStats(ctx context.Context) (*bgStat, error) {
	args := m.Called(ctx)
	return args.Get(0).(*bgStat), args.Error(1)
}

func (m *mockClient) getMaxConnections(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockClient) getLatestWalAgeSeconds(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockClient) getReplicationStats(ctx context.Context) ([]replicationStats, error) {
	args := m.Called(ctx)
	return args.Get(0).([]replicationStats), args.Error(1)
}

func (m *mockClient) listDatabases(_ context.Context) ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockClientFactory) getClient(_ *Config, database string) (client, error) {
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

	if database == "" {
		m.On("listDatabases").Return(databases, nil)

		commitsAndRollbacks := map[databaseName]databaseStats{}
		dbSize := map[databaseName]int64{}
		backends := map[databaseName]int64{}

		for idx, db := range databases {
			commitsAndRollbacks[databaseName(db)] = databaseStats{
				transactionCommitted: int64(idx + 1),
				transactionRollback:  int64(idx + 2),
			}
			dbSize[databaseName(db)] = int64(idx + 4)
			backends[databaseName(db)] = int64(idx + 3)
		}

		m.On("getDatabaseStats", databases).Return(commitsAndRollbacks, nil)
		m.On("getDatabaseSize", databases).Return(dbSize, nil)
		m.On("getBackends", databases).Return(backends, nil)
		m.On("getBGWriterStats", mock.Anything).Return(&bgStat{
			checkpointsReq:       1,
			checkpointsScheduled: 2,
			checkpointWriteTime:  3.12,
			checkpointSyncTime:   4.23,
			bgWrites:             5,
			backendWrites:        6,
			bufferBackendWrites:  7,
			bufferFsyncWrites:    8,
			bufferCheckpoints:    9,
			buffersAllocated:     10,
			maxWritten:           11,
		}, nil)
		m.On("getMaxConnections", mock.Anything).Return(int64(100), nil)
		m.On("getLatestWalAgeSeconds", mock.Anything).Return(int64(3600), nil)
		m.On("getReplicationStats", mock.Anything).Return([]replicationStats{
			{
				clientAddr:   "unix",
				pendingBytes: 1024,
				flushLag:     600,
				replayLag:    700,
				writeLag:     800,
			},
			{
				clientAddr:   "nulls",
				pendingBytes: -1,
				flushLag:     -1,
				replayLag:    -1,
				writeLag:     -1,
			},
		}, nil)
	} else {
		table1 := "public.table1"
		table2 := "public.table2"
		tableMetrics := map[tableIdentifier]tableStats{
			tableKey(database, table1): {
				database:    database,
				table:       table1,
				live:        int64(index + 7),
				dead:        int64(index + 8),
				inserts:     int64(index + 39),
				upd:         int64(index + 40),
				del:         int64(index + 41),
				hotUpd:      int64(index + 42),
				size:        int64(index + 43),
				vacuumCount: int64(index + 44),
			},
			tableKey(database, table2): {
				database:    database,
				table:       table2,
				live:        int64(index + 9),
				dead:        int64(index + 10),
				inserts:     int64(index + 43),
				upd:         int64(index + 44),
				del:         int64(index + 45),
				hotUpd:      int64(index + 46),
				size:        int64(index + 47),
				vacuumCount: int64(index + 48),
			},
		}

		blocksMetrics := map[tableIdentifier]tableIOStats{
			tableKey(database, table1): {
				database:  database,
				table:     table1,
				heapRead:  int64(index + 19),
				heapHit:   int64(index + 20),
				idxRead:   int64(index + 21),
				idxHit:    int64(index + 22),
				toastRead: int64(index + 23),
				toastHit:  int64(index + 24),
				tidxRead:  int64(index + 25),
				tidxHit:   int64(index + 26),
			},
			tableKey(database, table2): {
				database:  database,
				table:     table2,
				heapRead:  int64(index + 27),
				heapHit:   int64(index + 28),
				idxRead:   int64(index + 29),
				idxHit:    int64(index + 30),
				toastRead: int64(index + 31),
				toastHit:  int64(index + 32),
				tidxRead:  int64(index + 33),
				tidxHit:   int64(index + 34),
			},
		}

		m.On("getDatabaseTableMetrics", mock.Anything, database).Return(tableMetrics, nil)
		m.On("getBlocksReadByTable", mock.Anything, database).Return(blocksMetrics, nil)

		index1 := fmt.Sprintf("%s_test1_pkey", database)
		index2 := fmt.Sprintf("%s_test2_pkey", database)
		indexStats := map[indexIdentifer]indexStat{
			indexKey(database, table1, index1): {
				database: database,
				table:    table1,
				index:    index1,
				scans:    int64(index + 35),
				size:     int64(index + 36),
			},
			indexKey(index2, table2, index2): {
				database: database,
				table:    table2,
				index:    index2,
				scans:    int64(index + 37),
				size:     int64(index + 38),
			},
		}
		m.On("getIndexStats", mock.Anything, database).Return(indexStats, nil)
	}
}
