// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

func TestUnsuccessfulScrape(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "fake:11111"

	scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newDefaultClientFactory(cfg), newCache(1), newTTLCache[string](1, time.Second))

	actualMetrics, err := scraper.scrape(context.Background())
	require.Error(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(pmetric.NewMetrics(), actualMetrics))
}

func TestScraper(t *testing.T) {
	factory := new(mockClientFactory)
	factory.initMocks([]string{"otel"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, separateSchemaAttrGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)
		cfg.Databases = []string{"otel"}
		cfg.Metrics.PostgresqlWalDelay.Enabled = true
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		cfg.Metrics.PostgresqlTupUpdated.Enabled = true
		cfg.Metrics.PostgresqlTupReturned.Enabled = true
		cfg.Metrics.PostgresqlTupFetched.Enabled = true
		cfg.Metrics.PostgresqlTupInserted.Enabled = true
		cfg.Metrics.PostgresqlTupDeleted.Enabled = true
		cfg.Metrics.PostgresqlBlksHit.Enabled = true
		cfg.Metrics.PostgresqlBlksRead.Enabled = true
		cfg.Metrics.PostgresqlSequentialScans.Enabled = true
		cfg.Metrics.PostgresqlDatabaseLocks.Enabled = true

		scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory, newCache(1), newTTLCache[string](1, time.Second))

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "otel", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_schemaattr.yaml")
	runTest(false, "expected.yaml")
}

func TestScraperNoDatabaseSingle(t *testing.T) {
	factory := new(mockClientFactory)
	factory.initMocks([]string{"otel"})

	runTest := func(separateSchemaAttr bool, file string, fileDefault string) {
		defer testutil.SetFeatureGateForTest(t, separateSchemaAttrGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)

		// Validate expected default config values and then enable all metrics
		require.False(t, cfg.Metrics.PostgresqlWalDelay.Enabled)
		cfg.Metrics.PostgresqlWalDelay.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDeadlocks.Enabled)
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempFiles.Enabled)
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupUpdated.Enabled)
		cfg.Metrics.PostgresqlTupUpdated.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupReturned.Enabled)
		cfg.Metrics.PostgresqlTupReturned.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupFetched.Enabled)
		cfg.Metrics.PostgresqlTupFetched.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupInserted.Enabled)
		cfg.Metrics.PostgresqlTupInserted.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupDeleted.Enabled)
		cfg.Metrics.PostgresqlTupDeleted.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlBlksHit.Enabled)
		cfg.Metrics.PostgresqlBlksHit.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlBlksRead.Enabled)
		cfg.Metrics.PostgresqlBlksRead.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlSequentialScans.Enabled)
		cfg.Metrics.PostgresqlSequentialScans.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDatabaseLocks.Enabled)
		cfg.Metrics.PostgresqlDatabaseLocks.Enabled = true

		scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "otel", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))

		cfg.Metrics.PostgresqlWalDelay.Enabled = false
		cfg.Metrics.PostgresqlDeadlocks.Enabled = false
		cfg.Metrics.PostgresqlTempFiles.Enabled = false
		cfg.Metrics.PostgresqlTupUpdated.Enabled = false
		cfg.Metrics.PostgresqlTupReturned.Enabled = false
		cfg.Metrics.PostgresqlTupFetched.Enabled = false
		cfg.Metrics.PostgresqlTupInserted.Enabled = false
		cfg.Metrics.PostgresqlTupDeleted.Enabled = false
		cfg.Metrics.PostgresqlBlksHit.Enabled = false
		cfg.Metrics.PostgresqlBlksRead.Enabled = false
		cfg.Metrics.PostgresqlSequentialScans.Enabled = false
		cfg.Metrics.PostgresqlDatabaseLocks.Enabled = false

		scraper = newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
		actualMetrics, err = scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile = filepath.Join("testdata", "scraper", "otel", fileDefault)
		expectedMetrics, err = golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_schemaattr.yaml", "expected_default_metrics_schemaattr.yaml")
	runTest(false, "expected.yaml", "expected_default_metrics.yaml")
}

func TestScraperNoDatabaseMultipleWithoutPreciseLag(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel", "open", "telemetry"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, separateSchemaAttrGate, separateSchemaAttr)()
		defer testutil.SetFeatureGateForTest(t, preciseLagMetricsFg, false)()

		cfg := createDefaultConfig().(*Config)

		// Validate expected default config values and then enable all metrics except wal delay
		require.False(t, cfg.Metrics.PostgresqlWalDelay.Enabled)
		require.False(t, cfg.Metrics.PostgresqlDeadlocks.Enabled)
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempFiles.Enabled)
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupUpdated.Enabled)
		cfg.Metrics.PostgresqlTupUpdated.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupReturned.Enabled)
		cfg.Metrics.PostgresqlTupReturned.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupFetched.Enabled)
		cfg.Metrics.PostgresqlTupFetched.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupInserted.Enabled)
		cfg.Metrics.PostgresqlTupInserted.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupDeleted.Enabled)
		cfg.Metrics.PostgresqlTupDeleted.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlBlksHit.Enabled)
		cfg.Metrics.PostgresqlBlksHit.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlBlksRead.Enabled)
		cfg.Metrics.PostgresqlBlksRead.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlSequentialScans.Enabled)
		cfg.Metrics.PostgresqlSequentialScans.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDatabaseLocks.Enabled)
		cfg.Metrics.PostgresqlDatabaseLocks.Enabled = true
		scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "multiple", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_imprecise_lag_schemaattr.yaml")
	runTest(false, "expected_imprecise_lag.yaml")
}

func TestScraperNoDatabaseMultiple(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel", "open", "telemetry"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, separateSchemaAttrGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)

		// Validate expected default config values and then enable all metrics
		require.False(t, cfg.Metrics.PostgresqlWalDelay.Enabled)
		cfg.Metrics.PostgresqlWalDelay.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDeadlocks.Enabled)
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempFiles.Enabled)
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupUpdated.Enabled)
		cfg.Metrics.PostgresqlTupUpdated.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupReturned.Enabled)
		cfg.Metrics.PostgresqlTupReturned.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupFetched.Enabled)
		cfg.Metrics.PostgresqlTupFetched.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupInserted.Enabled)
		cfg.Metrics.PostgresqlTupInserted.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupDeleted.Enabled)
		cfg.Metrics.PostgresqlTupDeleted.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlBlksHit.Enabled)
		cfg.Metrics.PostgresqlBlksHit.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlBlksRead.Enabled)
		cfg.Metrics.PostgresqlBlksRead.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlSequentialScans.Enabled)
		cfg.Metrics.PostgresqlSequentialScans.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDatabaseLocks.Enabled)
		cfg.Metrics.PostgresqlDatabaseLocks.Enabled = true
		scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "multiple", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)
		fmt.Println(actualMetrics.ResourceMetrics())
		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_schemaattr.yaml")
	runTest(false, "expected.yaml")
}

func TestScraperWithResourceAttributeFeatureGate(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel", "open", "telemetry"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, separateSchemaAttrGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)

		// Validate expected default config values and then enable all metrics
		require.False(t, cfg.Metrics.PostgresqlWalDelay.Enabled)
		cfg.Metrics.PostgresqlWalDelay.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDeadlocks.Enabled)
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempFiles.Enabled)
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupUpdated.Enabled)
		cfg.Metrics.PostgresqlTupUpdated.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupReturned.Enabled)
		cfg.Metrics.PostgresqlTupReturned.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupFetched.Enabled)
		cfg.Metrics.PostgresqlTupFetched.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupInserted.Enabled)
		cfg.Metrics.PostgresqlTupInserted.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupDeleted.Enabled)
		cfg.Metrics.PostgresqlTupDeleted.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlBlksHit.Enabled)
		cfg.Metrics.PostgresqlBlksHit.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlBlksRead.Enabled)
		cfg.Metrics.PostgresqlBlksRead.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlSequentialScans.Enabled)
		cfg.Metrics.PostgresqlSequentialScans.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDatabaseLocks.Enabled)
		cfg.Metrics.PostgresqlDatabaseLocks.Enabled = true

		scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "multiple", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_schemaattr.yaml")
	runTest(false, "expected.yaml")
}

func TestScraperWithResourceAttributeFeatureGateSingle(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, separateSchemaAttrGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)

		// Validate expected default config values and then enable all metrics
		require.False(t, cfg.Metrics.PostgresqlWalDelay.Enabled)
		cfg.Metrics.PostgresqlWalDelay.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDeadlocks.Enabled)
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempFiles.Enabled)
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupUpdated.Enabled)
		cfg.Metrics.PostgresqlTupUpdated.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupReturned.Enabled)
		cfg.Metrics.PostgresqlTupReturned.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupFetched.Enabled)
		cfg.Metrics.PostgresqlTupFetched.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupInserted.Enabled)
		cfg.Metrics.PostgresqlTupInserted.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTupDeleted.Enabled)
		cfg.Metrics.PostgresqlTupDeleted.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlBlksHit.Enabled)
		cfg.Metrics.PostgresqlBlksHit.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlBlksRead.Enabled)
		cfg.Metrics.PostgresqlBlksRead.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlSequentialScans.Enabled)
		cfg.Metrics.PostgresqlSequentialScans.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDatabaseLocks.Enabled)
		cfg.Metrics.PostgresqlDatabaseLocks.Enabled = true
		scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "otel", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_schemaattr.yaml")
	runTest(false, "expected.yaml")
}

func TestScraperExcludeDatabase(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel", "telemetry"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, separateSchemaAttrGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)
		cfg.ExcludeDatabases = []string{"open"}

		scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "multiple", file)

		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "exclude_schemaattr.yaml")
	runTest(false, "exclude.yaml")
}

//go:embed testdata/scraper/query-sample/expectedSql.sql
var expectedScrapeSampleQuery string

func TestScrapeQuerySample(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{}
	cfg.QuerySampleCollection.Enabled = true
	cfg.Events.DbServerQuerySample.Enabled = true
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)

	defer db.Close()

	factory := mockSimpleClientFactory{
		db: db,
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	logger, err := zap.NewProduction()
	assert.NoError(t, err)
	settings.TelemetrySettings = component.TelemetrySettings{
		Logger: logger,
	}
	scraper := newPostgreSQLScraper(settings, cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	mock.ExpectQuery(expectedScrapeSampleQuery).WillReturnRows(sqlmock.NewRows(
		[]string{"datname", "usename", "client_addrs", "client_hostname", "client_port", "query_start", "wait_event_type", "wait_event", "query_id", "pid", "application_name", "state", "query"},
	).FromCSVString("postgres,otelu,11.4.5.14,otel,114514,2025-02-12T16:37:54.843+08:00,,,123131231231,1450,receiver,idle,select * from pg_stat_activity where id = 32"))
	actualLogs, err := scraper.scrapeQuerySamples(context.Background(), 30)
	assert.NoError(t, err)
	expectedFile := filepath.Join("testdata", "scraper", "query-sample", "expected.yaml")
	expectedLogs, err := golden.ReadLogs(expectedFile)
	require.NoError(t, err)
	errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreTimestamp())
	assert.NoError(t, errs)
}

//go:embed testdata/scraper/top-query/expectedSql.sql
var expectedScrapeTopQuery string

//go:embed testdata/scraper/top-query/expectedExplain.sql
var expectedExplain string

func TestScrapeTopQueries(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{}
	cfg.TopQueryCollection.Enabled = true
	cfg.Events.DbServerTopQuery.Enabled = true
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)

	defer db.Close()

	factory := mockSimpleClientFactory{
		db: db,
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	logger, err := zap.NewProduction()
	assert.NoError(t, err)
	settings.TelemetrySettings = component.TelemetrySettings{
		Logger: logger,
	}

	queryid := "114514"
	expectedReturnedValue := map[string]string{
		"calls":               "123",
		"datname":             "postgres",
		"shared_blks_dirtied": "1111",
		"shared_blks_hit":     "1112",
		"shared_blks_read":    "1113",
		"shared_blks_written": "1114",
		"temp_blks_read":      "1115",
		"temp_blks_written":   "1116",
		"query":               "select * from pg_stat_activity where id = 32",
		"queryid":             queryid,
		"rolname":             "master",
		"rows":                "30",
		"total_exec_time":     "11000",
		"total_plan_time":     "12000",
	}

	expectedRows := make([]string, 0, len(expectedReturnedValue))
	expectedValues := ""
	for k, v := range expectedReturnedValue {
		expectedRows = append(expectedRows, k)
		expectedValues += fmt.Sprintf("%s,", v)
	}

	scraper := newPostgreSQLScraper(settings, cfg, factory, newCache(30), newTTLCache[string](1, time.Second))
	scraper.cache.Add(queryid+totalExecTimeColumnName, 10)
	scraper.cache.Add(queryid+totalPlanTimeColumnName, 11)
	scraper.cache.Add(queryid+callsColumnName, 120)
	scraper.cache.Add(queryid+rowsColumnName, 20)

	scraper.cache.Add(queryid+sharedBlksDirtiedColumnName, 1110)
	scraper.cache.Add(queryid+sharedBlksHitColumnName, 1110)
	scraper.cache.Add(queryid+sharedBlksReadColumnName, 1110)
	scraper.cache.Add(queryid+sharedBlksWrittenColumnName, 1110)
	scraper.cache.Add(queryid+tempBlksReadColumnName, 1110)
	scraper.cache.Add(queryid+tempBlksWrittenColumnName, 1110)

	mock.ExpectQuery(expectedScrapeTopQuery).WillReturnRows(sqlmock.NewRows(expectedRows).FromCSVString(expectedValues[:len(expectedValues)-1]))
	mock.ExpectQuery(expectedExplain).WillReturnRows(sqlmock.NewRows([]string{"QUERY PLAN"}).AddRow("[{\"Plan\":{\"Node Type\":\"Merge Join\",\"Parallel Aware\":false,\"Async Capable\":false,\"Join Type\":\"Inner\",\"Startup Cost\":0.43,\"Total Cost\":55.27,\"Plan Rows\":290,\"Plan Width\":1675,\"Inner Unique\":\"?\",\"Merge Cond\":\"( e.businessentityid = p.businessentityid )\",\"Plans\":[{\"Node Type\":\"Index Scan\",\"Parent Relationship\":\"Outer\",\"Parallel Aware\":false,\"Async Capable\":false,\"Scan Direction\":\"Forward\",\"Index Name\":\"PK_Employee_BusinessEntityID\",\"Relation Name\":\"employee\",\"Alias\":\"e\",\"Startup Cost\":0.15,\"Total Cost\":21.5,\"Plan Rows\":290,\"Plan Width\":112},{\"Node Type\":\"Index Scan\",\"Parent Relationship\":\"Inner\",\"Parallel Aware\":false,\"Async Capable\":false,\"Scan Direction\":\"Forward\",\"Index Name\":\"PK_Person_BusinessEntityID\",\"Relation Name\":\"person\",\"Alias\":\"p\",\"Startup Cost\":0.29,\"Total Cost\":2261.87,\"Plan Rows\":19972,\"Plan Width\":1563}]}}]"))
	actualLogs, err := scraper.scrapeTopQuery(context.Background(), 31, 32, 33)
	assert.NoError(t, err)
	expectedFile := filepath.Join("testdata", "scraper", "top-query", "expected.yaml")
	expectedLogs, err := golden.ReadLogs(expectedFile)
	require.NoError(t, err)
	// golden.WriteLogs(t, expectedFile, actualLogs)
	errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreTimestamp())
	assert.NoError(t, errs)
}

type (
	mockClientFactory       struct{ mock.Mock }
	mockClient              struct{ mock.Mock }
	mockSimpleClientFactory struct {
		db *sql.DB
	}
)

// explainQuery implements client.
func (m *mockClient) explainQuery(_ string, _ string, _ *zap.Logger) (string, error) {
	panic("unimplemented")
}

// getTopQuery implements client.
func (m *mockClient) getTopQuery(_ context.Context, _ int64, _ *zap.Logger) ([]map[string]any, error) {
	panic("unimplemented")
}

// close implements postgreSQLClientFactory.
func (m mockSimpleClientFactory) close() error {
	return nil
}

// getClient implements postgreSQLClientFactory.
func (m mockSimpleClientFactory) getClient(_ string) (client, error) {
	return &postgreSQLClient{
		client:  m.db,
		closeFn: m.close,
	}, nil
}

// getQuerySamples implements client.
func (m *mockClient) getQuerySamples(_ context.Context, _ int64, _ *zap.Logger) ([]map[string]any, error) {
	panic("this should not be invoked")
}

var _ client = &mockClient{}

func (m *mockClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockClient) getDatabaseStats(_ context.Context, databases []string) (map[databaseName]databaseStats, error) {
	args := m.Called(databases)
	return args.Get(0).(map[databaseName]databaseStats), args.Error(1)
}

func (m *mockClient) getDatabaseLocks(ctx context.Context) ([]databaseLocks, error) {
	args := m.Called(ctx)
	return args.Get(0).([]databaseLocks), args.Error(1)
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

func (m *mockClient) getVersion(_ context.Context) (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *mockClientFactory) getClient(database string) (client, error) {
	args := m.Called(database)
	return args.Get(0).(client), args.Error(1)
}

func (m *mockClientFactory) close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockClientFactory) initMocks(databases []string) {
	listClient := new(mockClient)
	listClient.initMocks(defaultPostgreSQLDatabase, "public", databases, 0)
	m.On("getClient", defaultPostgreSQLDatabase).Return(listClient, nil)

	for index, db := range databases {
		client := new(mockClient)
		client.initMocks(db, "public", databases, index)
		m.On("getClient", db).Return(client, nil)
	}
}

func (m *mockClient) initMocks(database string, schema string, databases []string, index int) {
	m.On("Close").Return(nil)

	if database == defaultPostgreSQLDatabase {
		m.On("listDatabases").Return(databases, nil)

		dbStats := map[databaseName]databaseStats{}
		dbSize := map[databaseName]int64{}
		backends := map[databaseName]int64{}

		for idx, db := range databases {
			dbStats[databaseName(db)] = databaseStats{
				transactionCommitted: int64(idx + 1),
				transactionRollback:  int64(idx + 2),
				deadlocks:            int64(idx + 3),
				tempFiles:            int64(idx + 4),
				tupUpdated:           int64(idx + 5),
				tupReturned:          int64(idx + 6),
				tupFetched:           int64(idx + 7),
				tupInserted:          int64(idx + 8),
				tupDeleted:           int64(idx + 9),
				blksHit:              int64(idx + 10),
				blksRead:             int64(idx + 11),
			}
			dbSize[databaseName(db)] = int64(idx + 4)
			backends[databaseName(db)] = int64(idx + 3)
		}

		m.On("getDatabaseStats", databases).Return(dbStats, nil)
		m.On("getDatabaseSize", databases).Return(dbSize, nil)
		m.On("getBackends", databases).Return(backends, nil)
		m.On("getBGWriterStats", mock.Anything).Return(&bgStat{
			checkpointsReq:       1,
			checkpointsScheduled: 2,
			checkpointWriteTime:  3.12,
			checkpointSyncTime:   4.23,
			bgWrites:             5,
			bufferBackendWrites:  7,
			bufferFsyncWrites:    8,
			bufferCheckpoints:    9,
			buffersAllocated:     10,
			maxWritten:           11,
		}, nil)
		m.On("getMaxConnections", mock.Anything).Return(int64(100), nil)
		m.On("getLatestWalAgeSeconds", mock.Anything).Return(int64(3600), nil)
		m.On("getDatabaseLocks", mock.Anything).Return([]databaseLocks{
			{
				relation: "pg_locks",
				mode:     "AccessShareLock",
				lockType: "relation",
				locks:    3600,
			},
			{
				relation: "pg_class",
				mode:     "AccessShareLock",
				lockType: "relation",
				locks:    5600,
			},
		}, nil)
		m.On("getDatabaseLocks", mock.Anything).Return([]databaseLocks{
			{
				relation: "abd_table",
				mode:     "ExplicitLock",
				lockType: "relation",
				locks:    1600,
			},
			{
				relation: "pg_class",
				mode:     "AccessShareLock",
				lockType: "relation",
				locks:    5600,
			},
		}, errors.New("some error"))
		m.On("getReplicationStats", mock.Anything).Return([]replicationStats{
			{
				clientAddr:   "unix",
				pendingBytes: 1024,
				flushLagInt:  600,
				replayLagInt: 700,
				writeLagInt:  800,
				flushLag:     600.400,
				replayLag:    700.550,
				writeLag:     800.660,
			},
			{
				clientAddr:   "nulls",
				pendingBytes: -1,
				flushLagInt:  -1,
				replayLagInt: -1,
				writeLagInt:  -1,
				flushLag:     -1,
				replayLag:    -1,
				writeLag:     -1,
			},
		}, nil)
	} else {
		table1 := "table1"
		table2 := "table2"
		tableMetrics := map[tableIdentifier]tableStats{
			tableKey(database, schema, table1): {
				database:    database,
				schema:      schema,
				table:       table1,
				live:        int64(index + 7),
				dead:        int64(index + 8),
				inserts:     int64(index + 39),
				upd:         int64(index + 40),
				del:         int64(index + 41),
				hotUpd:      int64(index + 42),
				size:        int64(index + 43),
				vacuumCount: int64(index + 44),
				seqScans:    int64(index + 45),
			},
			tableKey(database, schema, table2): {
				database:    database,
				schema:      schema,
				table:       table2,
				live:        int64(index + 9),
				dead:        int64(index + 10),
				inserts:     int64(index + 43),
				upd:         int64(index + 44),
				del:         int64(index + 45),
				hotUpd:      int64(index + 46),
				size:        int64(index + 47),
				vacuumCount: int64(index + 48),
				seqScans:    int64(index + 49),
			},
		}

		blocksMetrics := map[tableIdentifier]tableIOStats{
			tableKey(database, schema, table1): {
				database:  database,
				schema:    schema,
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
			tableKey(database, schema, table2): {
				database:  database,
				schema:    schema,
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

		index1 := database + "_test1_pkey"
		index2 := database + "_test2_pkey"
		indexStats := map[indexIdentifer]indexStat{
			indexKey(database, schema, table1, index1): {
				database: database,
				schema:   schema,
				table:    table1,
				index:    index1,
				scans:    int64(index + 35),
				size:     int64(index + 36),
			},
			indexKey(index2, schema, table2, index2): {
				database: database,
				schema:   schema,
				table:    table2,
				index:    index2,
				scans:    int64(index + 37),
				size:     int64(index + 38),
			},
		}
		m.On("getIndexStats", mock.Anything, database).Return(indexStats, nil)
	}
}
