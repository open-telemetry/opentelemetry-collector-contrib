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

func (m *mockClient) getDatabaseTableMetrics(_ context.Context) ([]MetricStat, error) {
	args := m.Called()
	return args.Get(0).([]MetricStat), args.Error(1)
}

func (m *mockClient) getBlocksReadByTable(_ context.Context) ([]MetricStat, error) {
	args := m.Called()
	return args.Get(0).([]MetricStat), args.Error(1)
}

func (m *mockClient) listDatabases(_ context.Context) ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
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
	} else {
		tableMetrics := []MetricStat{}
		tableMetrics = append(tableMetrics, MetricStat{
			database: database,
			table:    "public.table1",
			stats: map[string]string{
				"live":    fmt.Sprintf("%d", index+7),
				"dead":    fmt.Sprintf("%d", index+8),
				"ins":     fmt.Sprintf("%d", index+39),
				"upd":     fmt.Sprintf("%d", index+40),
				"del":     fmt.Sprintf("%d", index+41),
				"hot_upd": fmt.Sprintf("%d", index+42),
			},
		})

		tableMetrics = append(tableMetrics, MetricStat{
			database: database,
			table:    "public.table2",
			stats: map[string]string{
				"live":    fmt.Sprintf("%d", index+9),
				"dead":    fmt.Sprintf("%d", index+10),
				"ins":     fmt.Sprintf("%d", index+43),
				"upd":     fmt.Sprintf("%d", index+44),
				"del":     fmt.Sprintf("%d", index+45),
				"hot_upd": fmt.Sprintf("%d", index+46),
			},
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
	}
}
