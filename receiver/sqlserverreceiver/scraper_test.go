// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"context"
	"database/sql"
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
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

func configureAllScraperMetricsAndEvents(cfg *Config, enabled bool) {
	// Some of these metrics are enabled by default, but it's still helpful to include
	// in the case of using a config that may have previously disabled a metric.
	cfg.MetricsBuilderConfig.Metrics.SqlserverAccessScanRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverBatchRequestRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverCPUUtilization.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverBatchSQLCompilationRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverBatchSQLRecompilationRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverConnectionResetRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseBackupOrRestoreRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseExecutionErrors.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseFullScanRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseIo.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseLatency.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseOperations.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseTempdbSpace.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDatabaseTempdbVersionStoreSize.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDeadlockRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDiskOperations.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverDiskIo.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverErrorRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverExtentOperationRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverGhostRecordSkippedRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverIndexFragmentation.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverIndexPageCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverIndexPageUtilization.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverIndexRecordCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverIndexSearchRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverIndexSize.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLatchSuperlatchCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLatchSuperlatchTransitionRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLatchWaitRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLatchWaitTimeAvg.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLatchWaitTimeTotal.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockBlockCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockEscalationRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockMemory.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockRequestRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockTimeoutRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockWaitCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockWaitRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockWaitTimeAvg.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockWaitTimeTotal.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLoginRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverLogoutRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverMemoryArea.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverMemoryCacheObjectCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverMemoryGrantsPendingCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverMemoryPageCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverMemoryUsage.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverOsWaitDuration.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageAllocationRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageCompressionRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageBufferCacheFreeListStallsRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageBufferCacheHitRatio.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageCheckpointFlushRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageLazyWriteRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageLifeExpectancy.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageLookupRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageOperationRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageReadAheadRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageSplitRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverProcessesBlocked.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverReplicaDataRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverResourcePoolDiskOperations.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverResourcePoolDiskThrottledReadRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverResourcePoolDiskThrottledWriteRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverScanPointRevalidationRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverAttentionRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverClrExecutionTime.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverCursorCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverCursorMemoryUsage.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverCursorPlanCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverCursorRequestRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverParameterizationRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverPlanExecutionRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverRecompilationRatio.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverStoredProcedureInvocationRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTableCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTaskCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTaskRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionDelay.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogFlushDataRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogFlushRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogFlushWaitRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogGrowthCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogShrinkCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionLogUsage.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionMirrorWriteRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverTransactionWriteRate.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverUserConnectionCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverHostMemoryLimit.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverHostMemoryUsage.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverWorkerRequestCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverWorkerThreadCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverWorktableCacheHitRatio.Enabled = enabled

	cfg.MetricsBuilderConfig.Metrics.SqlserverAvailabilityGroupDatabaseReplicaSecondaryLag.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverAvailabilityGroupDatabaseReplicaQueueSize.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverAvailabilityGroupDatabaseReplicaQueueRate.Enabled = enabled
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = enabled
	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverCPUCount.Enabled = enabled
	cfg.MetricsBuilderConfig.Metrics.SqlserverComputerUptime.Enabled = enabled
	// cfg.TopQueryCollection.Enabled = enabled
	// cfg.QuerySample.Enabled = enabled
}

func enableSQLServerResourceAttributesForTests(resourceAttributes *metadata.ResourceAttributesConfig) {
	resourceAttributes.SqlserverComputerName.Enabled = true
	resourceAttributes.SqlserverInstanceName.Enabled = true
	resourceAttributes.ServerAddress.Enabled = true
	resourceAttributes.ServerPort.Enabled = true
	resourceAttributes.ServiceName.Enabled = true
	resourceAttributes.ServiceNamespace.Enabled = true
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

	scrapers, _ := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.Empty(t, scrapers)
}

func TestSuccessfulScrape(t *testing.T) {
	tests := []struct {
		name                  string
		propertiesFixtureFile string
	}{
		{
			name: "TestSuccessfulScrape",
		},
		{
			// Azure SQL Managed Instance (EngineEdition 8) returns a reduced property column
			// set: the host/registry columns (service_name, instance_type, ForceEncryption,
			// Port, PortType, hardware_type) are omitted, none of which the receiver consumes.
			// The emitted metrics must therefore match the full on-prem golden file, guarding
			// against a future change that reads a now-absent column without a nil guard.
			name:                  "TestSuccessfulScrape with Azure SQL Managed Instance property columns",
			propertiesFixtureFile: "propertyQueryDataManagedInstance.txt",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Username = "sa"
			cfg.Password = "password"
			cfg.Port = 1433
			cfg.Server = "0.0.0.0"
			cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
			cfg.MetricsBuilderConfig.ResourceAttributes.ServiceName.Enabled = true
			cfg.MetricsBuilderConfig.ResourceAttributes.ServiceNamespace.Enabled = true
			cfg.LogsBuilderConfig.ResourceAttributes.ServiceName.Enabled = true
			cfg.LogsBuilderConfig.ResourceAttributes.ServiceNamespace.Enabled = true
			assert.NoError(t, cfg.Validate())

			configureAllScraperMetricsAndEvents(cfg, true)

			scrapers, provider := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
			assert.NotEmpty(t, scrapers)
			// The receiver owns the shared pool; close it once the test is done.
			t.Cleanup(func() { assert.NoError(t, provider.close()) })

			for _, scraper := range scrapers {
				err := scraper.Start(t.Context(), componenttest.NewNopHost())
				assert.NoError(t, err)
				defer assert.NoError(t, scraper.Shutdown(t.Context()))

				scraper.client = mockClient{
					instanceName:          scraper.config.InstanceName,
					SQL:                   scraper.sqlQuery,
					maxQuerySampleCount:   1000,
					lookbackTime:          20,
					propertiesFixtureFile: test.propertiesFixtureFile,
				}

				actualMetrics, err := scraper.ScrapeMetrics(t.Context())
				assert.NoError(t, err)
				var expectedFile string
				switch scraper.sqlQuery {
				case getSQLServerAvailabilityGroupQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedAvailabilityGroupMetrics.yaml")
				case getSQLServerDatabaseIOQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedDatabaseIO.yaml")
				case getSQLServerPerformanceCounterQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedPerfCounters.yaml")
				case getSQLServerPropertiesQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedProperties.yaml")
				case getSQLServerWaitStatsQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedWaitStats.yaml")
				case getSQLServerIndexPhysicalStatsQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedIndexPhysicalMetrics.yaml")
				case getSQLServerWorkerThreadsQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedWorkerThreads.yaml")
				case getSQLServerCPUMemoryQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedCPUMemory.yaml")
				case getSQLServerDiskIOQuery(scraper.config.InstanceName):
					expectedFile = filepath.Join("testdata", "expectedDiskIO.yaml")
				}

				// Uncomment line below to re-generate expected metrics.
				// golden.WriteMetrics(t, expectedFile, actualMetrics)
				expectedMetrics, err := golden.ReadMetrics(expectedFile)
				assert.NoError(t, err)

				assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
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
	scrapers, provider := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, scrapers)
	// The receiver owns the shared pool; close it once the test is done.
	t.Cleanup(func() { assert.NoError(t, provider.close()) })

	for _, scraper := range scrapers {
		err := scraper.Start(t.Context(), componenttest.NewNopHost())
		assert.NoError(t, err)
		defer assert.NoError(t, scraper.Shutdown(t.Context()))

		scraper.client = mockClient{
			instanceName: scraper.config.InstanceName,
			SQL:          "Invalid SQL query",
		}

		actualMetrics, err := scraper.ScrapeMetrics(t.Context())
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
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetricsAndEvents(cfg, false)

	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	scrapers, _ := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, scrapers)

	scraper := scrapers[0]
	cached, val := scraper.cacheAndDiff("query_hash", "query_plan_hash", "procedure_id", "column", -1)
	assert.False(t, cached)
	assert.Equal(t, int64(0), val)

	cached, val = scraper.cacheAndDiff("query_hash", "query_plan_hash", "procedure_id", "column", 1)
	assert.False(t, cached)
	assert.Equal(t, int64(1), val)

	cached, val = scraper.cacheAndDiff("query_hash", "query_plan_hash", "procedure_id", "column", 1)
	assert.True(t, cached)
	assert.Equal(t, int64(0), val)

	cached, val = scraper.cacheAndDiff("query_hash", "query_plan_hash", "procedure_id", "column", 3)
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
		sortRows([]sqlquery.StringMap{{"abc": "12"}, {"ghi": "56"}, {"def": "34"}}, []int64{1, 2, 2}, 3),
	)

	assert.Equal(
		t,
		[]sqlquery.StringMap{{"ghi": "56"}, {"def": "34"}},
		sortRows([]sqlquery.StringMap{{"abc": "12"}, {"ghi": "56"}, {"def": "34"}}, []int64{1, 2, 2}, 2),
	)

	assert.Equal(
		t,
		[]sqlquery.StringMap{{"ghi": "56"}},
		sortRows([]sqlquery.StringMap{{"abc": "12"}, {"ghi": "56"}, {"def": "34"}}, []int64{1, 2, 2}, 1),
	)

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

var (
	_ sqlquery.DbClient = (*mockClient)(nil)
	_ sqlquery.DbClient = (*mockMultiStatementProcClient)(nil)
)

type mockClient struct {
	SQL                 string
	instanceName        string
	maxQuerySampleCount uint
	lookbackTime        uint
	topQueryCount       uint
	maxRowsPerQuery     uint64
	// propertiesFixtureFile, when set, overrides the fixture returned for the
	// server properties query. Used to exercise the reduced Azure SQL Managed
	// Instance column shape.
	propertiesFixtureFile string
}

type mockInvalidClient struct {
	mockClient
}

type mockMultiStatementProcClient struct {
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

func TestParseWaitResource(t *testing.T) {
	testCases := map[string]struct {
		input        string
		expectedType string
		expectedID   string
	}{
		"empty": {
			input:        "",
			expectedType: "",
			expectedID:   "",
		},
		"invalid": {
			input:        "not-a-wait-resource",
			expectedType: "",
			expectedID:   "",
		},
		"key": {
			input:        "KEY: 5:72057594043359232 (abc)",
			expectedType: "KEY",
			expectedID:   "72057594043359232",
		},
		"page": {
			input:        "PAGE: 7:1:232",
			expectedType: "PAGE",
			expectedID:   "1:232",
		},
		"rid": {
			input:        "RID: 7:1:232:3",
			expectedType: "RID",
			expectedID:   "1:232:3",
		},
		"object": {
			input:        "OBJECT: 9:245575913:0",
			expectedType: "OBJECT",
			expectedID:   "245575913:0",
		},
		"database": {
			input:        "DATABASE: 5",
			expectedType: "DATABASE",
			expectedID:   "5",
		},
		"file": {
			input:        "FILE: 5:2",
			expectedType: "FILE",
			expectedID:   "2",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			resourceType, resourceID := parseWaitResource(tc.input)
			assert.Equal(t, tc.expectedType, resourceType)
			assert.Equal(t, tc.expectedID, resourceID)
		})
	}
}

func (mc mockClient) QueryRows(context.Context, ...any) ([]sqlquery.StringMap, error) {
	var queryResults []sqlquery.StringMap
	var err error

	switch mc.SQL {
	case getSQLServerAvailabilityGroupQuery(mc.instanceName):
		queryResults, err = readFile("availabilityGroupQueryData.txt")
	case getSQLServerDatabaseIOQuery(mc.instanceName):
		queryResults, err = readFile("database_io_scraped_data.txt")
	case getSQLServerPerformanceCounterQuery(mc.instanceName):
		queryResults, err = readFile("perfCounterQueryData.txt")
	case getSQLServerPropertiesQuery(mc.instanceName):
		fixture := "propertyQueryData.txt"
		if mc.propertiesFixtureFile != "" {
			fixture = mc.propertiesFixtureFile
		}
		queryResults, err = readFile(fixture)
	case getSQLServerWaitStatsQuery(mc.instanceName):
		queryResults, err = readFile("waitStatsQueryData.txt")
	case getSQLServerWorkerThreadsQuery(mc.instanceName):
		queryResults, err = readFile("workerThreadsQueryData.txt")
	case getSQLServerIndexPhysicalStatsQuery(mc.instanceName):
		queryResults, err = readFile("indexPhysicalQueryData.txt")
	case getSQLServerCPUMemoryQuery(mc.instanceName):
		queryResults, err = readFile("cpuMemoryQueryData.txt")
	case getSQLServerDiskIOQuery(mc.instanceName):
		queryResults, err = readFile("diskIOQueryData.txt")
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

func (mc mockMultiStatementProcClient) QueryRows(context.Context, ...any) ([]sqlquery.StringMap, error) {
	switch mc.SQL {
	case getSQLServerQueryTextAndPlanQuery():
		return readFile("queryTextAndPlanMultiStatementProcData.txt")
	default:
		return nil, errors.New("No valid query found")
	}
}

func TestQueryTextAndPlanQueryMetricsShouldBeCachedSinceFirstCollection(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	enableSQLServerResourceAttributesForTests(&cfg.LogsBuilderConfig.ResourceAttributes)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetricsAndEvents(cfg, false)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.CollectionInterval = cfg.ControllerConfig.CollectionInterval

	scrapers, _ := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, scrapers)

	scraper := scrapers[0]
	assert.NotNil(t, scraper.cache)

	const totalElapsedTime = "total_elapsed_time"
	const rowsReturned = "total_rows"
	const totalWorkerTime = "total_worker_time"
	const logicalReads = "total_logical_reads"
	const physicalReads = "total_physical_reads"
	const executionCount = "execution_count"
	const totalGrant = "total_grant_kb"
	const procedureExecutionCount = "procedure_execution_count"

	scraper.client = mockClient{
		instanceName:        scraper.config.InstanceName,
		SQL:                 scraper.sqlQuery,
		maxQuerySampleCount: 1000,
		lookbackTime:        20,
		topQueryCount:       200,
	}

	_, err := scraper.ScrapeLogs(t.Context())
	assert.NoError(t, err)

	expectedFile := filepath.Join("testdata", "expectedQueryTextAndPlanQuery.yaml")
	expectedLogs, _ := golden.ReadLogs(expectedFile)

	queryHash, _ := expectedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("sqlserver.query_hash")
	planHash, _ := expectedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("sqlserver.query_plan_hash")
	keyPrefix := queryHash.Str() + "-" + planHash.Str()

	tetValue, ok := scraper.cache.Get(keyPrefix + "-" + totalElapsedTime)
	assert.True(t, ok, "Expected to find elapsed time in cache right after the first collection")
	assert.Equal(t, 3846, int(tetValue))

	rtValue, ok := scraper.cache.Get(keyPrefix + "-" + rowsReturned)
	assert.True(t, ok, "Expected to find rowsReturned in cache right after the first collection")
	assert.Equal(t, 2, int(rtValue))

	twtValue, ok := scraper.cache.Get(keyPrefix + "-" + totalWorkerTime)
	assert.True(t, ok, "Expected to find totalWorkerTime in cache right after the first collection")
	assert.Equal(t, 3845, int(twtValue))

	lrValue, ok := scraper.cache.Get(keyPrefix + "-" + logicalReads)
	assert.True(t, ok, "Expected to find logicalReads in cache right after the first collection")
	assert.Equal(t, 3, int(lrValue))

	prValue, ok := scraper.cache.Get(keyPrefix + "-" + physicalReads)
	assert.True(t, ok, "Expected to find physicalReads in cache right after the first collection")
	assert.Equal(t, 5, int(prValue))

	ecValue, ok := scraper.cache.Get(keyPrefix + "-" + executionCount)
	assert.True(t, ok, "Expected to find executionCount in cache right after the first collection")
	assert.Equal(t, 6, int(ecValue))

	tgValue, ok := scraper.cache.Get(keyPrefix + "-" + totalGrant)
	assert.True(t, ok, "Expected to find totalGrant in cache right after the first collection")
	assert.Equal(t, 3096, int(tgValue))

	pecValue, ok := scraper.cache.Get(keyPrefix + "-" + procedureExecutionCount)
	assert.True(t, ok, "Expected to find procedureExecutionCount in cache right after the first collection")
	assert.Equal(t, 0, int(pecValue))
}

func TestQueryTextAndPlanQuery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	enableSQLServerResourceAttributesForTests(&cfg.LogsBuilderConfig.ResourceAttributes)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetricsAndEvents(cfg, false)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.CollectionInterval = cfg.ControllerConfig.CollectionInterval

	scrapers, _ := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
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
	const procedureExecutionCount = "procedure_execution_count"

	queryHash := hex.EncodeToString([]byte("0x37849E874171E3F3"))
	queryPlanHash := hex.EncodeToString([]byte("0xD3112909429A1B50"))
	procedureID := "0"
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, totalElapsedTime, 846)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, rowsReturned, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, logicalReads, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, logicalWrites, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, physicalReads, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, executionCount, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, totalWorkerTime, 845)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, totalGrant, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, procedureExecutionCount, 0)

	scraper.client = mockClient{
		instanceName:        scraper.config.InstanceName,
		SQL:                 scraper.sqlQuery,
		maxQuerySampleCount: 1000,
		lookbackTime:        20,
		topQueryCount:       200,
	}

	actualLogs, err := scraper.ScrapeLogs(t.Context())
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
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetricsAndEvents(cfg, false)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true

	scrapers, _ := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
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
	const procedureExecutionCount = "procedure_execution_count"

	queryHash := hex.EncodeToString([]byte("0x37849E874171E3F3"))
	queryPlanHash := hex.EncodeToString([]byte("0xD3112909429A1B50"))
	procedureID := "0"
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, totalElapsedTime, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, rowsReturned, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, logicalReads, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, logicalWrites, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, physicalReads, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, executionCount, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, totalWorkerTime, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, totalGrant, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, procedureExecutionCount, 0)

	scraper.client = mockInvalidClient{
		mockClient: mockClient{
			instanceName:        scraper.config.InstanceName,
			SQL:                 scraper.sqlQuery,
			maxQuerySampleCount: 1000,
			lookbackTime:        20,
		},
	}

	actualLogs, err := scraper.ScrapeLogs(t.Context())
	assert.Error(t, err)

	assert.Zero(t, actualLogs.LogRecordCount(), "If the metrics does not hold meaningful values then those records need not be exported by the receiver")
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
			enableSQLServerResourceAttributesForTests(&cfg.LogsBuilderConfig.ResourceAttributes)
			assert.NoError(t, cfg.Validate())

			configureAllScraperMetricsAndEvents(cfg, false)
			cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true

			scrapers, _ := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
			assert.NotNil(t, scrapers)

			scraper := scrapers[0]
			assert.NotNil(t, scraper.cache)

			scraper.client = tc.mockClient(scraper.instanceName, scraper.sqlQuery)

			actualLogs, err := scraper.ScrapeLogs(t.Context())
			if tc.errors {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			logRecord := actualLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
			blockingSessionIDAttr, ok := logRecord.Attributes().Get("sqlserver.blocking_session_id")
			assert.True(t, ok)
			blockingStartTimeAttr, hasBlockingStartTime := logRecord.Attributes().Get("sqlserver.blocking.start_time")
			assert.Equal(t, blockingSessionIDAttr.Int() > 0, hasBlockingStartTime)
			if hasBlockingStartTime {
				_, parseErr := time.Parse(time.RFC3339, blockingStartTimeAttr.Str())
				assert.NoError(t, parseErr)
			}

			// Uncomment line below to re-generate expected logs.
			// golden.WriteLogs(t, filepath.Join("testdata", tc.expectedFile), actualLogs)
			expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", tc.expectedFile))
			assert.NoError(t, err)
			removeAttributeFromAllLogRecords(expectedLogs, "sqlserver.blocking.start_time")
			removeAttributeFromAllLogRecords(actualLogs, "sqlserver.blocking.start_time")
			errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreTimestamp())
			assert.Equal(t, "db.server.query_sample", logRecord.EventName())
			assert.NoError(t, errs)
		})
	}
}

func removeAttributeFromAllLogRecords(logs plog.Logs, key string) {
	resourceLogs := logs.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		scopeLogs := resourceLogs.At(i).ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			logRecords := scopeLogs.At(j).LogRecords()
			for k := 0; k < logRecords.Len(); k++ {
				logRecords.At(k).Attributes().Remove(key)
			}
		}
	}
}

type queryRowsFuncClient struct {
	queryRowsFunc func(context.Context, ...any) ([]sqlquery.StringMap, error)
}

func (c queryRowsFuncClient) QueryRows(ctx context.Context, args ...any) ([]sqlquery.StringMap, error) {
	return c.queryRowsFunc(ctx, args...)
}

func setupQuerySampleScraper(t *testing.T, logger *zap.Logger) *sqlServerScraperHelper {
	t.Helper()

	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	enableSQLServerResourceAttributesForTests(&cfg.LogsBuilderConfig.ResourceAttributes)
	configureAllScraperMetricsAndEvents(cfg, false)
	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
	assert.NoError(t, cfg.Validate())

	settings := receivertest.NewNopSettings(metadata.Type)
	if logger != nil {
		settings.Logger = logger
	}

	scrapers, _ := setupSQLServerLogsScrapers(settings, cfg)
	assert.Len(t, scrapers, 1)
	return scrapers[0]
}

func buildQuerySampleRow(sessionID, blockingSessionID, command, statement string) sqlquery.StringMap {
	return sqlquery.StringMap{
		"sql_instance":                "sqlserver",
		"computer_name":               "DESKTOP-GHAEGRD",
		"db_name":                     "master",
		"client_address":              "172.19.0.1",
		"client_port":                 "59286",
		"query_start":                 "2025-02-12T16:37:54.843+08:00",
		"session_id":                  sessionID,
		"session_status":              "running",
		"request_status":              "running",
		"host_name":                   "DESKTOP-GHAEGRD",
		"command":                     command,
		"statement_text":              statement,
		"blocking_session_id":         blockingSessionID,
		"wait_type":                   "",
		"wait_time":                   "0",
		"wait_resource":               "",
		"open_transaction_count":      "0",
		"transaction_id":              "11089",
		"percent_complete":            "0",
		"estimated_completion_time":   "0",
		"cpu_time":                    "6",
		"total_elapsed_time":          "6",
		"reads":                       "0",
		"writes":                      "0",
		"logical_reads":               "38",
		"transaction_isolation_level": "2",
		"lock_timeout":                "-1",
		"deadlock_priority":           "0",
		"row_count":                   "1",
		"query_hash":                  "0x70A3B130B1048D4D",
		"query_plan_hash":             "0x140210F64B788CB9",
		"context_info":                "",
		"username":                    "sa",
		"client_app_name":             "SSMS",
		"session_start_time":          "2025-02-12T15:00:00.000+08:00",
		"session_duration":            "720456",
		"procedure_id":                "0",
		"procedure_name":              "",
		"blocking_start_time":         "",
	}
}

func buildIdleBlockerRow(sessionID string) sqlquery.StringMap {
	row := buildQuerySampleRow(sessionID, "0", "IDLE_BLOCKER", "SELECT 1")
	row["session_status"] = "sleeping"
	row["request_status"] = ""
	return row
}

func TestRecordDatabaseSampleQueryFetchesIdleBlockers(t *testing.T) {
	scraper := setupQuerySampleScraper(t, nil)
	scraper.db = &sql.DB{}

	mainRows := []sqlquery.StringMap{buildQuerySampleRow("60", "77", "SELECT", "SELECT 2")}
	idleRows := []sqlquery.StringMap{buildIdleBlockerRow("77")}

	idleQueryCalls := 0
	scraper.client = queryRowsFuncClient{queryRowsFunc: func(context.Context, ...any) ([]sqlquery.StringMap, error) {
		return mainRows, nil
	}}
	scraper.clientProviderFunc = func(_ sqlquery.Db, query string, _ *zap.Logger, _ sqlquery.TelemetryConfig) sqlquery.DbClient {
		expectedQuery := fmt.Sprintf(getSQLServerIdleBlockingSessionsQuery(), "77")
		assert.Equal(t, expectedQuery, query)
		return queryRowsFuncClient{queryRowsFunc: func(_ context.Context, args ...any) ([]sqlquery.StringMap, error) {
			idleQueryCalls++
			assert.Equal(t, []any{
				sql.Named("top", scraper.config.QuerySample.MaxRowsPerQuery),
			}, args)
			return idleRows, nil
		}}
	}

	actualLogs, err := scraper.ScrapeLogs(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, 2, actualLogs.LogRecordCount())
	assert.Equal(t, 1, idleQueryCalls)

	foundIdleBlocker := false
	records := actualLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	for i := 0; i < records.Len(); i++ {
		attrs := records.At(i).Attributes()
		command, ok := attrs.Get("sqlserver.command")
		if ok && command.Str() == "IDLE_BLOCKER" {
			foundIdleBlocker = true
			sessionID, hasSessionID := attrs.Get("sqlserver.session_id")
			assert.True(t, hasSessionID)
			assert.Equal(t, int64(77), sessionID.Int())
			_, hasBlockingStartTime := attrs.Get("sqlserver.blocking.start_time")
			assert.False(t, hasBlockingStartTime)
		}
	}
	assert.True(t, foundIdleBlocker)
}

func TestRecordDatabaseSampleQueryDoesNotFetchIdleBlockersWhenNoneMissing(t *testing.T) {
	scraper := setupQuerySampleScraper(t, nil)
	scraper.db = &sql.DB{}

	mainRows := []sqlquery.StringMap{
		buildQuerySampleRow("60", "61", "SELECT", "SELECT 2"),
		buildQuerySampleRow("61", "0", "SELECT", "SELECT 3"),
	}

	providerCalls := 0
	scraper.client = queryRowsFuncClient{queryRowsFunc: func(context.Context, ...any) ([]sqlquery.StringMap, error) {
		return mainRows, nil
	}}
	scraper.clientProviderFunc = func(_ sqlquery.Db, _ string, _ *zap.Logger, _ sqlquery.TelemetryConfig) sqlquery.DbClient {
		providerCalls++
		return queryRowsFuncClient{queryRowsFunc: func(context.Context, ...any) ([]sqlquery.StringMap, error) {
			return nil, nil
		}}
	}

	actualLogs, err := scraper.ScrapeLogs(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, 2, actualLogs.LogRecordCount())
	assert.Equal(t, 0, providerCalls)
}

func TestRecordDatabaseSampleQueryIdleBlockerQueryFailureDoesNotFailScrape(t *testing.T) {
	core, observedLogs := observer.New(zap.WarnLevel)
	scraper := setupQuerySampleScraper(t, zap.New(core))
	scraper.db = &sql.DB{}

	mainRows := []sqlquery.StringMap{buildQuerySampleRow("60", "77", "SELECT", "SELECT 2")}

	idleQueryCalls := 0
	scraper.client = queryRowsFuncClient{queryRowsFunc: func(context.Context, ...any) ([]sqlquery.StringMap, error) {
		return mainRows, nil
	}}
	scraper.clientProviderFunc = func(_ sqlquery.Db, query string, _ *zap.Logger, _ sqlquery.TelemetryConfig) sqlquery.DbClient {
		expectedQuery := fmt.Sprintf(getSQLServerIdleBlockingSessionsQuery(), "77")
		assert.Equal(t, expectedQuery, query)
		return queryRowsFuncClient{queryRowsFunc: func(_ context.Context, args ...any) ([]sqlquery.StringMap, error) {
			idleQueryCalls++
			assert.Equal(t, []any{
				sql.Named("top", scraper.config.QuerySample.MaxRowsPerQuery),
			}, args)
			return nil, errors.New("idle blocker query failed")
		}}
	}

	actualLogs, err := scraper.ScrapeLogs(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, 1, actualLogs.LogRecordCount())
	assert.Equal(t, 1, idleQueryCalls)
	assert.Equal(t, 1, observedLogs.FilterMessageSnippet("problems encountered getting idle blocker log rows").Len())
}

// TestMultiStatementProcNoDuplicateRows validates that a stored procedure
// containing multiple SELECT statements (each with a distinct query_hash /
// query_plan_hash but sharing the same plan_handle) produces exactly one
// log record per statement -- not duplicated rows caused by a 1:N join on
// plan_handle alone.
func TestMultiStatementProcNoDuplicateRows(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetricsAndEvents(cfg, false)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.CollectionInterval = cfg.ControllerConfig.CollectionInterval

	scrapers, _ := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, scrapers)

	scraper := scrapers[0]
	assert.NotNil(t, scraper.cache)

	// Seed the cache so that cacheAndDiff returns non-zero diffs (simulates a
	// prior scrape). Use the hex-encoded query_hash values from the mock data.
	stmt1Hash := hex.EncodeToString([]byte("0xAAAAAAAAAAAAAAAA"))
	stmt1PlanHash := hex.EncodeToString([]byte("0xBBBBBBBBBBBBBBBB"))
	stmt2Hash := hex.EncodeToString([]byte("0xCCCCCCCCCCCCCCCC"))
	stmt2PlanHash := hex.EncodeToString([]byte("0xDDDDDDDDDDDDDDDD"))
	procID := "1431676148"

	for _, pair := range [][2]string{{stmt1Hash, stmt1PlanHash}, {stmt2Hash, stmt2PlanHash}} {
		scraper.cacheAndDiff(pair[0], pair[1], procID, "execution_count", 1)
		scraper.cacheAndDiff(pair[0], pair[1], procID, "total_elapsed_time", 1)
		scraper.cacheAndDiff(pair[0], pair[1], procID, "total_grant_kb", 1)
		scraper.cacheAndDiff(pair[0], pair[1], procID, "total_logical_reads", 1)
		scraper.cacheAndDiff(pair[0], pair[1], procID, "total_logical_writes", 1)
		scraper.cacheAndDiff(pair[0], pair[1], procID, "total_physical_reads", 1)
		scraper.cacheAndDiff(pair[0], pair[1], procID, "total_rows", 1)
		scraper.cacheAndDiff(pair[0], pair[1], procID, "total_worker_time", 1)
	}

	scraper.client = mockMultiStatementProcClient{
		mockClient: mockClient{
			instanceName:        scraper.config.InstanceName,
			SQL:                 scraper.sqlQuery,
			maxQuerySampleCount: 1000,
			lookbackTime:        20,
			topQueryCount:       200,
		},
	}

	actualLogs, err := scraper.ScrapeLogs(t.Context())
	assert.NoError(t, err)

	// The mock data contains exactly 2 rows (two distinct statements inside one
	// stored procedure sharing a single plan_handle). Before the fix, a join on
	// plan_handle alone would fan these into 4 rows. After the fix the join
	// additionally matches on query_hash + query_plan_hash, keeping the count
	// at 2. Verify we get exactly 2 log records.
	assert.Equal(t, 2, actualLogs.LogRecordCount(),
		"Expected exactly 2 log records for 2 distinct statements; duplicates indicate the plan_handle join is too broad")

	// Verify both records are top_query events.
	scopeLogs := actualLogs.ResourceLogs().At(0).ScopeLogs().At(0)
	for i := 0; i < scopeLogs.LogRecords().Len(); i++ {
		assert.Equal(t, "db.server.top_query", scopeLogs.LogRecords().At(i).EventName())
	}

	// Collect query_hash attribute values and verify they are distinct.
	seenHashes := make(map[string]bool)
	for i := 0; i < scopeLogs.LogRecords().Len(); i++ {
		qh, ok := scopeLogs.LogRecords().At(i).Attributes().Get("sqlserver.query_hash")
		assert.True(t, ok)
		seenHashes[qh.Str()] = true
	}
	assert.Len(t, seenHashes, 2,
		"Expected 2 distinct query_hash values, got duplicates")
}

func TestSetupResourceBuilder(t *testing.T) {
	tests := []struct {
		name             string
		config           *Config
		expectedHostName string
	}{
		{
			name: "with server configuration",
			config: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Server = "testserver.example.com"
				cfg.Port = 1433
				cfg.MetricsBuilderConfig.ResourceAttributes.HostName.Enabled = true
				return cfg
			}(),
			expectedHostName: "testserver.example.com",
		},
		{
			name: "with datasource configuration",
			config: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.DataSource = "sqlserver://testuser:testpass@datasource-host.example.com:1434?database=testdb"
				cfg.MetricsBuilderConfig.ResourceAttributes.HostName.Enabled = true
				return cfg
			}(),
			expectedHostName: "datasource-host.example.com",
		},
		{
			name: "with datasource default port",
			config: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.DataSource = "sqlserver://testuser:testpass@datasource-host2.example.com?database=testdb"
				cfg.MetricsBuilderConfig.ResourceAttributes.HostName.Enabled = true
				return cfg
			}(),
			expectedHostName: "datasource-host2.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := receivertest.NewNopSettings(metadata.Type)
			scraper := newSQLServerScraper(
				settings.ID,
				"SELECT 1",
				sqlquery.TelemetryConfig{},
				func() (*sql.DB, error) { return nil, nil },
				func(_ sqlquery.Db, _ string, _ *zap.Logger, _ sqlquery.TelemetryConfig) sqlquery.DbClient {
					return nil
				},
				settings,
				tt.config,
				nil,
			)
			scraper.mb = metadata.NewMetricsBuilder(tt.config.MetricsBuilderConfig, settings)

			row := sqlquery.StringMap{
				computerNameKey: "test-computer",
				instanceNameKey: "test-instance",
			}

			rb := scraper.setupResourceBuilder(scraper.mb.NewResourceBuilder(), row)
			resource := rb.Emit()

			hostName, exists := resource.Attributes().Get("host.name")
			assert.True(t, exists)
			assert.Equal(t, tt.expectedHostName, hostName.AsString())
		})
	}
}

func TestRecordDatabaseSampleQueryUsesResourceBuilderForLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.DataSource = "sqlserver://testuser:testpass@datasource-host.example.com:1434?database=testdb"
	enableSQLServerResourceAttributesForTests(&cfg.LogsBuilderConfig.ResourceAttributes)
	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetricsAndEvents(cfg, false)
	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true

	scrapers, _ := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.Len(t, scrapers, 1)

	scraper := scrapers[0]
	scraper.client = mockClient{
		instanceName:    scraper.config.InstanceName,
		SQL:             scraper.sqlQuery,
		maxRowsPerQuery: 100,
	}

	actualLogs, err := scraper.ScrapeLogs(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, 1, actualLogs.ResourceLogs().Len())

	resourceAttributes := actualLogs.ResourceLogs().At(0).Resource().Attributes()
	hostName, exists := resourceAttributes.Get("host.name")
	assert.True(t, exists)
	assert.Equal(t, "datasource-host.example.com", hostName.AsString())

	serviceInstanceID, exists := resourceAttributes.Get("service.instance.id")
	assert.True(t, exists)
	assert.Equal(t, "datasource-host.example.com:1434", serviceInstanceID.AsString())

	computerName, exists := resourceAttributes.Get("sqlserver.computer.name")
	assert.True(t, exists)
	assert.Equal(t, "DESKTOP-GHAEGRD", computerName.AsString())

	instanceName, exists := resourceAttributes.Get("sqlserver.instance.name")
	assert.True(t, exists)
	assert.Equal(t, "sqlserver", instanceName.AsString())
}

func TestRecordDatabaseQueryTextAndPlanUsesResourceBuilderForLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.DataSource = "sqlserver://testuser:testpass@datasource-host.example.com:1434?database=testdb"
	enableSQLServerResourceAttributesForTests(&cfg.LogsBuilderConfig.ResourceAttributes)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetricsAndEvents(cfg, false)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.CollectionInterval = cfg.ControllerConfig.CollectionInterval

	scrapers, _ := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.Len(t, scrapers, 1)

	scraper := scrapers[0]
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
	procedureID := "0"
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, totalElapsedTime, 846)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, rowsReturned, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, logicalReads, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, logicalWrites, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, physicalReads, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, executionCount, 1)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, totalWorkerTime, 845)
	scraper.cacheAndDiff(queryHash, queryPlanHash, procedureID, totalGrant, 1)

	scraper.client = mockClient{
		instanceName:        scraper.config.InstanceName,
		SQL:                 scraper.sqlQuery,
		maxQuerySampleCount: 1000,
		lookbackTime:        20,
		topQueryCount:       200,
	}

	actualLogs, err := scraper.ScrapeLogs(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, 1, actualLogs.ResourceLogs().Len())

	resourceAttributes := actualLogs.ResourceLogs().At(0).Resource().Attributes()
	hostName, exists := resourceAttributes.Get("host.name")
	assert.True(t, exists)
	assert.Equal(t, "datasource-host.example.com", hostName.AsString())

	serviceInstanceID, exists := resourceAttributes.Get("service.instance.id")
	assert.True(t, exists)
	assert.Equal(t, "datasource-host.example.com:1434", serviceInstanceID.AsString())

	computerName, exists := resourceAttributes.Get("sqlserver.computer.name")
	assert.True(t, exists)
	assert.Equal(t, "DESKTOP-GHAEGRD", computerName.AsString())

	instanceName, exists := resourceAttributes.Get("sqlserver.instance.name")
	assert.True(t, exists)
	assert.Equal(t, "sqlserver", instanceName.AsString())

	serverAddress, exists := resourceAttributes.Get("server.address")
	assert.True(t, exists)
	assert.Equal(t, "datasource-host.example.com", serverAddress.AsString())

	serverPort, exists := resourceAttributes.Get("server.port")
	assert.True(t, exists)
	assert.Equal(t, int64(1434), serverPort.Int())
}

func TestRecordWorkerThreadMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Server = "0.0.0.0"
	cfg.Port = 1433
	assert.NoError(t, cfg.Validate())
	cfg.MetricsBuilderConfig.Metrics.SqlserverWorkerThreadCount.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverWorkerRequestCount.Enabled = true

	scrapers, provider := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotEmpty(t, scrapers)

	var workerScraper *sqlServerScraperHelper
	for _, s := range scrapers {
		if s.sqlQuery == getSQLServerWorkerThreadsQuery(cfg.InstanceName) {
			workerScraper = s
			break
		}
	}
	assert.NotNil(t, workerScraper, "worker threads scraper should be present")

	err := workerScraper.Start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, workerScraper.Shutdown(t.Context()))
		// The receiver owns the shared pool; close it here in the test.
		assert.NoError(t, provider.close())
	}()

	workerScraper.client = mockClient{
		instanceName: workerScraper.config.InstanceName,
		SQL:          workerScraper.sqlQuery,
	}

	actualMetrics, err := workerScraper.ScrapeMetrics(t.Context())
	assert.NoError(t, err)

	// Verify all five data points: 4 worker.state variants + 1 worker.request.waiting
	var totalDP int
	for i := 0; i < actualMetrics.ResourceMetrics().Len(); i++ {
		rm := actualMetrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				switch m.Name() {
				case metadata.MetricsInfo.SqlserverWorkerThreadCount.Name:
					totalDP += m.Gauge().DataPoints().Len()
				case metadata.MetricsInfo.SqlserverWorkerRequestCount.Name:
					totalDP += m.Gauge().DataPoints().Len()
				}
			}
		}
	}
	assert.Equal(t, 5, totalDP)
}

func TestIsWorkerThreadsQueryEnabled(t *testing.T) {
	assert.False(t, isWorkerThreadsQueryEnabled(nil))

	metrics := &metadata.MetricsConfig{}
	assert.False(t, isWorkerThreadsQueryEnabled(metrics))

	metrics.SqlserverWorkerThreadCount.Enabled = true
	assert.True(t, isWorkerThreadsQueryEnabled(metrics))

	metrics.SqlserverWorkerThreadCount.Enabled = false
	metrics.SqlserverWorkerRequestCount.Enabled = true
	assert.True(t, isWorkerThreadsQueryEnabled(metrics))
}

func TestRecordDatabaseStatusMetricsUsesResourceBuilderForMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.DataSource = "sqlserver://testuser:testpass@datasource-host.example.com:1434?database=testdb"
	enableSQLServerResourceAttributesForTests(&cfg.MetricsBuilderConfig.ResourceAttributes)
	cfg.MetricsBuilderConfig.Metrics.SqlserverCPUCount.Enabled = true
	assert.NoError(t, cfg.Validate())

	configureAllScraperMetricsAndEvents(cfg, false)
	cfg.MetricsBuilderConfig.Metrics.SqlserverCPUCount.Enabled = true

	scrapers, _ := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.Len(t, scrapers, 1)

	scraper := scrapers[0]
	scraper.client = mockClient{
		instanceName: scraper.config.InstanceName,
		SQL:          scraper.sqlQuery,
	}

	actualMetrics, err := scraper.ScrapeMetrics(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, 1, actualMetrics.ResourceMetrics().Len())

	resourceAttributes := actualMetrics.ResourceMetrics().At(0).Resource().Attributes()
	hostName, exists := resourceAttributes.Get("host.name")
	assert.True(t, exists)
	assert.Equal(t, "datasource-host.example.com", hostName.AsString())

	serviceInstanceID, exists := resourceAttributes.Get("service.instance.id")
	assert.True(t, exists)
	assert.Equal(t, "datasource-host.example.com:1434", serviceInstanceID.AsString())

	computerName, exists := resourceAttributes.Get("sqlserver.computer.name")
	assert.True(t, exists)
	assert.Equal(t, "abcde", computerName.AsString())

	instanceName, exists := resourceAttributes.Get("sqlserver.instance.name")
	assert.True(t, exists)
	assert.Equal(t, "ad8fb2b53dce", instanceName.AsString())

	serverAddress, exists := resourceAttributes.Get("server.address")
	assert.True(t, exists)
	assert.Equal(t, "datasource-host.example.com", serverAddress.AsString())

	serverPort, exists := resourceAttributes.Get("server.port")
	assert.True(t, exists)
	assert.Equal(t, int64(1434), serverPort.Int())
}

func TestRecordCPUMemoryMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Server = "0.0.0.0"
	cfg.Port = 1433
	assert.NoError(t, cfg.Validate())
	cfg.MetricsBuilderConfig.Metrics.SqlserverCPUUtilization.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverHostMemoryLimit.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverHostMemoryUsage.Enabled = true

	scrapers, provider := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotEmpty(t, scrapers)
	defer func() { assert.NoError(t, provider.close()) }()

	var cpuMemScraper *sqlServerScraperHelper
	for _, s := range scrapers {
		if s.sqlQuery == getSQLServerCPUMemoryQuery(cfg.InstanceName) {
			cpuMemScraper = s
			break
		}
	}
	assert.NotNil(t, cpuMemScraper, "cpu memory scraper should be present")

	err := cpuMemScraper.Start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)
	defer assert.NoError(t, cpuMemScraper.Shutdown(t.Context()))

	cpuMemScraper.client = mockClient{
		instanceName: cpuMemScraper.config.InstanceName,
		SQL:          cpuMemScraper.sqlQuery,
	}

	actualMetrics, err := cpuMemScraper.ScrapeMetrics(t.Context())
	assert.NoError(t, err)

	// Verify 4 data points: 1 cpu utilization + 1 memory limit + 2 memory usage (used, free)
	var totalDP int
	var cpuVal, limitVal, usedVal, freeVal float64
	var sawCPU, sawLimit, sawUsed, sawFree bool
	for i := 0; i < actualMetrics.ResourceMetrics().Len(); i++ {
		rm := actualMetrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				switch m.Name() {
				case metadata.MetricsInfo.SqlserverCPUUtilization.Name:
					dps := m.Gauge().DataPoints()
					totalDP += dps.Len()
					for d := 0; d < dps.Len(); d++ {
						cpuVal = dps.At(d).DoubleValue()
						sawCPU = true
					}
				case metadata.MetricsInfo.SqlserverHostMemoryLimit.Name:
					dps := m.Gauge().DataPoints()
					totalDP += dps.Len()
					for d := 0; d < dps.Len(); d++ {
						limitVal = float64(dps.At(d).IntValue())
						sawLimit = true
					}
				case metadata.MetricsInfo.SqlserverHostMemoryUsage.Name:
					dps := m.Gauge().DataPoints()
					totalDP += dps.Len()
					for d := 0; d < dps.Len(); d++ {
						state, _ := dps.At(d).Attributes().Get("system.memory.state")
						switch state.Str() {
						case "used":
							usedVal = float64(dps.At(d).IntValue())
							sawUsed = true
						case "free":
							freeVal = float64(dps.At(d).IntValue())
							sawFree = true
						}
					}
				}
			}
		}
	}
	assert.Equal(t, 4, totalDP)
	assert.True(t, sawCPU, "cpu utilization data point should be emitted")
	assert.True(t, sawLimit, "host memory limit data point should be emitted")
	assert.True(t, sawUsed, "host memory usage used data point should be emitted")
	assert.True(t, sawFree, "host memory usage free data point should be emitted")
	assert.InDelta(t, 0.425, cpuVal, 0.0001)
	assert.Equal(t, float64(17179869184), limitVal)
	assert.Equal(t, float64(8589934592), usedVal)
	assert.Equal(t, float64(8589934592), freeVal)
}

func TestRecordDiskIOMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Server = "0.0.0.0"
	cfg.Port = 1433
	assert.NoError(t, cfg.Validate())
	cfg.MetricsBuilderConfig.Metrics.SqlserverDiskOperations.Enabled = true
	cfg.MetricsBuilderConfig.Metrics.SqlserverDiskIo.Enabled = true

	scrapers, provider := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotEmpty(t, scrapers)
	defer func() { assert.NoError(t, provider.close()) }()

	var diskScraper *sqlServerScraperHelper
	for _, s := range scrapers {
		if s.sqlQuery == getSQLServerDiskIOQuery(cfg.InstanceName) {
			diskScraper = s
			break
		}
	}
	assert.NotNil(t, diskScraper, "disk io scraper should be present")

	err := diskScraper.Start(t.Context(), componenttest.NewNopHost())
	assert.NoError(t, err)
	defer assert.NoError(t, diskScraper.Shutdown(t.Context()))

	diskScraper.client = mockClient{
		instanceName: diskScraper.config.InstanceName,
		SQL:          diskScraper.sqlQuery,
	}

	actualMetrics, err := diskScraper.ScrapeMetrics(t.Context())
	assert.NoError(t, err)

	// 3 drives x (2 directions for operations + 2 directions for bytes) = 12 data points
	var totalDP int
	for i := 0; i < actualMetrics.ResourceMetrics().Len(); i++ {
		rm := actualMetrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				switch m.Name() {
				case metadata.MetricsInfo.SqlserverDiskOperations.Name:
					totalDP += m.Sum().DataPoints().Len()
				case metadata.MetricsInfo.SqlserverDiskIo.Name:
					totalDP += m.Sum().DataPoints().Len()
				}
			}
		}
	}
	assert.Equal(t, 12, totalDP)
}

func TestIsCPUMemoryQueryEnabled(t *testing.T) {
	assert.False(t, isCPUMemoryQueryEnabled(nil))

	metrics := &metadata.MetricsConfig{}
	assert.False(t, isCPUMemoryQueryEnabled(metrics))

	metrics.SqlserverCPUUtilization.Enabled = true
	assert.True(t, isCPUMemoryQueryEnabled(metrics))

	metrics.SqlserverCPUUtilization.Enabled = false
	metrics.SqlserverHostMemoryLimit.Enabled = true
	assert.True(t, isCPUMemoryQueryEnabled(metrics))

	metrics.SqlserverHostMemoryLimit.Enabled = false
	metrics.SqlserverHostMemoryUsage.Enabled = true
	assert.True(t, isCPUMemoryQueryEnabled(metrics))
}

func TestIsDiskIOQueryEnabled(t *testing.T) {
	assert.False(t, isDiskIOQueryEnabled(nil))

	metrics := &metadata.MetricsConfig{}
	assert.False(t, isDiskIOQueryEnabled(metrics))

	metrics.SqlserverDiskOperations.Enabled = true
	assert.True(t, isDiskIOQueryEnabled(metrics))

	metrics.SqlserverDiskOperations.Enabled = false
	metrics.SqlserverDiskIo.Enabled = true
	assert.True(t, isDiskIOQueryEnabled(metrics))
}
