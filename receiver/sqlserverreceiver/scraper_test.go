// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
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
}

func TestSortRows(t *testing.T) {
	rand.New(rand.NewSource(time.Now().UnixNano()))

	weights := make([]int64, 50)

	for i := range weights {
		weights[i] = rand.Int63()
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
		assert.Nil(t, err)
		assert.Equal(t, expected, actual)
	}
}

var _ sqlquery.DbClient = (*mockClient)(nil)

type mockClient struct {
	SQL                 string
	instanceName        string
	maxQuerySampleCount uint
	granularity         uint
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

func TestAnyOf(t *testing.T) {
	tests := []struct {
		s    string
		f    func(a, b string) bool
		vals []string
		want bool
	}{
		{"TRANSACTION_MUTEX", strings.HasPrefix, []string{"XACT", "DTC"}, false},
		{"XACT_123", strings.HasPrefix, []string{"XACT", "DTC"}, true},
		{"DTC_123", strings.HasPrefix, []string{"XACT", "DTC"}, true},
		{"", strings.HasPrefix, []string{}, false},
		{"hello", func(a, b string) bool { return a == b }, []string{"hello", "world"}, true},
		{"notfound", func(a, b string) bool { return a == b }, []string{"hello", "world"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			if got := anyOf(tt.s, tt.f, tt.vals...); got != tt.want {
				t.Errorf("anyOf(%q, %v) = %v, want %v", tt.s, tt.vals, got, tt.want)
			}
		})
	}
}

func TestGetWaitCategory(t *testing.T) {
	tests := []struct {
		s       string
		want    uint
		wantStr string
	}{
		{"Unknown", 0, "Unknown"},
		{"SOS_SCHEDULER_YIELD", 1, "CPU"},
		{"THREADPOOL", 2, "Worker Thread"},
		{"LOCK_M_S", 3, "Lock"},
		{"LATCH_", 4, "Latch"},
		{"PAGELATCH_SH", 5, "Buffer Latch"},
		{"PAGEIOLATCH_SH", 6, "Buffer IO"},
		{"RESOURCE_SEMAPHORE_QUERY_COMPILE", 7, "Compilation"},
		{"CLR", 8, "SQL CLR"},
		{"DBMIRROR", 9, "Mirroring"},
		{"TRANSACTION_MUTEX", 10, "Transaction"},
		{"XACT_123", 10, "Transaction"},
		{"SLEEP_", 11, "Idle"},
		{"LAZYWRITER_SLEEP", 11, "Idle"},
		{"PREEMPTIVE_", 12, "Preemptive"},
		{"BROKER_", 13, "Service Broker"},
		{"LOGMGR", 14, "Tran Log IO"},
		{"ASYNC_NETWORK_IO", 15, "Network IO"},
		{"CXCONSUMER", 16, "Parallelism"},
		{"RESOURCE_SEMAPHORE", 17, "Memory"},
		{"WAITFOR", 18, "User Wait"},
		{"TRACEWRITE", 19, "Tracing"},
		{"FT_RESTART_CRAWL", 20, "Full Text Search"},
		{"ASYNC_IO_COMPLETION", 21, "Other Disk IO"},
		{"SE_REPL_", 22, "Replication"},
		{"RBIO_RG_", 23, "Log Rate Governor"},
		{"HADR_THROTTLE_LOG_RATE_GOVERNOR", 23, "Log Rate Governor"},
		{"NonExistent", 0, "Unknown"},
		{"DIRTY_PAGE_POLL", 0, "Unknown"},
		{"PVS_PREALLOCATE", 0, "Unknown"},
		{"WAIT_XTP_HOST_WAIT", 0, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got, gotStr := getWaitCategory(tt.s)
			if got != tt.want || gotStr != tt.wantStr {
				t.Errorf("getWaitCategory(%q) = (%v, %q), want (%v, %q)", tt.s, got, gotStr, tt.want, tt.wantStr)
			}
		})
	}
}
