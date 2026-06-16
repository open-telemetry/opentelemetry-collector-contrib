// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"text/template"
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

	actualMetrics, err := scraper.scrape(t.Context())
	require.Error(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(pmetric.NewMetrics(), actualMetrics))
}

func TestScraper(t *testing.T) {
	factory := new(mockClientFactory)
	factory.initMocks([]string{"otel"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)
		cfg.Databases = []string{"otel"}
		cfg.Metrics.PostgresqlWalDelay.Enabled = true
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		cfg.Metrics.PostgresqlTempIo.Enabled = true
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

		actualMetrics, err := scraper.scrape(t.Context())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "otel", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceAttributeValue("service.instance.id"), pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_schemaattr.yaml")
	runTest(false, "expected.yaml")
}

func TestScraperNoDatabaseSingle(t *testing.T) {
	factory := new(mockClientFactory)
	factory.initMocks([]string{"otel"})

	runTest := func(separateSchemaAttr bool, file, fileDefault string) {
		defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)

		// Validate expected default config values and then enable all metrics
		require.False(t, cfg.Metrics.PostgresqlWalDelay.Enabled)
		cfg.Metrics.PostgresqlWalDelay.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDeadlocks.Enabled)
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempFiles.Enabled)
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempIo.Enabled)
		cfg.Metrics.PostgresqlTempIo.Enabled = true
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
		actualMetrics, err := scraper.scrape(t.Context())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "otel", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceAttributeValue("service.instance.id"), pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))

		cfg.Metrics.PostgresqlWalDelay.Enabled = false
		cfg.Metrics.PostgresqlDeadlocks.Enabled = false
		cfg.Metrics.PostgresqlTempFiles.Enabled = false
		cfg.Metrics.PostgresqlTempIo.Enabled = false
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
		actualMetrics, err = scraper.scrape(t.Context())
		require.NoError(t, err)

		expectedFile = filepath.Join("testdata", "scraper", "otel", fileDefault)
		expectedMetrics, err = golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceAttributeValue("service.instance.id"), pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_schemaattr.yaml", "expected_default_metrics_schemaattr.yaml")
	runTest(false, "expected.yaml", "expected_default_metrics.yaml")
}

func TestScraperNoDatabaseMultipleWithoutPreciseLag(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel", "open", "telemetry"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, separateSchemaAttr)()
		defer testutil.SetFeatureGateForTest(t, metadata.PostgresqlreceiverPreciselagmetricsFeatureGate, false)()

		cfg := createDefaultConfig().(*Config)

		// Validate expected default config values and then enable all metrics except wal delay
		require.False(t, cfg.Metrics.PostgresqlWalDelay.Enabled)
		require.False(t, cfg.Metrics.PostgresqlDeadlocks.Enabled)
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempFiles.Enabled)
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempIo.Enabled)
		cfg.Metrics.PostgresqlTempIo.Enabled = true
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

		actualMetrics, err := scraper.scrape(t.Context())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "multiple", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceAttributeValue("service.instance.id"), pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_imprecise_lag_schemaattr.yaml")
	runTest(false, "expected_imprecise_lag.yaml")
}

func TestScraperNoDatabaseMultiple(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel", "open", "telemetry"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)

		// Validate expected default config values and then enable all metrics
		require.False(t, cfg.Metrics.PostgresqlWalDelay.Enabled)
		cfg.Metrics.PostgresqlWalDelay.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDeadlocks.Enabled)
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempFiles.Enabled)
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempIo.Enabled)
		cfg.Metrics.PostgresqlTempIo.Enabled = true
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

		actualMetrics, err := scraper.scrape(t.Context())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "multiple", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)
		fmt.Println(actualMetrics.ResourceMetrics())
		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceAttributeValue("service.instance.id"), pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_schemaattr.yaml")
	runTest(false, "expected.yaml")
}

func TestScraperWithResourceAttributeFeatureGate(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel", "open", "telemetry"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)

		// Validate expected default config values and then enable all metrics
		require.False(t, cfg.Metrics.PostgresqlWalDelay.Enabled)
		cfg.Metrics.PostgresqlWalDelay.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDeadlocks.Enabled)
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempFiles.Enabled)
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempIo.Enabled)
		cfg.Metrics.PostgresqlTempIo.Enabled = true
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

		actualMetrics, err := scraper.scrape(t.Context())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "multiple", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceAttributeValue("service.instance.id"), pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_schemaattr.yaml")
	runTest(false, "expected.yaml")
}

func TestScraperWithResourceAttributeFeatureGateSingle(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)

		// Validate expected default config values and then enable all metrics
		require.False(t, cfg.Metrics.PostgresqlWalDelay.Enabled)
		cfg.Metrics.PostgresqlWalDelay.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlDeadlocks.Enabled)
		cfg.Metrics.PostgresqlDeadlocks.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempFiles.Enabled)
		cfg.Metrics.PostgresqlTempFiles.Enabled = true
		require.False(t, cfg.Metrics.PostgresqlTempIo.Enabled)
		cfg.Metrics.PostgresqlTempIo.Enabled = true
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

		actualMetrics, err := scraper.scrape(t.Context())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "otel", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceAttributeValue("service.instance.id"), pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "expected_schemaattr.yaml")
	runTest(false, "expected.yaml")
}

func TestScraperExcludeDatabase(t *testing.T) {
	factory := mockClientFactory{}
	factory.initMocks([]string{"otel", "telemetry"})

	runTest := func(separateSchemaAttr bool, file string) {
		defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, separateSchemaAttr)()

		cfg := createDefaultConfig().(*Config)
		cfg.ExcludeDatabases = []string{"open"}

		scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))

		actualMetrics, err := scraper.scrape(t.Context())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "multiple", file)

		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreResourceAttributeValue("service.instance.id"), pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	}

	runTest(true, "exclude_schemaattr.yaml")
	runTest(false, "exclude.yaml")
}

// disableAllMetrics clears the Enabled flag on every metric in the config. It uses
// reflection so the test stays correct as metrics are added or removed.
func disableAllMetrics(mc *metadata.MetricsConfig) {
	v := reflect.ValueOf(mc).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() != reflect.Struct {
			continue
		}
		enabledField := field.FieldByName("Enabled")
		if enabledField.IsValid() && enabledField.CanSet() {
			enabledField.SetBool(false)
		}
	}
}

// TestScraperSkipsQueriesForDisabledMetrics verifies that when every metric is
// disabled, none of the metric-fetching queries are executed. The mock client
// panics on any unexpected call, so a query that should have been skipped will
// fail the test loudly; AssertNotCalled documents the intent explicitly.
func TestScraperSkipsQueriesForDisabledMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{"otel"} // set explicitly so listDatabases is not called
	disableAllMetrics(&cfg.Metrics)

	dbClient := new(mockClient)
	dbClient.On("Close").Return(nil)

	factory := new(mockClientFactory)
	factory.On("getClient", defaultPostgreSQLDatabase).Return(dbClient, nil)
	factory.On("getClient", "otel").Return(dbClient, nil)

	scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory, newCache(1), newTTLCache[string](1, time.Second))

	_, err := scraper.scrape(t.Context())
	require.NoError(t, err)

	dbClient.AssertNotCalled(t, "getBackends", mock.Anything)
	dbClient.AssertNotCalled(t, "getDatabaseSize", mock.Anything)
	dbClient.AssertNotCalled(t, "getDatabaseStats", mock.Anything)
	dbClient.AssertNotCalled(t, "getDatabaseTableMetrics", mock.Anything, mock.Anything)
	dbClient.AssertNotCalled(t, "getTableCount", mock.Anything)
	dbClient.AssertNotCalled(t, "getBlocksReadByTable", mock.Anything, mock.Anything)
	dbClient.AssertNotCalled(t, "getIndexStats", mock.Anything, mock.Anything)
	dbClient.AssertNotCalled(t, "getFunctionStats", mock.Anything, mock.Anything)
	dbClient.AssertNotCalled(t, "getBGWriterStats", mock.Anything)
	dbClient.AssertNotCalled(t, "getMaxConnections", mock.Anything)
	dbClient.AssertNotCalled(t, "getLatestWalAgeSeconds", mock.Anything)
	dbClient.AssertNotCalled(t, "getReplicationStats", mock.Anything)
	dbClient.AssertNotCalled(t, "getDatabaseLocks", mock.Anything)
}

// TestScraperTableCountUsesCheapQuery verifies that when only
// postgresql.table.count is enabled, the count is satisfied by the cheap
// getTableCount query rather than the expensive per-table getDatabaseTableMetrics
// query (and that the separate blocks-read query is not run either).
func TestScraperTableCountUsesCheapQuery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{"otel"}
	disableAllMetrics(&cfg.Metrics)
	cfg.Metrics.PostgresqlTableCount.Enabled = true

	dbClient := new(mockClient)
	dbClient.On("Close").Return(nil)
	dbClient.On("getTableCount", mock.Anything).Return(int64(7), nil)

	factory := new(mockClientFactory)
	factory.On("getClient", defaultPostgreSQLDatabase).Return(dbClient, nil)
	factory.On("getClient", "otel").Return(dbClient, nil)

	scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory, newCache(1), newTTLCache[string](1, time.Second))

	_, err := scraper.scrape(t.Context())
	require.NoError(t, err)

	dbClient.AssertCalled(t, "getTableCount", mock.Anything)
	dbClient.AssertNotCalled(t, "getDatabaseTableMetrics", mock.Anything, mock.Anything)
	dbClient.AssertNotCalled(t, "getBlocksReadByTable", mock.Anything, mock.Anything)
}

// TestScraperTableCountWithPerTableStats verifies that when a per-table stat is
// enabled alongside the count, the full per-table query is used (and the cheap
// count query is not), since the count can be derived from its results for free.
func TestScraperTableCountWithPerTableStats(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{"otel"}
	disableAllMetrics(&cfg.Metrics)
	cfg.Metrics.PostgresqlTableCount.Enabled = true
	cfg.Metrics.PostgresqlRows.Enabled = true

	dbClient := new(mockClient)
	dbClient.On("Close").Return(nil)
	dbClient.On("getDatabaseTableMetrics", mock.Anything, mock.Anything).
		Return(map[tableIdentifier]tableStats{}, nil)

	factory := new(mockClientFactory)
	factory.On("getClient", defaultPostgreSQLDatabase).Return(dbClient, nil)
	factory.On("getClient", "otel").Return(dbClient, nil)

	scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory, newCache(1), newTTLCache[string](1, time.Second))

	_, err := scraper.scrape(t.Context())
	require.NoError(t, err)

	dbClient.AssertCalled(t, "getDatabaseTableMetrics", mock.Anything, mock.Anything)
	dbClient.AssertNotCalled(t, "getTableCount", mock.Anything)
}

// enableMetricByName enables the single metric whose mapstructure tag matches name.
func enableMetricByName(t *testing.T, mc *metadata.MetricsConfig, name string) {
	t.Helper()
	v := reflect.ValueOf(mc).Elem()
	tp := v.Type()
	for i := 0; i < tp.NumField(); i++ {
		if tp.Field(i).Tag.Get("mapstructure") == name {
			v.Field(i).FieldByName("Enabled").SetBool(true)
			return
		}
	}
	t.Fatalf("no metric field with mapstructure tag %q", name)
}

// emittedMetricNames returns the set of metric names present in md.
func emittedMetricNames(md pmetric.Metrics) map[string]struct{} {
	names := map[string]struct{}{}
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				names[ms.At(k).Name()] = struct{}{}
			}
		}
	}
	return names
}

// TestEnabledMetricIsEmitted is a safety net for the hand-maintained mapping
// between queries and the metrics they feed (the enablement guards in the
// collect*/retrieve* functions). For every metric, it enables only that metric
// and asserts the scrape still emits it. Because the metric list is discovered
// by reflection over MetricsConfig, a newly added metric is covered automatically:
// if it is wired into a query but not into that query's enablement guard, enabling
// only the new metric skips the query, emits nothing, and fails this test.
func TestEnabledMetricIsEmitted(t *testing.T) {
	mt := reflect.TypeFor[metadata.MetricsConfig]()
	for i := 0; i < mt.NumField(); i++ {
		name := mt.Field(i).Tag.Get("mapstructure")
		t.Run(name, func(t *testing.T) {
			if name == "postgresql.wal.lag" {
				// wal.lag is the legacy counterpart of wal.delay and only emits
				// when the (beta, default-on) precise-lag feature gate is disabled.
				defer testutil.SetFeatureGateForTest(t, metadata.PostgresqlreceiverPreciselagmetricsFeatureGate, false)()
			}

			factory := mockClientFactory{}
			factory.initMocks([]string{"otel"})

			cfg := createDefaultConfig().(*Config)
			disableAllMetrics(&cfg.Metrics)
			enableMetricByName(t, &cfg.Metrics, name)

			scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))

			md, err := scraper.scrape(t.Context())
			require.NoError(t, err)

			_, ok := emittedMetricNames(md)[name]
			assert.True(t, ok, "metric %q is enabled but was not emitted; if it is recorded by a query, "+
				"make sure that query's enablement guard in scraper.go includes it", name)
		})
	}
}

// errQueryProbe is returned by the targeted query in newQueryMockClient to
// simulate a single query failing while the rest succeed.
var errQueryProbe = errors.New("query probe failure")

// infraMethods are client methods that are not metric-fetching queries; they are
// ignored when introspecting which queries a scrape ran.
var infraMethods = map[string]bool{
	"Close": true, "listDatabases": true, "getVersion": true,
	"getQuerySamples": true, "getTopQuery": true, "explainQuery": true,
}

// newQueryMockClient returns a mock client that serves valid data for every query
// so that any enabled metric is emitted. If errMethod is non-empty, that one query
// instead returns empty data and an error — mirroring how the real client returns
// nil on failure — which lets a test probe whether a query is actually required to
// produce a given metric.
func newQueryMockClient(errMethod string) *mockClient {
	// probe returns (val, nil) normally, or the zero value and an error when method
	// is the one being failed, matching the real client's nil-on-error behavior.
	probe := func(method string, val any) (any, error) {
		if method == errMethod {
			switch val.(type) {
			case map[databaseName]int64:
				return map[databaseName]int64(nil), errQueryProbe
			case map[databaseName]databaseStats:
				return map[databaseName]databaseStats(nil), errQueryProbe
			case *bgStat:
				return (*bgStat)(nil), errQueryProbe
			case int64:
				return int64(0), errQueryProbe
			case []replicationStats:
				return []replicationStats(nil), errQueryProbe
			case []databaseLocks:
				return []databaseLocks(nil), errQueryProbe
			case map[tableIdentifier]tableStats:
				return map[tableIdentifier]tableStats(nil), errQueryProbe
			case map[tableIdentifier]tableIOStats:
				return map[tableIdentifier]tableIOStats(nil), errQueryProbe
			case map[indexIdentifer]indexStat:
				return map[indexIdentifer]indexStat(nil), errQueryProbe
			case map[functionIdentifer]functionStat:
				return map[functionIdentifer]functionStat(nil), errQueryProbe
			}
		}
		return val, nil
	}

	const db, schema = "otel", "public"
	c := new(mockClient)
	c.On("Close").Return(nil)
	c.On("listDatabases").Return([]string{db}, nil)
	c.On("getVersion").Return("16.0", nil)
	c.On("getBackends", mock.Anything).Return(probe("getBackends", map[databaseName]int64{databaseName(db): 1}))
	c.On("getDatabaseSize", mock.Anything).Return(probe("getDatabaseSize", map[databaseName]int64{databaseName(db): 1}))
	c.On("getDatabaseStats", mock.Anything).Return(probe("getDatabaseStats", map[databaseName]databaseStats{databaseName(db): {
		transactionCommitted: 1, transactionRollback: 1, deadlocks: 1, tempFiles: 1, tempIo: 1,
		tupUpdated: 1, tupReturned: 1, tupFetched: 1, tupInserted: 1, tupDeleted: 1, blksHit: 1, blksRead: 1,
	}}))
	c.On("getBGWriterStats", mock.Anything).Return(probe("getBGWriterStats", &bgStat{
		checkpointsReq: 1, checkpointsScheduled: 1, checkpointWriteTime: 1, checkpointSyncTime: 1,
		bgWrites: 1, bufferBackendWrites: 1, bufferFsyncWrites: 1, bufferCheckpoints: 1, buffersAllocated: 1, maxWritten: 1,
	}))
	c.On("getMaxConnections", mock.Anything).Return(probe("getMaxConnections", int64(100)))
	c.On("getLatestWalAgeSeconds", mock.Anything).Return(probe("getLatestWalAgeSeconds", int64(3600)))
	c.On("getReplicationStats", mock.Anything).Return(probe("getReplicationStats", []replicationStats{{
		clientAddr: "addr", pendingBytes: 1,
		flushLagInt: 1, replayLagInt: 1, writeLagInt: 1, flushLag: 1, replayLag: 1, writeLag: 1,
	}}))
	c.On("getDatabaseLocks", mock.Anything).Return(probe("getDatabaseLocks", []databaseLocks{
		{relation: "pg_class", mode: "AccessShareLock", lockType: "relation", locks: 1},
	}))
	c.On("getDatabaseTableMetrics", mock.Anything, mock.Anything).Return(probe("getDatabaseTableMetrics", map[tableIdentifier]tableStats{
		tableKey(db, schema, "t1"): {database: db, schema: schema, table: "t1", live: 1, dead: 1, inserts: 1, upd: 1, del: 1, hotUpd: 1, size: 1, vacuumCount: 1, seqScans: 1},
	}))
	c.On("getTableCount", mock.Anything).Return(probe("getTableCount", int64(1)))
	c.On("getBlocksReadByTable", mock.Anything, mock.Anything).Return(probe("getBlocksReadByTable", map[tableIdentifier]tableIOStats{
		tableKey(db, schema, "t1"): {database: db, schema: schema, table: "t1", heapRead: 1, heapHit: 1, idxRead: 1, idxHit: 1, toastRead: 1, toastHit: 1, tidxRead: 1, tidxHit: 1},
	}))
	c.On("getIndexStats", mock.Anything, mock.Anything).Return(probe("getIndexStats", map[indexIdentifer]indexStat{
		indexKey(db, schema, "t1", "i1"): {database: db, schema: schema, table: "t1", index: "i1", scans: 1, size: 1},
	}))
	c.On("getFunctionStats", mock.Anything, mock.Anything).Return(probe("getFunctionStats", map[functionIdentifer]functionStat{
		functionKey(db, schema, "f1"): {database: db, schema: schema, function: "f1", calls: 1},
	}))
	return c
}

// scrapeProbe enables only the named metric, runs a scrape against a mock where
// errMethod (if set) fails, and reports which queries ran and whether the metric
// was emitted.
func scrapeProbe(t *testing.T, metric, errMethod string) (ranQueries map[string]bool, emitted bool) {
	t.Helper()
	if metric == "postgresql.wal.lag" {
		// wal.lag is the legacy counterpart of wal.delay and only emits when the
		// (beta, default-on) precise-lag feature gate is disabled.
		defer testutil.SetFeatureGateForTest(t, metadata.PostgresqlreceiverPreciselagmetricsFeatureGate, false)()
	}

	client := newQueryMockClient(errMethod)
	factory := new(mockClientFactory)
	factory.On("getClient", mock.Anything).Return(client, nil)

	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{"otel"} // set explicitly so listDatabases is not called
	disableAllMetrics(&cfg.Metrics)
	enableMetricByName(t, &cfg.Metrics, metric)

	scraper := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory, newCache(1), newTTLCache[string](1, time.Second))

	// A probe deliberately fails a query, so scrape may return a partial error; the
	// signal we care about is whether the metric was still emitted.
	md, _ := scraper.scrape(t.Context())

	ranQueries = map[string]bool{}
	for i := range client.Calls {
		method := client.Calls[i].Method
		if !infraMethods[method] {
			ranQueries[method] = true
		}
	}
	_, emitted = emittedMetricNames(md)[metric]
	return ranQueries, emitted
}

// TestOnlyNeededQueriesRun is the inverse of TestEnabledMetricIsEmitted: for each
// metric, it asserts that every query the scrape runs is actually required to
// produce that metric, so no unnecessary queries are issued.
//
// It needs no hand-maintained metric-to-query mapping. Instead it discovers, by
// reflection over MetricsConfig, every metric; runs a scrape with only that metric
// enabled to observe which queries run; then forces each observed query to fail in
// turn. A query that is genuinely needed makes the metric disappear when it fails;
// a query that still leaves the metric emitted was run needlessly (its enablement
// guard is too broad).
func TestOnlyNeededQueriesRun(t *testing.T) {
	mt := reflect.TypeFor[metadata.MetricsConfig]()
	for i := 0; i < mt.NumField(); i++ {
		name := mt.Field(i).Tag.Get("mapstructure")
		t.Run(name, func(t *testing.T) {
			ranQueries, emitted := scrapeProbe(t, name, "")
			require.Truef(t, emitted, "metric %q was enabled but not emitted", name)

			if name == "postgresql.table.count" {
				// table.count is recorded unconditionally from the table count (it
				// emits 0 even if its query fails), so the failure probe below can't
				// prove query necessity. Its query gating is covered instead by
				// TestScraperTableCountUsesCheapQuery and ...WithPerTableStats.
				return
			}

			for q := range ranQueries {
				_, stillEmitted := scrapeProbe(t, name, q)
				assert.False(t, stillEmitted, fmt.Sprintf(
					"query %q ran for metric %q but is not required to produce it "+
						"(forcing it to fail still emitted the metric); its enablement guard is too broad", q, name))
			}
		})
	}
}

//go:embed testdata/scraper/query-sample/expectedSql.sql
var expectedScrapeSampleQuery string

var querySampleColumns = []string{
	querySampleColumnDatname,
	querySampleColumnUsename,
	querySampleColumnClientAddr,
	querySampleColumnClientHostname,
	querySampleColumnClientPort,
	querySampleColumnQueryStart,
	querySampleColumnWaitEventType,
	querySampleColumnWaitEvent,
	querySampleColumnQueryID,
	querySampleColumnPID,
	querySampleColumnApplicationName,
	querySampleColumnQueryStartTimestamp,
	querySampleColumnState,
	querySampleColumnQuery,
	querySampleColumnDurationMilliseconds,
}

func newQuerySampleRows(t *testing.T, values map[string]any) *sqlmock.Rows {
	t.Helper()

	rowValues := make([]driver.Value, len(querySampleColumns))
	for i, col := range querySampleColumns {
		if v, ok := values[col]; ok {
			rowValues[i] = v
			continue
		}
		rowValues[i] = ""
	}

	return sqlmock.NewRows(querySampleColumns).AddRow(rowValues...)
}

func TestScrapeQuerySample(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{}
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
	scraper.newestQueryTimestamp = 123440.111
	mock.ExpectQuery(expectedScrapeSampleQuery).WillReturnRows(newQuerySampleRows(t, map[string]any{
		querySampleColumnDatname:              "postgres",
		querySampleColumnUsename:              "otelu",
		querySampleColumnClientAddr:           "11.4.5.14",
		querySampleColumnClientHostname:       "otel",
		querySampleColumnClientPort:           "114514",
		querySampleColumnQueryStart:           "2025-02-12T16:37:54.843+08:00",
		querySampleColumnQueryID:              "123131231231",
		querySampleColumnPID:                  "1450",
		querySampleColumnApplicationName:      "receiver",
		querySampleColumnQueryStartTimestamp:  "123445.123",
		querySampleColumnState:                "idle",
		querySampleColumnQuery:                "select * from pg_stat_activity where id = 32",
		querySampleColumnDurationMilliseconds: "1.2",
	}))
	actualLogs, err := scraper.scrapeQuerySamples(t.Context(), 30)
	assert.NoError(t, err)
	expectedFile := filepath.Join("testdata", "scraper", "query-sample", "expected.yaml")
	// Uncomment line below to re-generate expected logs.
	// golden.WriteLogs(t, expectedFile, actualLogs)
	expectedLogs, err := golden.ReadLogs(expectedFile)
	require.NoError(t, err)
	errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreResourceAttributeValue("service.instance.id"), plogtest.IgnoreTimestamp())
	assert.NoError(t, errs)
}

func TestScrapeQuerySampleWithTraceparent(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{}
	cfg.Events.DbServerQuerySample.Enabled = true
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)

	defer db.Close()

	factory := mockSimpleClientFactory{
		db: db,
	}

	settings := receivertest.NewNopSettings(metadata.Type)
	logger, err := zap.NewProduction()
	require.NoError(t, err)
	settings.TelemetrySettings = component.TelemetrySettings{
		Logger: logger,
	}

	scraper := newPostgreSQLScraper(settings, cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	scraper.newestQueryTimestamp = 123440.111

	traceparent := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	mock.ExpectQuery(expectedScrapeSampleQuery).WillReturnRows(newQuerySampleRows(t, map[string]any{
		querySampleColumnDatname:              "postgres",
		querySampleColumnUsename:              "otelu",
		querySampleColumnClientAddr:           "11.4.5.14",
		querySampleColumnClientHostname:       "otel",
		querySampleColumnClientPort:           "114514",
		querySampleColumnQueryStart:           "2025-02-12T16:37:54.843+08:00",
		querySampleColumnQueryID:              "123131231231",
		querySampleColumnPID:                  "1450",
		querySampleColumnApplicationName:      traceparent,
		querySampleColumnQueryStartTimestamp:  "123445.123",
		querySampleColumnState:                "idle",
		querySampleColumnQuery:                "select * from pg_stat_activity where id = 32",
		querySampleColumnDurationMilliseconds: "1.2",
	}))
	actualLogs, err := scraper.scrapeQuerySamples(t.Context(), 30)
	require.NoError(t, err)

	require.Equal(t, 1, actualLogs.ResourceLogs().Len())
	rl := actualLogs.ResourceLogs().At(0)
	require.Equal(t, 1, rl.ScopeLogs().Len())
	sl := rl.ScopeLogs().At(0)
	require.Equal(t, 1, sl.LogRecords().Len())
	lr := sl.LogRecords().At(0)

	require.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", lr.TraceID().String())
	require.Equal(t, "00f067aa0ba902b7", lr.SpanID().String())

	applicationName, ok := lr.Attributes().Get("postgresql.application_name")
	require.True(t, ok)
	require.Equal(t, traceparent, applicationName.Str())
}

func TestQuerySampleTemplateRendering(t *testing.T) {
	tmpl := template.Must(template.New("querySample").Option("missingkey=error").Parse(querySampleTemplate))

	tests := []struct {
		name   string
		params map[string]any
	}{
		{
			name: "renders with standard parameters",
			params: map[string]any{
				"limit":                int64(50),
				"newestQueryTimestamp": 999999.555,
			},
		},
		{
			name: "renders with zero timestamp",
			params: map[string]any{
				"limit":                int64(10),
				"newestQueryTimestamp": float64(0),
			},
		},
	}

	requiredClauses := []string{
		"pid != pg_backend_pid()",
		"query_start IS NOT NULL",
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := bytes.Buffer{}
			err := tmpl.Execute(&buf, tc.params)
			require.NoError(t, err)

			rendered := buf.String()
			for _, clause := range requiredClauses {
				assert.Contains(t, rendered, clause, "rendered SQL should contain %q", clause)
			}

			assert.Contains(t, rendered, fmt.Sprintf("LIMIT %v;", tc.params["limit"]))
			assert.Contains(t, rendered, fmt.Sprintf("TO_TIMESTAMP(%v)", tc.params["newestQueryTimestamp"]))
		})
	}
}

func TestScrapeQuerySampleNoResults(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{}
	cfg.Events.DbServerQuerySample.Enabled = true
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)

	defer db.Close()

	factory := mockSimpleClientFactory{db: db}

	settings := receivertest.NewNopSettings(metadata.Type)
	logger, err := zap.NewProduction()
	require.NoError(t, err)
	settings.TelemetrySettings = component.TelemetrySettings{Logger: logger}

	scraper := newPostgreSQLScraper(settings, cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	scraper.newestQueryTimestamp = 123440.111

	mock.ExpectQuery(expectedScrapeSampleQuery).WillReturnRows(sqlmock.NewRows(querySampleColumns))

	actualLogs, err := scraper.scrapeQuerySamples(t.Context(), 30)
	assert.NoError(t, err)

	totalRecords := 0
	for i := 0; i < actualLogs.ResourceLogs().Len(); i++ {
		rl := actualLogs.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			totalRecords += rl.ScopeLogs().At(j).LogRecords().Len()
		}
	}
	assert.Equal(t, 0, totalRecords)
}

func TestScrapeQuerySampleMultipleRows(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{}
	cfg.Events.DbServerQuerySample.Enabled = true
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)

	defer db.Close()

	factory := mockSimpleClientFactory{db: db}

	settings := receivertest.NewNopSettings(metadata.Type)
	logger, err := zap.NewProduction()
	require.NoError(t, err)
	settings.TelemetrySettings = component.TelemetrySettings{Logger: logger}

	scraper := newPostgreSQLScraper(settings, cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	scraper.newestQueryTimestamp = 123440.111

	row1 := map[string]any{
		querySampleColumnDatname:              "postgres",
		querySampleColumnUsename:              "user1",
		querySampleColumnClientAddr:           "10.0.0.1",
		querySampleColumnClientHostname:       "host1",
		querySampleColumnClientPort:           "5432",
		querySampleColumnQueryStart:           "2025-02-12T16:37:54.843+08:00",
		querySampleColumnQueryID:              "111",
		querySampleColumnPID:                  "1001",
		querySampleColumnApplicationName:      "app1",
		querySampleColumnQueryStartTimestamp:  "123445.123",
		querySampleColumnState:                "active",
		querySampleColumnQuery:                "SELECT * FROM orders WHERE status = 'pending'",
		querySampleColumnDurationMilliseconds: "5.3",
	}
	row2 := map[string]any{
		querySampleColumnDatname:              "postgres",
		querySampleColumnUsename:              "user2",
		querySampleColumnClientAddr:           "10.0.0.2",
		querySampleColumnClientHostname:       "host2",
		querySampleColumnClientPort:           "5433",
		querySampleColumnQueryStart:           "2025-02-12T16:38:00.000+08:00",
		querySampleColumnQueryID:              "222",
		querySampleColumnPID:                  "1002",
		querySampleColumnApplicationName:      "app2",
		querySampleColumnQueryStartTimestamp:  "123450.000",
		querySampleColumnState:                "idle",
		querySampleColumnQuery:                "UPDATE users SET last_login = now() WHERE id = 42",
		querySampleColumnDurationMilliseconds: "12.7",
	}

	rows := sqlmock.NewRows(querySampleColumns)
	for _, rowData := range []map[string]any{row1, row2} {
		rowValues := make([]driver.Value, len(querySampleColumns))
		for i, col := range querySampleColumns {
			if v, ok := rowData[col]; ok {
				rowValues[i] = v
				continue
			}
			rowValues[i] = ""
		}
		rows.AddRow(rowValues...)
	}

	mock.ExpectQuery(expectedScrapeSampleQuery).WillReturnRows(rows)

	actualLogs, err := scraper.scrapeQuerySamples(t.Context(), 30)
	assert.NoError(t, err)

	require.Equal(t, 1, actualLogs.ResourceLogs().Len())
	rl := actualLogs.ResourceLogs().At(0)
	require.Equal(t, 1, rl.ScopeLogs().Len())
	sl := rl.ScopeLogs().At(0)
	assert.Equal(t, 2, sl.LogRecords().Len())
}

//go:embed testdata/scraper/top-query/expectedSql.sql
var expectedScrapeTopQuery string

//go:embed testdata/scraper/top-query/expectedExplain.sql
var expectedExplain string

func TestScrapeTopQueries(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{}
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
	var expectedValuesBuilder strings.Builder
	for k, v := range expectedReturnedValue {
		expectedRows = append(expectedRows, k)
		fmt.Fprintf(&expectedValuesBuilder, "%s,", v)
	}
	expectedValues := expectedValuesBuilder.String()

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
	actualLogs, err := scraper.scrapeTopQuery(t.Context(), 31, 32, 33, time.Minute)
	assert.NoError(t, err)
	expectedFile := filepath.Join("testdata", "scraper", "top-query", "expected.yaml")
	expectedLogs, err := golden.ReadLogs(expectedFile)
	require.NoError(t, err)
	// golden.WriteLogs(t, expectedFile, actualLogs)
	errs := plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreResourceAttributeValue("service.instance.id"), plogtest.IgnoreTimestamp())
	assert.NoError(t, errs)

	// Verify the cache has updated with latest counter

	calls, callsExists := scraper.cache.Get(queryid + callsColumnName)
	assert.True(t, callsExists)
	assert.Equal(t, float64(123), calls)
	execTime, execTimeExists := scraper.cache.Get(queryid + totalExecTimeColumnName)
	assert.True(t, execTimeExists)
	assert.Equal(t, float64(11), execTime)
	planTime, planTimeExists := scraper.cache.Get(queryid + totalPlanTimeColumnName)
	assert.True(t, planTimeExists)
	assert.Equal(t, float64(12), planTime)
}

func TestIsExplainableQuery(t *testing.T) {
	testCases := []struct {
		name     string
		query    string
		expected bool
	}{
		// Explainable queries (whitelist: SELECT, TABLE, DELETE, INSERT, UPDATE, WITH, MERGE, VALUES)
		{name: "simple SELECT", query: "SELECT * FROM users", expected: true},
		{name: "SELECT with WHERE", query: "SELECT id, name FROM users WHERE active = true", expected: true},
		{name: "INSERT", query: "INSERT INTO users (name) VALUES ('test')", expected: true},
		{name: "UPDATE", query: "UPDATE users SET name = 'test' WHERE id = 1", expected: true},
		{name: "DELETE", query: "DELETE FROM users WHERE id = 1", expected: true},
		{name: "SELECT with leading whitespace", query: "   SELECT * FROM users", expected: true},
		{name: "WITH CTE", query: "WITH cte AS (SELECT * FROM users) SELECT * FROM cte", expected: true},
		{name: "TABLE shorthand", query: "TABLE users", expected: true},
		{name: "VALUES", query: "VALUES (1, 'a'), (2, 'b')", expected: true},
		{name: "MERGE", query: "MERGE INTO target USING source ON target.id = source.id WHEN MATCHED THEN UPDATE SET name = source.name", expected: true},
		{name: "SELECT after comment", query: "-- comment\nSELECT * FROM users", expected: true},
		{name: "SELECT after multi-line comment", query: "/* comment */ SELECT * FROM users", expected: true},
		{name: "SELECT with parenthesis", query: "(SELECT * FROM users)", expected: false}, // Subquery alone - first char is '('

		// Non-explainable queries (anything not in whitelist)
		{name: "GRANT", query: "GRANT SELECT ON pg_locks TO demo", expected: false},
		{name: "REVOKE", query: "REVOKE ALL ON FUNCTION pg_stat_statements_reset() FROM PUBLIC", expected: false},
		{name: "DROP FUNCTION", query: "DROP FUNCTION pg_stat_statements_reset(Oid, Oid, bigint)", expected: false},
		{name: "DROP TRIGGER", query: "DROP TRIGGER IF EXISTS update_users_updated_at ON users", expected: false},
		{name: "DROP TABLE", query: "DROP TABLE users", expected: false},
		{name: "CREATE INDEX", query: "CREATE INDEX idx_users_name ON users(name)", expected: false},
		{name: "CREATE EXTENSION", query: "CREATE EXTENSION IF NOT EXISTS pg_stat_statements", expected: false},
		{name: "CREATE FUNCTION", query: "CREATE FUNCTION my_func() RETURNS void AS $$ BEGIN END; $$ LANGUAGE plpgsql", expected: false},
		{name: "CREATE TRIGGER", query: "CREATE TRIGGER update_users BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at()", expected: false},
		{name: "CREATE TABLE", query: "CREATE TABLE users (id INT)", expected: false},
		{name: "CREATE TABLE AS SELECT", query: "CREATE TABLE new_table AS SELECT * FROM old_table", expected: false}, // CREATE not in whitelist
		{name: "ALTER TABLE", query: "ALTER TABLE users ADD COLUMN email VARCHAR(255)", expected: false},
		{name: "TRUNCATE", query: "TRUNCATE TABLE users", expected: false},
		{name: "SET", query: "SET plan_cache_mode = force_generic_plan", expected: false},
		{name: "COMMENT", query: "COMMENT ON TABLE users IS 'User table'", expected: false},
		{name: "VACUUM", query: "VACUUM ANALYZE users", expected: false},
		{name: "ANALYZE", query: "ANALYZE users", expected: false},
		{name: "BEGIN", query: "BEGIN", expected: false},
		{name: "COMMIT", query: "COMMIT", expected: false},
		{name: "ROLLBACK", query: "ROLLBACK", expected: false},
		{name: "COPY", query: "COPY users FROM '/tmp/data.csv'", expected: false},
		{name: "EXPLAIN", query: "EXPLAIN SELECT * FROM users", expected: false}, // EXPLAIN itself not in whitelist
		{name: "PREPARE", query: "PREPARE stmt AS SELECT * FROM users", expected: false},
		{name: "EXECUTE", query: "EXECUTE stmt", expected: false},
		{name: "DEALLOCATE", query: "DEALLOCATE stmt", expected: false},

		// DDL with leading comments (should still be detected as non-explainable)
		{name: "GRANT after comment", query: "-- Grant permissions\nGRANT SELECT ON users TO demo", expected: false},
		{name: "DROP after multi-line comment", query: "/* cleanup */ DROP TABLE old_table", expected: false},
		{name: "REVOKE after nested comments", query: "/* comment */ -- another\nREVOKE ALL FROM demo", expected: false},

		// Edge cases
		{name: "empty query", query: "", expected: false},
		{name: "only whitespace", query: "   ", expected: false},
		{name: "only comment", query: "-- just a comment", expected: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isExplainableQuery(tc.query)
			assert.Equal(t, tc.expected, result, "query: %s", tc.query)
		})
	}
}

func TestScrapeTopQueriesCollectsOnlyWhenIntervalHasElapsed(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Databases = []string{}
	cfg.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.CollectionInterval = 600 * time.Second
	db, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
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

	scraper := newPostgreSQLScraper(settings, cfg, factory, newCache(30), newTTLCache[string](1, time.Second))

	assert.True(t, scraper.lastExecutionTimestamp.IsZero(), "lastExecutionTimestamp should be zero before first collection")
	logs1, err := scraper.scrapeTopQuery(t.Context(), 31, 32, 33, time.Minute)
	assert.NotNil(t, logs1)
	assert.NoError(t, err)
	assert.False(t, scraper.lastExecutionTimestamp.IsZero(), "lastExecutionTimestamp won't be zero after first collection")

	collectionTime := scraper.lastExecutionTimestamp
	logs2, err := scraper.scrapeTopQuery(t.Context(), 31, 32, 33, time.Minute)
	assert.NotNil(t, logs2)
	assert.NoError(t, err)
	assert.Equal(t, collectionTime, scraper.lastExecutionTimestamp, "No new collection should happen until configured collection_interval")
}

func TestIsCollectionDue(t *testing.T) {
	collectionInterval := 20 * time.Second
	currentCollectionTime := time.Now()

	logger, err := zap.NewProduction()
	require.NoError(t, err)
	scrpr := postgreSQLScraper{
		// setting lastExecutionTimestamp to be past 'collectionInterval'
		lastExecutionTimestamp: currentCollectionTime.Add(-collectionInterval),
		logger:                 logger,
	}
	isCollectionDue := scrpr.isCollectionDue(currentCollectionTime, collectionInterval)
	assert.True(t, isCollectionDue, "lastExecutionTimestamp is older than collection_interval, so collection should be due.")

	scrpr.lastExecutionTimestamp = currentCollectionTime.Add(-10 * time.Second)
	isCollectionDue = scrpr.isCollectionDue(currentCollectionTime, collectionInterval)
	assert.False(t, isCollectionDue, "collection_interval is not yet reached since lastExecutionTimestamp, so collection is not due.")
}

func TestExplainQuery(t *testing.T) {
	testCases := []struct {
		name           string
		query          string
		queryID        string
		expectedSQL    string
		mockPlanResult string
	}{
		{
			name:           "query with no parameters",
			query:          "SELECT * FROM users",
			queryID:        "12345",
			expectedSQL:    "/* otel-collector-ignore */ SET plan_cache_mode = force_generic_plan;PREPARE otel_12345 AS SELECT * FROM users;EXPLAIN(FORMAT JSON) EXECUTE otel_12345;",
			mockPlanResult: `[{"Plan":{"Node Type":"Seq Scan","Relation Name":"users"}}]`,
		},
		{
			name:           "query with single parameter",
			query:          "SELECT * FROM users WHERE id = $1",
			queryID:        "12346",
			expectedSQL:    "/* otel-collector-ignore */ SET plan_cache_mode = force_generic_plan;PREPARE otel_12346 AS SELECT * FROM users WHERE id = $1;EXPLAIN(FORMAT JSON) EXECUTE otel_12346(null);",
			mockPlanResult: `[{"Plan":{"Node Type":"Index Scan","Relation Name":"users"}}]`,
		},
		{
			name:           "query with multiple parameters",
			query:          "SELECT * FROM orders WHERE user_id = $1 AND status = $2 AND created_at > $3",
			queryID:        "12347",
			expectedSQL:    "/* otel-collector-ignore */ SET plan_cache_mode = force_generic_plan;PREPARE otel_12347 AS SELECT * FROM orders WHERE user_id = $1 AND status = $2 AND created_at > $3;EXPLAIN(FORMAT JSON) EXECUTE otel_12347(null, null, null);",
			mockPlanResult: `[{"Plan":{"Node Type":"Index Scan","Relation Name":"orders"}}]`,
		},
		{
			name:           "query with hyphenated queryID",
			query:          "SELECT * FROM products WHERE id = $1",
			queryID:        "abc-def-123",
			expectedSQL:    "/* otel-collector-ignore */ SET plan_cache_mode = force_generic_plan;PREPARE otel_abc_def_123 AS SELECT * FROM products WHERE id = $1;EXPLAIN(FORMAT JSON) EXECUTE otel_abc_def_123(null);",
			mockPlanResult: `[{"Plan":{"Node Type":"Index Scan","Relation Name":"products"}}]`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			logger, err := zap.NewProduction()
			require.NoError(t, err)

			client := &postgreSQLClient{
				client:  db,
				closeFn: func() error { return nil },
			}

			// Expect the EXPLAIN query
			mock.ExpectQuery(tc.expectedSQL).WillReturnRows(
				sqlmock.NewRows([]string{"QUERY PLAN"}).AddRow(tc.mockPlanResult),
			)

			plan, err := client.explainQuery(tc.query, tc.queryID, logger)
			require.NoError(t, err)
			assert.Equal(t, tc.mockPlanResult, plan)
		})
	}
}

type (
	mockClientFactory       struct{ mock.Mock }
	mockClient              struct{ mock.Mock }
	mockSimpleClientFactory struct {
		db *sql.DB
	}
)

// explainQuery implements client.
func (*mockClient) explainQuery(string, string, *zap.Logger) (string, error) {
	panic("unimplemented")
}

// getTopQuery implements client.
func (*mockClient) getTopQuery(context.Context, int64, *zap.Logger) ([]map[string]any, error) {
	panic("unimplemented")
}

// close implements postgreSQLClientFactory.
func (mockSimpleClientFactory) close() error {
	return nil
}

// getClient implements postgreSQLClientFactory.
func (m mockSimpleClientFactory) getClient(string) (client, error) {
	return &postgreSQLClient{
		client:  m.db,
		closeFn: m.close,
	}, nil
}

// getQuerySamples implements client.
func (*mockClient) getQuerySamples(context.Context, int64, float64, *zap.Logger) ([]map[string]any, float64, error) {
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

func (m *mockClient) getTableCount(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockClient) getBlocksReadByTable(ctx context.Context, database string) (map[tableIdentifier]tableIOStats, error) {
	args := m.Called(ctx, database)
	return args.Get(0).(map[tableIdentifier]tableIOStats), args.Error(1)
}

func (m *mockClient) getIndexStats(ctx context.Context, database string) (map[indexIdentifer]indexStat, error) {
	args := m.Called(ctx, database)
	return args.Get(0).(map[indexIdentifer]indexStat), args.Error(1)
}

func (m *mockClient) getFunctionStats(ctx context.Context, database string) (map[functionIdentifer]functionStat, error) {
	args := m.Called(ctx, database)
	return args.Get(0).(map[functionIdentifer]functionStat), args.Error(1)
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

func (m *mockClient) initMocks(database, schema string, databases []string, index int) {
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
				tempIo:               int64(idx + 12),
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
		m.On("getTableCount", mock.Anything).Return(int64(len(tableMetrics)), nil)
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

		function1 := "test_function1"
		function2 := "test_function2"
		functionStats := map[functionIdentifer]functionStat{
			functionKey(database, schema, function1): {
				database: database,
				schema:   schema,
				function: function1,
				calls:    int64(index + 50),
			},
			functionKey(database, schema, function2): {
				database: database,
				schema:   schema,
				function: function2,
				calls:    int64(index + 51),
			},
		}
		m.On("getFunctionStats", mock.Anything, database).Return(functionStats, nil)
	}
}

func TestGetInstanceId(t *testing.T) {
	localhostName, _ := os.Hostname()

	instanceString := "example.com:5432"
	instanceID := getInstanceID(instanceString, zap.NewNop())
	assert.Equal(t, "example.com:5432", instanceID)

	localHostStringUppercase := "Localhost:5432"
	localInstanceID := getInstanceID(localHostStringUppercase, zap.NewNop())
	assert.NotNil(t, localInstanceID)
	assert.Equal(t, localhostName+":5432", localInstanceID)

	localHostString := "127.0.0.1:5432"
	localInstanceID = getInstanceID(localHostString, zap.NewNop())
	assert.NotNil(t, localInstanceID)
	assert.Equal(t, localhostName+":5432", localInstanceID)

	localHostStringIPV6 := "[::1]:5432"
	localInstanceID = getInstanceID(localHostStringIPV6, zap.NewNop())
	assert.NotNil(t, localInstanceID)
	assert.Equal(t, localhostName+":5432", localInstanceID)

	hostNameErrorSample := ""
	localInstanceID = getInstanceID(hostNameErrorSample, zap.NewNop())
	assert.NotNil(t, localInstanceID)
	assert.Equal(t, "unknown:5432", localInstanceID)
}
