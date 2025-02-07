// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
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
)

func enableAllScraperMetrics(cfg *Config, enabled bool) {
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

	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryExecutionCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalElapsedTime.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalGrantKb.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalLogicalReads.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalLogicalWrites.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalPhysicalReads.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalRows.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalWorkerTime.Enabled = enabled
}

func TestEmptyScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	assert.NoError(t, cfg.Validate())

	// Ensure there aren't any scrapers when all metrics are disabled.
	// Disable all metrics manually that are enabled by default
	enableAllScraperMetrics(cfg, false)

	scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(), cfg)
	assert.Empty(t, scrapers)
}

func TestSuccessfulScrape(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	assert.NoError(t, cfg.Validate())

	enableAllScraperMetrics(cfg, true)
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryExecutionCount.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalRows.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalElapsedTime.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalGrantKb.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalLogicalReads.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalLogicalWrites.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalPhysicalReads.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalWorkerTime.Enabled = false

	scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(), cfg)
	assert.NotEmpty(t, scrapers)

	for _, scraper := range scrapers {
		err := scraper.Start(context.Background(), componenttest.NewNopHost())
		assert.NoError(t, err)
		defer assert.NoError(t, scraper.Shutdown(context.Background()))

		scraper.client = mockClient{
			instanceName:        scraper.instanceName,
			SQL:                 scraper.sqlQuery,
			maxQuerySampleCount: 10000,
			granularity:         10,
		}

		actualMetrics, err := scraper.ScrapeMetrics(context.Background())
		assert.NoError(t, err)

		var expectedFile string
		switch scraper.sqlQuery {
		case getSQLServerDatabaseIOQuery(scraper.instanceName):
			expectedFile = filepath.Join("testdata", "expectedDatabaseIO.yaml")
		case getSQLServerPerformanceCounterQuery(scraper.instanceName):
			expectedFile = filepath.Join("testdata", "expectedPerfCounters.yaml")
		case getSQLServerPropertiesQuery(scraper.instanceName):
			expectedFile = filepath.Join("testdata", "expectedProperties.yaml")
		case getSQLServerQueryMetricsQuery(scraper.instanceName, scraper.maxQuerySampleCount, scraper.granularity):
			expectedFile = filepath.Join("testdata", "expectedQueryMetrics.yaml")
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

func TestScrapeQueryMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	assert.NoError(t, cfg.Validate())

	enableAllScraperMetrics(cfg, false)
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryExecutionCount.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalRows.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalElapsedTime.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalGrantKb.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalLogicalReads.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalLogicalWrites.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalPhysicalReads.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalWorkerTime.Enabled = true

	scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(), cfg)
	assert.NotNil(t, scrapers)

	scraper := scrapers[0]
	assert.NotNil(t, scraper.cache)

	// update cache for testing
	queryHash := hex.EncodeToString([]byte("s1"))
	queryPlanHash := hex.EncodeToString([]byte("s3"))
	scraper.cacheAndDiff(queryHash, queryPlanHash, "execution_count", 5)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_elapsed_time", 149700)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_grant_kb", 13310)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_logical_reads", 0)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_logical_writes", 0)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_physical_reads", 0)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_rows", 100)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_worker_time", 149650)

	err := scraper.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
	defer assert.NoError(t, scraper.Shutdown(context.Background()))

	scraper.client = mockClient{
		instanceName:        scraper.instanceName,
		SQL:                 scraper.sqlQuery,
		maxQuerySampleCount: 10000,
		granularity:         10,
	}

	actualMetrics, err := scraper.ScrapeMetrics(context.Background())
	assert.NoError(t, err)

	expectedFile := filepath.Join("testdata", "expectedQueryMetrics.yaml")

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

func TestScrapeQueryInvalidMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	assert.NoError(t, cfg.Validate())

	enableAllScraperMetrics(cfg, false)
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryExecutionCount.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalRows.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalElapsedTime.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalGrantKb.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalLogicalReads.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalLogicalWrites.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalPhysicalReads.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalWorkerTime.Enabled = true

	scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(), cfg)
	assert.NotNil(t, scrapers)

	scraper := scrapers[0]
	assert.NotNil(t, scraper.cache)

	// update cache for testing
	queryHash := hex.EncodeToString([]byte("s1"))
	queryPlanHash := hex.EncodeToString([]byte("s3"))
	scraper.cacheAndDiff(queryHash, queryPlanHash, "execution_count", 5)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_elapsed_time", 149700)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_grant_kb", 13310)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_logical_reads", 0)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_logical_writes", 0)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_physical_reads", 0)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_rows", 100)
	scraper.cacheAndDiff(queryHash, queryPlanHash, "total_worker_time", 149650)

	err := scraper.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
	defer assert.NoError(t, scraper.Shutdown(context.Background()))

	scraper.client = mockInvalidClient{
		mockClient: mockClient{
			instanceName:        scraper.instanceName,
			SQL:                 scraper.sqlQuery,
			maxQuerySampleCount: 10000,
			granularity:         10,
		},
	}

	_, err = scraper.ScrapeMetrics(context.Background())
	assert.Error(t, err)
}

func TestScrapeInvalidQuery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true

	assert.NoError(t, cfg.Validate())

	enableAllScraperMetrics(cfg, true)
	scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(), cfg)
	assert.NotNil(t, scrapers)

	for _, scraper := range scrapers {
		err := scraper.Start(context.Background(), componenttest.NewNopHost())
		assert.NoError(t, err)
		defer assert.NoError(t, scraper.Shutdown(context.Background()))

		scraper.client = mockClient{
			instanceName: scraper.instanceName,
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

	assert.NoError(t, cfg.Validate())

	enableAllScraperMetrics(cfg, false)
	cfg.MetricsBuilderConfig.Metrics.SqlserverQueryTotalRows.Enabled = true

	scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(), cfg)
	assert.NotNil(t, scrapers)
	assert.Len(t, scrapers, 1)

	scraper := scrapers[0]
	cached, val := scraper.cacheAndDiff("query_hash", "query_plan_hash", "column", -1)
	assert.False(t, cached)
	assert.Equal(t, 0.0, val)

	cached, val = scraper.cacheAndDiff("query_hash", "query_plan_hash", "column", 1)
	assert.False(t, cached)
	assert.Equal(t, 1.0, val)

	cached, val = scraper.cacheAndDiff("query_hash", "query_plan_hash", "column", 1)
	assert.True(t, cached)
	assert.Equal(t, 0.0, val)

	cached, val = scraper.cacheAndDiff("query_hash", "query_plan_hash", "column", 3)
	assert.True(t, cached)
	assert.Equal(t, 2.0, val)

	scraper.cache = nil
	cached, val = scraper.cacheAndDiff("query_hash", "query_plan_hash", "column", 2)
	assert.False(t, cached)
	assert.Equal(t, 0.0, val)
}

func TestSortRows(t *testing.T) {
	weights := make([]int64, 50)

	for i := range weights {
		weights[i] = rand.Int64()
	}

	var rows []sqlquery.StringMap
	for _, v := range weights {
		rows = append(rows, sqlquery.StringMap{"column": strconv.FormatInt(v, 10)})
	}

	rows = sortRows(rows, weights)
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

var (
	_ sqlquery.DbClient = (*mockClient)(nil)
	_ sqlquery.DbClient = (*mockInvalidClient)(nil)
)

type mockClient struct {
	SQL                 string
	instanceName        string
	maxQuerySampleCount uint
	granularity         uint
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
	case getSQLServerQueryMetricsQuery(mc.instanceName, mc.maxQuerySampleCount, mc.granularity):
		queryResults, err = readFile("queryMetricsQueryData.txt")
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
	case getSQLServerPropertiesQuery(mc.instanceName):
		queryResults, err = readFile("propertyInvalidQueryData.txt")
	case getSQLServerQueryMetricsQuery(mc.instanceName, mc.maxQuerySampleCount, mc.granularity):
		queryResults, err = readFile("queryMetricsInvalidQueryData.txt")
	default:
		return nil, errors.New("No valid query found")
	}

	if err != nil {
		return nil, err
	}
	return queryResults, nil
}
