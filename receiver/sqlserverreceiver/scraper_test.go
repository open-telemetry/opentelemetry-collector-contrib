// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

func configureAllScraperMetricsAndEvents(cfg *Config, enabled bool) {
	// Some of these metrics are enabled by default, but it's still helpful to include
	// in the case of using a config that may have previously disabled a metric.
	cfg.Metrics.SqlserverBatchRequestRate.Enabled = enabled
	cfg.Metrics.SqlserverBatchSQLCompilationRate.Enabled = enabled
	cfg.Metrics.SqlserverBatchSQLRecompilationRate.Enabled = enabled
	cfg.Metrics.SqlserverDatabaseBackupOrRestoreRate.Enabled = enabled
	cfg.Metrics.SqlserverDatabaseCount.Enabled = enabled
	cfg.Metrics.SqlserverDatabaseExecutionErrors.Enabled = enabled
	cfg.Metrics.SqlserverDatabaseFullScanRate.Enabled = enabled
	cfg.Metrics.SqlserverDatabaseIo.Enabled = enabled
	cfg.Metrics.SqlserverDatabaseLatency.Enabled = enabled
	cfg.Metrics.SqlserverDatabaseOperations.Enabled = enabled
	cfg.Metrics.SqlserverDatabaseTempdbSpace.Enabled = enabled
	cfg.Metrics.SqlserverDatabaseTempdbVersionStoreSize.Enabled = enabled
	cfg.Metrics.SqlserverDeadlockRate.Enabled = enabled
	cfg.Metrics.SqlserverIndexSearchRate.Enabled = enabled
	cfg.Metrics.SqlserverLockTimeoutRate.Enabled = enabled
	cfg.Metrics.SqlserverLockWaitCount.Enabled = enabled
	cfg.Metrics.SqlserverLockWaitRate.Enabled = enabled
	cfg.Metrics.SqlserverLockWaitTimeAvg.Enabled = enabled
	cfg.Metrics.SqlserverLoginRate.Enabled = enabled
	cfg.Metrics.SqlserverLogoutRate.Enabled = enabled
	cfg.Metrics.SqlserverMemoryGrantsPendingCount.Enabled = enabled
	cfg.Metrics.SqlserverMemoryUsage.Enabled = enabled
	cfg.Metrics.SqlserverOsWaitDuration.Enabled = enabled
	cfg.Metrics.SqlserverPageBufferCacheFreeListStallsRate.Enabled = enabled
	cfg.Metrics.SqlserverPageBufferCacheHitRatio.Enabled = enabled
	cfg.Metrics.SqlserverPageCheckpointFlushRate.Enabled = enabled
	cfg.Metrics.SqlserverPageLazyWriteRate.Enabled = enabled
	cfg.Metrics.SqlserverPageLifeExpectancy.Enabled = enabled
	cfg.Metrics.SqlserverPageLookupRate.Enabled = enabled
	cfg.Metrics.SqlserverPageOperationRate.Enabled = enabled
	cfg.Metrics.SqlserverPageSplitRate.Enabled = enabled
	cfg.Metrics.SqlserverProcessesBlocked.Enabled = enabled
	cfg.Metrics.SqlserverReplicaDataRate.Enabled = enabled
	cfg.Metrics.SqlserverResourcePoolDiskOperations.Enabled = enabled
	cfg.Metrics.SqlserverResourcePoolDiskThrottledReadRate.Enabled = enabled
	cfg.Metrics.SqlserverResourcePoolDiskThrottledWriteRate.Enabled = enabled
	cfg.Metrics.SqlserverTableCount.Enabled = enabled
	cfg.Metrics.SqlserverTransactionDelay.Enabled = enabled
	cfg.Metrics.SqlserverTransactionLogFlushDataRate.Enabled = enabled
	cfg.Metrics.SqlserverTransactionLogFlushRate.Enabled = enabled
	cfg.Metrics.SqlserverTransactionLogFlushWaitRate.Enabled = enabled
	cfg.Metrics.SqlserverTransactionLogGrowthCount.Enabled = enabled
	cfg.Metrics.SqlserverTransactionLogShrinkCount.Enabled = enabled
	cfg.Metrics.SqlserverTransactionLogUsage.Enabled = enabled
	cfg.Metrics.SqlserverTransactionMirrorWriteRate.Enabled = enabled
	cfg.Metrics.SqlserverTransactionRate.Enabled = enabled
	cfg.Metrics.SqlserverTransactionWriteRate.Enabled = enabled
	cfg.Metrics.SqlserverUserConnectionCount.Enabled = enabled

	cfg.Events.DbServerTopQuery.Enabled = enabled
	cfg.Events.DbServerQuerySample.Enabled = enabled
	// cfg.TopQueryCollection.Enabled = enabled
	// cfg.QuerySample.Enabled = enabled
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
	configureAllScraperMetricsAndEvents(cfg, false)

	scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.Empty(t, scrapers)
}

func TestSuccessfulScrape(t *testing.T) {
	tests := []struct {
		removeServerResourceAttributeFeatureGate bool
		name                                     string
	}{
		{
			name:                                     "TestSuccessfulScrape with removing server resource attribute feature gate on",
			removeServerResourceAttributeFeatureGate: true,
		},
		{
			name:                                     "TestSuccessfulScrape with removing server resource attribute feature gate off",
			removeServerResourceAttributeFeatureGate: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testutil.SetFeatureGateForTest(t, removeServerResourceAttributeFeatureGate, test.removeServerResourceAttributeFeatureGate)
			cfg := createDefaultConfig().(*Config)
			cfg.Username = "sa"
			cfg.Password = "password"
			cfg.Port = 1433
			cfg.Server = "0.0.0.0"
			cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
			cfg.MetricsBuilderConfig.ResourceAttributes.ServerAddress.Enabled = true
			cfg.MetricsBuilderConfig.ResourceAttributes.ServerPort.Enabled = true
			assert.NoError(t, cfg.Validate())

			configureAllScraperMetricsAndEvents(cfg, true)

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
				fileSuffix := ".yaml"
				if test.removeServerResourceAttributeFeatureGate {
					fileSuffix = "RemoveServerResourceAttributes.yaml"
				}
				var expectedFile string
				switch scraper.sqlQuery {
				case getSQLServerDatabaseIOQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedDatabaseIO")
				case getSQLServerPerformanceCounterQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedPerfCounters")
				case getSQLServerPropertiesQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedProperties")
				case getSQLServerWaitStatsQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedWaitStats")
				}
				expectedFile += fileSuffix

				// Uncomment line below to re-generate expected metrics.
				// golden.WriteMetrics(t, expectedFile, actualMetrics)
				expectedMetrics, err := golden.ReadMetrics(expectedFile)
				assert.NoError(t, err)

				assert.NoError(t, pmetrictest.CompareMetrics(actualMetrics, expectedMetrics,
					pmetrictest.IgnoreMetricDataPointsOrder(),
					pmetrictest.IgnoreStartTimestamp(),
					pmetrictest.IgnoreTimestamp(),
					pmetrictest.IgnoreResourceMetricsOrder()), expectedFile)
			}
		})
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

	configureAllScraperMetricsAndEvents(cfg, true)
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
	cfg.Events.DbServerTopQuery.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetricsAndEvents(cfg, false)

	cfg.Events.DbServerTopQuery.Enabled = true
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
	var err error

	switch mc.SQL {
	case getSQLServerDatabaseIOQuery(mc.instanceName):
		queryResults, err = readFile("database_io_scraped_data.txt")
	case getSQLServerPerformanceCounterQuery(mc.instanceName):
		queryResults, err = readFile("perfCounterQueryData.txt")
	case getSQLServerPropertiesQuery(mc.instanceName):
		queryResults, err = readFile("propertyQueryData.txt")
	case getSQLServerWaitStatsQuery(mc.instanceName):
		queryResults, err = readFile("waitStatsQueryData.txt")
	case getSQLServerQueryTextAndPlanQuery():
		queryResults, err = readFile("queryTextAndPlanQueryData.txt")
	case getSQLServerQuerySamplesQuery():
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
	var err error

	switch mc.SQL {
	case getSQLServerQuerySamplesQuery():
		queryResults, err = readFile("recordInvalidDatabaseSampleQueryData.txt")
	case getSQLServerQueryTextAndPlanQuery():
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
	cfg.Events.DbServerTopQuery.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetricsAndEvents(cfg, false)
	cfg.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.CollectionInterval = cfg.ControllerConfig.CollectionInterval

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

	// Uncomment line below to re-generate expected logs.
	// golden.WriteLogs(t, expectedFile, actualLogs)
	expectedLogs, _ := golden.ReadLogs(expectedFile)
	errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreTimestamp())
	assert.Equal(t, "db.server.top_query", actualLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
	assert.NoError(t, errs)
}

func TestInvalidQueryTextAndPlanQuery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.Events.DbServerTopQuery.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetricsAndEvents(cfg, false)
	cfg.Events.DbServerTopQuery.Enabled = true

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

	actualLogs, err := scraper.ScrapeLogs(context.Background())
	assert.Error(t, err)

	expectedFile := "expectedQueryTextAndPlanQueryWithInvalidData.yaml"

	// Uncomment line below to re-generate expected logs.
	// golden.WriteLogs(t, filepath.Join("testdata", expectedFile), actualLogs)

	expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", expectedFile))
	assert.NoError(t, err)

	errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreTimestamp())
	assert.Equal(t, "db.server.top_query", actualLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
	assert.NoError(t, errs)
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

			configureAllScraperMetricsAndEvents(cfg, false)
			cfg.Events.DbServerQuerySample.Enabled = true

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

			// Uncomment line below to re-generate expected logs.
			// golden.WriteLogs(t, filepath.Join("testdata", tc.expectedFile), actualLogs)
			expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", tc.expectedFile))
			assert.NoError(t, err)
			errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreTimestamp())
			assert.Equal(t, "db.server.query_sample", actualLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
			assert.NoError(t, errs)
		})
	}
}
