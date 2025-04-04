// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

func configureAllScraperMetrics(cfg *Config, enabled bool) {
	// Some of these metrics are enabled by default, but it's still helpful to include
	// in the case of using a config that may have previously disabled a metric.
	cfg.MetricsBuilderConfig.Metrics.SqlserverBatchRequestRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverBatchSQLCompilationRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverBatchSQLRecompilationRate.Enabled = enabled

	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseIo.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseLatency.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseOperations.Enabled = enabled

	cfg.MetricsBuilderConfig.Metrics.SqlserverLockWaitRate.Enabled = enabled

	cfg.MetricsBuilderConfig.Metrics.SqlserverPageBufferCacheHitRatio.Enabled = enabled

	cfg.MetricsBuilderConfig.Metrics.SqlserverProcessesBlocked.Enabled = enabled

	cfg.MetricsBuilderConfig.Metrics.SqlserverResourcePoolDiskThrottledReadRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverResourcePoolDiskThrottledWriteRate.Enabled = enabled

	cfg.MetricsBuilderConfig.Metrics.SqlserverUserConnectionCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverUserConnectionCount.Enabled = enabled

	cfg.MetricsBuilderConfig.Metrics.SqlserverTableCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverReplicaDataRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseExecutionErrors.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageBufferCacheFreeListStallsRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseTempdbSpace.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseFullScanRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverIndexSearchRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockTimeoutRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLoginRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLogoutRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDeadlockRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionMirrorWriteRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverMemoryGrantsPendingCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageLookupRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionDelay.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseTempdbVersionStoreSize.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseBackupOrRestoreRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverMemoryUsage.Enabled = enabled

	cfg.TopQueryCollection.Enabled = enabled
	cfg.QuerySample.Enabled = enabled
}

func TestEmptyScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	cfg.MetricsBuilderConfig.ResourceAttributes.ServerPort.Enabled = true
	assert.NoError(t, cfg.Validate())

	// Ensure there aren't any scrapers when all metrics are disabled.
	// Disable all metrics manually that are enabled by default
	configureAllScraperMetrics(cfg, false)

	scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.Empty(t, scrapers)
}

func TestSuccessfulScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	cfg.MetricsBuilderConfig.ResourceAttributes.ServerAddress.Enabled = true
	cfg.MetricsBuilderConfig.ResourceAttributes.ServerPort.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetrics(cfg, true)

	scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotEmpty(t, scrapers)

	for _, scraper := range scrapers {
		err := scraper.Start(context.Background(), componenttest.NewNopHost())
		assert.NoError(t, err)
		defer assert.NoError(t, scraper.Shutdown(context.Background()))

		scraper.client = mockClient{
			instanceName:        scraper.config.InstanceName,
			SQL:                 scraper.sqlQuery,
			maxQuerySampleCount: 1000,
			lookbackTime:        20,
		}

		actualMetrics, err := scraper.ScrapeMetrics(context.Background())
		assert.NoError(t, err)

		var expectedFile string
		switch scraper.sqlQuery {
		case getSQLServerDatabaseIOQuery(scraper.config.InstanceName):
			expectedFile = filepath.Join("testdata", "expectedDatabaseIO.yaml")
		case getSQLServerPerformanceCounterQuery(scraper.config.InstanceName):
			expectedFile = filepath.Join("testdata", "expectedPerfCounters.yaml")
		case getSQLServerPropertiesQuery(scraper.config.InstanceName):
			expectedFile = filepath.Join("testdata", "expectedProperties.yaml")
		}

		// Uncomment line below to re-generate expected metrics.
		// golden.WriteMetrics(t, expectedFile, actualMetrics)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		assert.NoError(t, err)

		assert.NoError(t, pmetrictest.CompareMetrics(actualMetrics, expectedMetrics,
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreResourceMetricsOrder()))
	}
}

func TestScrapeInvalidQuery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	cfg.MetricsBuilderConfig.ResourceAttributes.ServerPort.Enabled = true

	assert.NoError(t, cfg.Validate())

	configureAllScraperMetrics(cfg, true)
	scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, scrapers)

	for _, scraper := range scrapers {
		err := scraper.Start(context.Background(), componenttest.NewNopHost())
		assert.NoError(t, err)
		defer assert.NoError(t, scraper.Shutdown(context.Background()))

		scraper.client = mockClient{
			instanceName: scraper.config.InstanceName,
			SQL:          "Invalid SQL query",
		}

		actualMetrics, err := scraper.ScrapeMetrics(context.Background())
		assert.Error(t, err)
		assert.Empty(t, actualMetrics)
	}
}

func TestScrapeCacheAndDiff(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	cfg.TopQueryCollection.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetrics(cfg, false)

	cfg.TopQueryCollection.Enabled = true
	scrapers := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, scrapers)

	scraper := scrapers[0]
	cached, val := scraper.cacheAndDiff("query_hash", "query_plan_hash", "column", -1)
	assert.False(t, cached)
	assert.Equal(t, int64(0), val)

	cached, val = scraper.cacheAndDiff("query_hash", "query_plan_hash", "column", 1)
	assert.False(t, cached)
	assert.Equal(t, int64(1), val)

	cached, val = scraper.cacheAndDiff("query_hash", "query_plan_hash", "column", 1)
	assert.True(t, cached)
	assert.Equal(t, int64(0), val)

	cached, val = scraper.cacheAndDiff("query_hash", "query_plan_hash", "column", 3)
	assert.True(t, cached)
	assert.Equal(t, int64(2), val)
}

func TestSortRows(t *testing.T) {
	assert.Equal(t, []sqlquery.StringMap{}, sortRows(nil, nil, 0))
	assert.Equal(t, []sqlquery.StringMap{}, sortRows([]sqlquery.StringMap{}, []int64{}, 0))
	assert.Equal(t, []sqlquery.StringMap{}, sortRows([]sqlquery.StringMap{
		{"column": "1"},
	}, []int64{1, 2}, 1))
	assert.Equal(
		t,
		[]sqlquery.StringMap{{"ghi": "56"}, {"def": "34"}, {"abc": "12"}},
		sortRows([]sqlquery.StringMap{{"abc": "12"}, {"ghi": "56"}, {"def": "34"}}, []int64{1, 2, 2}, 3))

	assert.Equal(
		t,
		[]sqlquery.StringMap{{"ghi": "56"}, {"def": "34"}},
		sortRows([]sqlquery.StringMap{{"abc": "12"}, {"ghi": "56"}, {"def": "34"}}, []int64{1, 2, 2}, 2))

	assert.Equal(
		t,
		[]sqlquery.StringMap{{"ghi": "56"}},
		sortRows([]sqlquery.StringMap{{"abc": "12"}, {"ghi": "56"}, {"def": "34"}}, []int64{1, 2, 2}, 1))

	weights := make([]int64, 50)

	for i := range weights {
		weights[i] = rand.Int64()
	}

	var rows []sqlquery.StringMap
	for _, v := range weights {
		rows = append(rows, sqlquery.StringMap{"column": strconv.FormatInt(v, 10)})
	}

	rows = sortRows(rows, weights, uint(len(weights)))
	sort.Slice(weights, func(i, j int) bool {
		return weights[i] > weights[j]
	})

	for i, v := range weights {
		expected := v
		actual, err := strconv.ParseInt(rows[i]["column"], 10, 64)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	}
}

var _ sqlquery.DbClient = (*mockClient)(nil)

type mockClient struct {
	SQL                 string
	instanceName        string
	maxQuerySampleCount uint
	lookbackTime        uint
	topQueryCount       uint
	maxRowsPerQuery     uint64
}

type mockInvalidClient struct {
	mockClient
}

func readFile(fname string) ([]sqlquery.StringMap, error) {
	file, err := os.ReadFile(filepath.Join("testdata", fname))
	if err != nil {
		return nil, err
	}

	var metrics []sqlquery.StringMap
	err = json.Unmarshal(file, &metrics)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

func (mc mockClient) QueryRows(context.Context, ...any) ([]sqlquery.StringMap, error) {
	var queryResults []sqlquery.StringMap

	queryTextAndPlanQuery, err := getSQLServerQueryTextAndPlanQuery(mc.instanceName, mc.maxQuerySampleCount, mc.lookbackTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get query text and plan query: %w", err)
	}

	switch mc.SQL {
	case getSQLServerDatabaseIOQuery(mc.instanceName):
		queryResults, err = readFile("database_io_scraped_data.txt")
	case getSQLServerPerformanceCounterQuery(mc.instanceName):
		queryResults, err = readFile("perfCounterQueryData.txt")
	case getSQLServerPropertiesQuery(mc.instanceName):
		queryResults, err = readFile("propertyQueryData.txt")
	case queryTextAndPlanQuery:
		queryResults, err = readFile("queryTextAndPlanQueryData.txt")
	case getSQLServerQuerySamplesQuery(mc.maxRowsPerQuery):
		queryResults, err = readFile("recordDatabaseSampleQueryData.txt")
	default:
		return nil, errors.New("No valid query found")
	}

	if err != nil {
		return nil, err
	}
	return queryResults, nil
}

func (mc mockInvalidClient) QueryRows(context.Context, ...any) ([]sqlquery.StringMap, error) {
	var queryResults []sqlquery.StringMap

	queryTextAndPlanQuery, err := getSQLServerQueryTextAndPlanQuery(mc.instanceName, mc.maxQuerySampleCount, mc.lookbackTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get query text and plan query: %w", err)
	}

	switch mc.SQL {
	case getSQLServerQuerySamplesQuery(mc.maxRowsPerQuery):
		queryResults, err = readFile("recordInvalidDatabaseSampleQueryData.txt")
	case queryTextAndPlanQuery:
		queryResults, err = readFile("queryTextAndPlanQueryInvalidData.txt")
	default:
		return nil, errors.New("No valid query found")
	}

	if err != nil {
		return nil, err
	}
	return queryResults, nil
}

func TestQueryTextAndPlanQuery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	cfg.TopQueryCollection.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetrics(cfg, false)
	cfg.TopQueryCollection.Enabled = true

	scrapers := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, scrapers)

	scraper := scrapers[0]
	assert.NotNil(t, scraper.cache)

	const totalElapsedTime = "total_elapsed_time"
	const rowsReturned = "total_rows"
	const totalWorkerTime = "total_worker_time"
	const logicalReads = "total_logical_reads"
	const logicalWrites = "total_logical_writes"
	const physicalReads = "total_physical_reads"
	const executionCount = "execution_count"
	const totalGrant = "total_grant_kb"

	queryHash := hex.EncodeToString([]byte("0x37849E874171E3F3"))
	queryPlanHash := hex.EncodeToString([]byte("0xD3112909429A1B50"))
	scraper.cacheAndDiff(queryHash, queryPlanHash, totalElapsedTime, 846)
	scraper.cacheAndDiff(queryHash, queryPlanHash, rowsReturned, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, logicalReads, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, logicalWrites, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, physicalReads, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, executionCount, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, totalWorkerTime, 845)
	scraper.cacheAndDiff(queryHash, queryPlanHash, totalGrant, 1)

	scraper.client = mockClient{
		instanceName:        scraper.config.InstanceName,
		SQL:                 scraper.sqlQuery,
		maxQuerySampleCount: 1000,
		lookbackTime:        20,
		topQueryCount:       200,
	}

	actualLogs, err := scraper.ScrapeLogs(context.Background())
	assert.NoError(t, err)

	expectedFile := filepath.Join("testdata", "expectedQueryTextAndPlanQuery.yaml")

	// Uncomment line below to re-generate expected metrics.
	// golden.WriteLogs(t, expectedFile, actualLogs)
	expectedLogs, _ := golden.ReadLogs(expectedFile)
	errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreTimestamp())
	assert.Equal(t, "top query", actualLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
	assert.NoError(t, errs)
}

func TestInvalidQueryTextAndPlanQuery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.TopQueryCollection.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetrics(cfg, false)
	cfg.TopQueryCollection.Enabled = true

	scrapers := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, scrapers)

	scraper := scrapers[0]
	assert.NotNil(t, scraper.cache)

	const totalElapsedTime = "total_elapsed_time"
	const rowsReturned = "total_rows"
	const totalWorkerTime = "total_worker_time"
	const logicalReads = "total_logical_reads"
	const logicalWrites = "total_logical_writes"
	const physicalReads = "total_physical_reads"
	const executionCount = "execution_count"
	const totalGrant = "total_grant_kb"

	queryHash := hex.EncodeToString([]byte("0x37849E874171E3F3"))
	queryPlanHash := hex.EncodeToString([]byte("0xD3112909429A1B50"))
	scraper.cacheAndDiff(queryHash, queryPlanHash, totalElapsedTime, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, rowsReturned, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, logicalReads, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, logicalWrites, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, physicalReads, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, executionCount, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, totalWorkerTime, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, totalGrant, 1)

	scraper.client = mockInvalidClient{
		mockClient: mockClient{
			instanceName:        scraper.config.InstanceName,
			SQL:                 scraper.sqlQuery,
			maxQuerySampleCount: 1000,
			lookbackTime:        20,
		},
	}

	_, err := scraper.ScrapeLogs(context.Background())
	assert.Error(t, err)
}

func TestRecordDatabaseSampleQuery(t *testing.T) {
	tests := map[string]struct {
		expectedFile string
		mockClient   func(instance, sql string) sqlquery.DbClient
		errors       bool
	}{
		"valid data": {
			expectedFile: "expectedRecordDatabaseSampleQuery.yaml",
			mockClient: func(instance, sql string) sqlquery.DbClient {
				return mockClient{
					instanceName:    instance,
					SQL:             sql,
					maxRowsPerQuery: 100,
				}
			},
			errors: false,
		},
		"invalid data": {
			expectedFile: "expectedRecordDatabaseSampleQueryWithInvalidData.yaml",
			mockClient: func(instance, sql string) sqlquery.DbClient {
				return mockInvalidClient{
					mockClient{
						instanceName:    instance,
						SQL:             sql,
						maxRowsPerQuery: 100,
					},
				}
			},
			errors: true,
		},
	}

	for name, tc := range tests {
		t.Run("TestRecordDatabaseSampleQuery/"+name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Username = "sa"
			cfg.Password = "password"
			cfg.Port = 1433
			cfg.Server = "0.0.0.0"
			cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
			assert.NoError(t, cfg.Validate())

			configureAllScraperMetrics(cfg, false)
			cfg.QuerySample.Enabled = true

			scrapers := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
			assert.NotNil(t, scrapers)

			scraper := scrapers[0]
			assert.NotNil(t, scraper.cache)

			scraper.client = tc.mockClient(scraper.instanceName, scraper.sqlQuery)

			actualLogs, err := scraper.ScrapeLogs(context.Background())
			if tc.errors {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", tc.expectedFile))
			assert.NoError(t, err)
			errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreTimestamp())
			assert.Equal(t, "query sample", actualLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
			assert.NoError(t, errs)
		})
	}
}
