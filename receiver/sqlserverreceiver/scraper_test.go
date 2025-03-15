// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	cfg.MetricsBuilderConfig.Metrics.SqlserverUserConnectionCount.Enabled = enabled

	cfg.EnableQuerySample = enabled
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
	cfg.MetricsBuilderConfig.Metrics.SqlserverBatchRequestRate.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverPageBufferCacheHitRatio.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverLockWaitRate.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverBatchSQLRecompilationRate.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverBatchSQLCompilationRate.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SqlserverUserConnectionCount.Enabled = false
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

	enableAllScraperMetrics(cfg, true)

	scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotEmpty(t, scrapers)

	for _, scraper := range scrapers {
		err := scraper.Start(context.Background(), componenttest.NewNopHost())
		assert.NoError(t, err)
		defer assert.NoError(t, scraper.Shutdown(context.Background()))

		scraper.client = mockClient{
			instanceName: scraper.config.InstanceName,
			SQL:          scraper.sqlQuery,
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

	enableAllScraperMetrics(cfg, true)
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

var _ sqlquery.DbClient = (*mockClient)(nil)

type mockClient struct {
	SQL               string
	instanceName      string
	maxResultPerQuery uint64
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
	case getSQLServerQuerySamplesQuery(mc.maxResultPerQuery):
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
	case getSQLServerQuerySamplesQuery(mc.maxResultPerQuery):
		queryResults, err = readFile("recordInvalidDatabaseSampleQueryData.txt")
	default:
		return nil, errors.New("No valid query found")
	}

	if err != nil {
		return nil, err
	}
	return queryResults, nil
}

func TestRecordDatabaseSampleQuery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	assert.NoError(t, cfg.Validate())

	enableAllScraperMetrics(cfg, false)
	cfg.EnableQuerySample = true

	scrapers := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, scrapers)

	scraper := scrapers[0]
	assert.NotNil(t, scraper.cache)

	scraper.client = mockClient{
		instanceName:      scraper.instanceName,
		SQL:               scraper.sqlQuery,
		maxResultPerQuery: 100,
	}

	actualLogs, err := scraper.ScrapeLogs(context.Background())
	assert.NoError(t, err)

	expectedLogs, _ := golden.ReadLogs(filepath.Join("testdata", "expectedRecordDatabaseSampleQuery.yaml"))
	errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreTimestamp())

	assert.NoError(t, errs)
}

func TestRecordInvalidDatabaseSampleQuery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "sa"
	cfg.Password = "password"
	cfg.Port = 1433
	cfg.Server = "0.0.0.0"
	cfg.MetricsBuilderConfig.ResourceAttributes.SqlserverInstanceName.Enabled = true
	assert.NoError(t, cfg.Validate())

	enableAllScraperMetrics(cfg, false)
	cfg.EnableQuerySample = true

	scrapers := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, scrapers)

	scraper := scrapers[0]
	assert.NotNil(t, scraper.cache)

	scraper.client = mockInvalidClient{
		mockClient{
			instanceName:      scraper.instanceName,
			SQL:               scraper.sqlQuery,
			maxResultPerQuery: 100,
		},
	}

	actualLogs, err := scraper.ScrapeLogs(context.Background())
	assert.NoError(t, err)
	expectedLogs, _ := golden.ReadLogs(filepath.Join("testdata", "expectedRecordDatabaseSampleQueryWithInvalidData.yaml"))
	errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreTimestamp())
	assert.NoError(t, errs)
}
