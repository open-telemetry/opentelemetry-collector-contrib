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
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pcommon"
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

	scraper, err := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, newDefaultClientFactory(cfg), newCache(1), newTTLCache[string](1, time.Second))
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(t.Context())
	require.Error(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(pmetric.NewMetrics(), actualMetrics))
}

func TestMetricsBuilderConfigForFeatureGate(t *testing.T) {
	cfg := metadata.NewDefaultMetricsBuilderConfig()

	semconvConfig := metricsBuilderConfigForFeatureGate(cfg, true)
	assert.Equal(t, cfg, semconvConfig)

	legacyConfig := metricsBuilderConfigForFeatureGate(cfg, false)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlBackends.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlBlksHit.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlBlksRead.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlCommits.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlDbSize.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlDeadlocks.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlIndexScans.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlIndexSize.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlRollbacks.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlSequentialScans.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlTableCount.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlTableSize.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlTableVacuumCount.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlTempIo.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlTempFiles.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlTupDeleted.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlTupFetched.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlTupInserted.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlTupReturned.EnabledAttributes)
	assert.Empty(t, legacyConfig.Metrics.PostgresqlTupUpdated.EnabledAttributes)
	assert.Equal(t, []metadata.PostgresqlBlocksReadMetricAttributeKey{metadata.PostgresqlBlocksReadMetricAttributeKeySource}, legacyConfig.Metrics.PostgresqlBlocksRead.EnabledAttributes)
	assert.Equal(t, []metadata.PostgresqlFunctionCallsMetricAttributeKey{metadata.PostgresqlFunctionCallsMetricAttributeKeyFunction}, legacyConfig.Metrics.PostgresqlFunctionCalls.EnabledAttributes)
	assert.Equal(t, []metadata.PostgresqlOperationsMetricAttributeKey{metadata.PostgresqlOperationsMetricAttributeKeyOperation}, legacyConfig.Metrics.PostgresqlOperations.EnabledAttributes)
	assert.Equal(t, []metadata.PostgresqlQueryConflictsMetricAttributeKey{metadata.PostgresqlQueryConflictsMetricAttributeKeyPostgresqlConflictType}, legacyConfig.Metrics.PostgresqlQueryConflicts.EnabledAttributes)
	assert.Equal(t, []metadata.PostgresqlRowsMetricAttributeKey{metadata.PostgresqlRowsMetricAttributeKeyState}, legacyConfig.Metrics.PostgresqlRows.EnabledAttributes)
	assert.Equal(t, cfg.Metrics.PostgresqlDatabaseLocks.EnabledAttributes, legacyConfig.Metrics.PostgresqlDatabaseLocks.EnabledAttributes)
	assert.Equal(t, cfg.Metrics.PostgresqlReplicationDataDelay.EnabledAttributes, legacyConfig.Metrics.PostgresqlReplicationDataDelay.EnabledAttributes)
	assert.Equal(t, cfg.Metrics.PostgresqlWalDelay.EnabledAttributes, legacyConfig.Metrics.PostgresqlWalDelay.EnabledAttributes)
	assert.Equal(t, cfg.Metrics.PostgresqlWalLag.EnabledAttributes, legacyConfig.Metrics.PostgresqlWalLag.EnabledAttributes)
	assert.NotEmpty(t, cfg.Metrics.PostgresqlBackends.EnabledAttributes)
	assert.Contains(t, cfg.Metrics.PostgresqlQueryConflicts.EnabledAttributes, metadata.PostgresqlQueryConflictsMetricAttributeKeyDbNamespace)
	assert.Equal(t, metadata.NewDefaultMetricsBuilderConfig(), cfg)

	customConfig := cfg
	customConfig.Metrics.PostgresqlBlocksRead.EnabledAttributes = []metadata.PostgresqlBlocksReadMetricAttributeKey{metadata.PostgresqlBlocksReadMetricAttributeKeyDbNamespace}
	customLegacyConfig := metricsBuilderConfigForFeatureGate(customConfig, false)
	assert.Empty(t, customLegacyConfig.Metrics.PostgresqlBlocksRead.EnabledAttributes)
	assert.Equal(t, []metadata.PostgresqlBlocksReadMetricAttributeKey{metadata.PostgresqlBlocksReadMetricAttributeKeyDbNamespace}, customConfig.Metrics.PostgresqlBlocksRead.EnabledAttributes)
}

func TestSemconvQueryConflictsPreserveDatabaseNamespace(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Metrics.PostgresqlQueryConflicts.Enabled = true
	scraper := &postgreSQLScraper{
		config:            cfg,
		mb:                metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, receivertest.NewNopSettings(metadata.Type)),
		serviceInstanceID: "example.com:5432",
		useOTelSemconv:    true,
	}
	retrieval := &dbRetrieval{
		dbConflictStats: map[databaseName]databaseConflictStats{
			"orders": {
				conflTablespace: 1,
				conflLock:       2,
				conflSnapshot:   3,
				conflBufferpin:  4,
				conflDeadlock:   5,
			},
			"users": {
				conflTablespace: 6,
				conflLock:       7,
				conflSnapshot:   8,
				conflBufferpin:  9,
				conflDeadlock:   10,
			},
		},
	}

	now := pcommon.NewTimestampFromTime(time.Unix(0, 1))
	scraper.recordDatabase(now, "orders", retrieval, 0)
	scraper.recordDatabase(now, "users", retrieval, 0)
	rb := scraper.setupSemconvResourceBuilder(scraper.mb.NewResourceBuilder())
	metrics := scraper.mb.Emit(metadata.WithResource(rb.Emit()))

	queryConflicts := pmetric.NewMetric()
	found := false
	resourceMetrics := metrics.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		scopeMetrics := resourceMetrics.At(i).ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			metricSlice := scopeMetrics.At(j).Metrics()
			for k := 0; k < metricSlice.Len(); k++ {
				if metricSlice.At(k).Name() == "postgresql.query.conflicts" {
					queryConflicts = metricSlice.At(k)
					found = true
				}
			}
		}
	}
	require.True(t, found)

	actual := map[string]int64{}
	dataPoints := queryConflicts.Sum().DataPoints()
	require.Equal(t, 10, dataPoints.Len())
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)
		namespace, ok := dp.Attributes().Get("db.namespace")
		require.True(t, ok)
		conflictType, ok := dp.Attributes().Get("postgresql.conflict.type")
		require.True(t, ok)
		actual[namespace.Str()+"/"+conflictType.Str()] = dp.IntValue()
	}
	require.Equal(t, map[string]int64{
		"orders/tablespace": 1,
		"orders/lock":       2,
		"orders/snapshot":   3,
		"orders/bufferpin":  4,
		"orders/deadlock":   5,
		"users/tablespace":  6,
		"users/lock":        7,
		"users/snapshot":    8,
		"users/bufferpin":   9,
		"users/deadlock":    10,
	}, actual)
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
		cfg.Metrics.PostgresqlQueryConflicts.Enabled = true

		scraper, err := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
		require.NoError(t, err)

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
		require.False(t, cfg.Metrics.PostgresqlQueryConflicts.Enabled)
		cfg.Metrics.PostgresqlQueryConflicts.Enabled = true

		scraper, err := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
		require.NoError(t, err)
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
		cfg.Metrics.PostgresqlQueryConflicts.Enabled = false

		scraper, err = newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
		require.NoError(t, err)
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
		scraper, err := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))
		require.NoError(t, err)

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

	runTest := func(separateSchemaAttr, useOTelSemconv bool, file string) {
		defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, separateSchemaAttr)()
		defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlUseOTelSemconvFeatureGate, useOTelSemconv)()

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
		scraper, err := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))
		require.NoError(t, err)

		actualMetrics, err := scraper.scrape(t.Context())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "multiple", file)
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)
		compareOpts := []pmetrictest.CompareMetricsOption{
			pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		}
		if useOTelSemconv {
			compareOpts = append(compareOpts,
				pmetrictest.IgnoreResourceAttributeValue("server.address"),
				pmetrictest.IgnoreResourceAttributeValue("server.port"),
			)
		}
		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, compareOpts...))
	}

	runTest(true, false, "expected_schemaattr.yaml")
	runTest(false, false, "expected.yaml")
	runTest(false, true, "expected_semconv.yaml")
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

		scraper, err := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))
		require.NoError(t, err)

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
		require.False(t, cfg.Metrics.PostgresqlQueryConflicts.Enabled)
		cfg.Metrics.PostgresqlQueryConflicts.Enabled = true
		scraper, err := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))
		require.NoError(t, err)

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

		scraper, err := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, &factory, newCache(1), newTTLCache[string](1, time.Second))
		require.NoError(t, err)

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

func TestMutualExclusionOfFeatureGates(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, true)()
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlUseOTelSemconvFeatureGate, true)()

	cfg := createDefaultConfig().(*Config)
	factory := new(mockClientFactory)
	factory.initMocks([]string{"otel"})

	_, err := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutually exclusive")
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
	querySampleColumnBlockingPIDs,
	querySampleColumnBlockingStartTime,
	querySampleColumnBlockingWaitDuration,
	querySampleColumnBlockingLockMode,
	querySampleColumnBlockingLockType,
	querySampleColumnBlockingLockRelation,
	querySampleColumnBlockingTxnStartTime,
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
	scraper, scraperErr := newPostgreSQLScraper(settings, cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	require.NoError(t, scraperErr)
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
		querySampleColumnBlockingPIDs:         "{}",
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

func TestScrapeQuerySampleSemconv(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlUseOTelSemconvFeatureGate, true)()

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

	scraper, scraperErr := newPostgreSQLScraper(settings, cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	require.NoError(t, scraperErr)
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
		querySampleColumnBlockingPIDs:         "{}",
	}))

	actualLogs, err := scraper.scrapeQuerySamples(t.Context(), 30)
	require.NoError(t, err)
	expectedFile := filepath.Join("testdata", "scraper", "query-sample", "expected_semconv.yaml")
	expectedLogs, err := golden.ReadLogs(expectedFile)
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expectedLogs, actualLogs,
		plogtest.IgnoreResourceAttributeValue("service.instance.id"),
		plogtest.IgnoreResourceAttributeValue("server.address"),
		plogtest.IgnoreResourceAttributeValue("server.port"),
		plogtest.IgnoreTimestamp(),
	))
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

	scraper, scraperErr := newPostgreSQLScraper(settings, cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	require.NoError(t, scraperErr)
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
		querySampleColumnBlockingPIDs:         "{}",
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

	scraper, scraperErr := newPostgreSQLScraper(settings, cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	require.NoError(t, scraperErr)
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

	scraper, scraperErr := newPostgreSQLScraper(settings, cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	require.NoError(t, scraperErr)
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

func TestScrapeQuerySampleBlockedSession(t *testing.T) {
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

	scraper, scraperErr := newPostgreSQLScraper(settings, cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	require.NoError(t, scraperErr)
	scraper.newestQueryTimestamp = 123440.111

	mock.ExpectQuery(expectedScrapeSampleQuery).WillReturnRows(newQuerySampleRows(t, map[string]any{
		querySampleColumnDatname:              "postgres",
		querySampleColumnUsename:              "otelu",
		querySampleColumnClientAddr:           "11.4.5.14",
		querySampleColumnClientHostname:       "otel",
		querySampleColumnClientPort:           "114514",
		querySampleColumnQueryStart:           "2025-02-12T16:37:54.843+08:00",
		querySampleColumnQueryID:              "999",
		querySampleColumnPID:                  "2500",
		querySampleColumnApplicationName:      "app",
		querySampleColumnQueryStartTimestamp:  "123445.123",
		querySampleColumnState:                "active",
		querySampleColumnQuery:                "update orders set status = ? where id = ?",
		querySampleColumnDurationMilliseconds: "42.0",
		querySampleColumnBlockingPIDs:         "{3001}",
		querySampleColumnBlockingStartTime:    "2025-02-12T16:37:50Z",
		querySampleColumnBlockingWaitDuration: "42",
		querySampleColumnBlockingLockMode:     "AccessExclusiveLock",
		querySampleColumnBlockingLockType:     "relation",
		querySampleColumnBlockingLockRelation: "orders",
		querySampleColumnBlockingTxnStartTime: "2025-02-12T16:37:49Z",
	}))

	actualLogs, err := scraper.scrapeQuerySamples(t.Context(), 30)
	require.NoError(t, err)
	require.Equal(t, 1, actualLogs.ResourceLogs().Len())
	lr := actualLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs := lr.Attributes().AsRaw()
	assert.Equal(t, "{3001}", attrs["postgresql.blocking.pids"])
	assert.Equal(t, "2025-02-12T16:37:50Z", attrs["postgresql.blocking.start_time"])
	assert.Equal(t, int64(42), attrs["postgresql.blocking.wait_duration"])
	assert.Equal(t, "AccessExclusiveLock", attrs["postgresql.blocking.lock.mode"])
	assert.Equal(t, "relation", attrs["postgresql.blocking.lock.type"])
	assert.Equal(t, "orders", attrs["postgresql.blocking.lock.relation"])
	assert.Equal(t, "2025-02-12T16:37:49Z", attrs["postgresql.blocking.transaction.start_time"])
}

func TestScrapeQuerySampleMultiBlocker(t *testing.T) {
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

	scraper, scraperErr := newPostgreSQLScraper(settings, cfg, factory, newCache(1), newTTLCache[string](1, time.Second))
	require.NoError(t, scraperErr)
	scraper.newestQueryTimestamp = 123440.111

	mock.ExpectQuery(expectedScrapeSampleQuery).WillReturnRows(newQuerySampleRows(t, map[string]any{
		querySampleColumnDatname:              "postgres",
		querySampleColumnUsename:              "otelu",
		querySampleColumnClientAddr:           "11.4.5.14",
		querySampleColumnClientHostname:       "otel",
		querySampleColumnClientPort:           "5432",
		querySampleColumnQueryStart:           "2025-02-12T16:37:54.843+08:00",
		querySampleColumnQueryID:              "888",
		querySampleColumnPID:                  "2600",
		querySampleColumnApplicationName:      "app",
		querySampleColumnQueryStartTimestamp:  "123445.123",
		querySampleColumnState:                "active",
		querySampleColumnQuery:                "update orders set status = ? where id = ?",
		querySampleColumnDurationMilliseconds: "10.0",
		querySampleColumnBlockingPIDs:         "{3001,3002}",
		querySampleColumnBlockingStartTime:    "2025-02-12T16:37:50Z",
		querySampleColumnBlockingWaitDuration: "10",
		querySampleColumnBlockingLockMode:     "RowExclusiveLock",
		querySampleColumnBlockingLockType:     "relation",
		querySampleColumnBlockingLockRelation: "orders",
		querySampleColumnBlockingTxnStartTime: "2025-02-12T16:37:49Z",
	}))

	actualLogs, err := scraper.scrapeQuerySamples(t.Context(), 30)
	require.NoError(t, err)
	require.Equal(t, 1, actualLogs.ResourceLogs().Len())
	lr := actualLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	attrs := lr.Attributes().AsRaw()
	assert.Equal(t, "{3001,3002}", attrs["postgresql.blocking.pids"])
	assert.Equal(t, "2025-02-12T16:37:50Z", attrs["postgresql.blocking.start_time"])
	assert.Equal(t, int64(10), attrs["postgresql.blocking.wait_duration"])
	assert.Equal(t, "RowExclusiveLock", attrs["postgresql.blocking.lock.mode"])
	assert.Equal(t, "relation", attrs["postgresql.blocking.lock.type"])
	assert.Equal(t, "orders", attrs["postgresql.blocking.lock.relation"])
	assert.Equal(t, "2025-02-12T16:37:49Z", attrs["postgresql.blocking.transaction.start_time"])
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

	scraper, scraperErr := newPostgreSQLScraper(settings, cfg, factory, newCache(30), newTTLCache[string](1, time.Second))
	require.NoError(t, scraperErr)
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

	scraper, scraperErr := newPostgreSQLScraper(settings, cfg, factory, newCache(30), newTTLCache[string](1, time.Second))
	require.NoError(t, scraperErr)

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

func (m *mockClient) getDatabaseConflicts(_ context.Context, databases []string) (map[databaseName]databaseConflictStats, error) {
	args := m.Called(databases)
	return args.Get(0).(map[databaseName]databaseConflictStats), args.Error(1)
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
		dbConflictStats := map[databaseName]databaseConflictStats{}
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
			dbConflictStats[databaseName(db)] = databaseConflictStats{
				conflTablespace: int64(idx + 13),
				conflLock:       int64(idx + 14),
				conflSnapshot:   int64(idx + 15),
				conflBufferpin:  int64(idx + 16),
				conflDeadlock:   int64(idx + 17),
			}
			dbSize[databaseName(db)] = int64(idx + 4)
			backends[databaseName(db)] = int64(idx + 3)
		}

		m.On("getDatabaseStats", databases).Return(dbStats, nil)
		m.On("getDatabaseConflicts", databases).Return(dbConflictStats, nil)
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

func TestResolveServiceInstanceSeed(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)
	localEndpoint := net.JoinHostPort(hostname, "5432")

	tests := []struct {
		name     string
		endpoint string
		expected string
	}{
		{
			name:     "hostname",
			endpoint: "db.example.com:5432",
			expected: "db.example.com:5432",
		},
		{
			name:     "IPv6",
			endpoint: "[2001:db8::1]:5432",
			expected: "[2001:db8::1]:5432",
		},
		{
			name:     "localhost",
			endpoint: "localhost:5432",
			expected: localEndpoint,
		},
		{
			name:     "case-insensitive localhost",
			endpoint: "Localhost:5432",
			expected: localEndpoint,
		},
		{
			name:     "IPv4 loopback",
			endpoint: "127.0.0.1:5432",
			expected: localEndpoint,
		},
		{
			name:     "IPv6 loopback",
			endpoint: "[::1]:5432",
			expected: localEndpoint,
		},
		{
			name:     "invalid endpoint",
			endpoint: "localhost",
			expected: "localhost",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Endpoint = tt.endpoint
			assert.Equal(t, tt.expected, resolveServiceInstanceSeed(cfg, zap.NewNop()))
		})
	}
}

func TestResolveUnixServiceInstanceSeed(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)
	unixSeed := func(endpoint string) string {
		cfg := createDefaultConfig().(*Config)
		cfg.Endpoint = endpoint
		cfg.Transport = confignet.TransportTypeUnix
		return resolveServiceInstanceSeed(cfg, zap.NewNop())
	}

	seed := unixSeed("var/run/postgresql:5432")
	assert.Equal(t, strings.Join([]string{"unix", hostname, "/var/run/postgresql/.s.PGSQL.5432"}, "\x00"), seed)
	assert.Equal(t, seed, unixSeed("/var/run/postgresql:5432"))
	assert.NotEqual(t, seed, unixSeed("var/lib/postgresql:5432"))
	assert.NotEqual(t, seed, unixSeed("var/run/postgresql:5433"))
	assert.Equal(t, strings.Join([]string{"unix", hostname, "var/run/postgresql"}, "\x00"), unixSeed("var/run/postgresql"))

	tcpConfig := createDefaultConfig().(*Config)
	tcpConfig.Endpoint = "var/run/postgresql:5432"
	assert.NotEqual(t, seed, resolveServiceInstanceSeed(tcpConfig, zap.NewNop()))
}

func TestNewPostgreSQLScraperSemconvServiceInstanceID(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlUseOTelSemconvFeatureGate, true)()

	hostname, err := os.Hostname()
	require.NoError(t, err)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "[::1]:5432"
	scraper, err := newPostgreSQLScraper(
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		newDefaultClientFactory(cfg),
		newCache(1),
		newTTLCache[string](1, time.Second),
	)
	require.NoError(t, err)

	seed := net.JoinHostPort(hostname, "5432")
	expected := uuid.NewSHA1(otelNamespaceUUID, []byte(seed)).String()
	assert.Equal(t, expected, scraper.serviceInstanceID)
}

func TestNewPostgreSQLScraperSemconvUnixServiceInstanceID(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlUseOTelSemconvFeatureGate, true)()

	hostname, err := os.Hostname()
	require.NoError(t, err)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "var/run/postgresql:5432"
	cfg.Transport = confignet.TransportTypeUnix
	scraper, err := newPostgreSQLScraper(
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		newDefaultClientFactory(cfg),
		newCache(1),
		newTTLCache[string](1, time.Second),
	)
	require.NoError(t, err)

	seed := strings.Join([]string{"unix", hostname, "/var/run/postgresql/.s.PGSQL.5432"}, "\x00")
	expected := uuid.NewSHA1(otelNamespaceUUID, []byte(seed)).String()
	assert.Equal(t, expected, scraper.serviceInstanceID)
}

func TestSetupSemconvResourceBuilder(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "127.0.0.1:5432"
	serviceInstanceID := uuid.NewSHA1(otelNamespaceUUID, []byte("collector-host:5432")).String()
	scraper := &postgreSQLScraper{
		logger:            zap.NewNop(),
		config:            cfg,
		mb:                metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, receivertest.NewNopSettings(metadata.Type)),
		serviceInstanceID: serviceInstanceID,
		useOTelSemconv:    true,
	}

	rb := scraper.mb.NewResourceBuilder()
	scraper.setupSemconvResourceBuilder(rb)
	res := rb.Emit()

	serverHost, ok := res.Attributes().Get("server.address")
	require.True(t, ok)
	assert.Equal(t, "127.0.0.1", serverHost.Str())

	serverPort, ok := res.Attributes().Get("server.port")
	require.True(t, ok)
	assert.Equal(t, int64(5432), serverPort.Int())

	instanceID, ok := res.Attributes().Get("service.instance.id")
	require.True(t, ok)
	assert.Equal(t, serviceInstanceID, instanceID.Str())
	_, err := uuid.Parse(instanceID.Str())
	require.NoError(t, err)

	rb2 := scraper.mb.NewResourceBuilder()
	scraper.setupSemconvResourceBuilder(rb2)
	res2 := rb2.Emit()
	uuid2, _ := res2.Attributes().Get("service.instance.id")
	assert.Equal(t, instanceID.Str(), uuid2.Str())

	_, hasServiceName := res.Attributes().Get("service.name")
	assert.False(t, hasServiceName)
}

func TestServerEndpointAttributes(t *testing.T) {
	tests := []struct {
		name            string
		endpoint        string
		transport       confignet.TransportType
		expectedAddress string
		expectedPort    int64
		wantErr         bool
	}{
		{
			name:            "hostname",
			endpoint:        "db.example.com:5432",
			transport:       confignet.TransportTypeTCP,
			expectedAddress: "db.example.com",
			expectedPort:    5432,
		},
		{
			name:            "loopback IPv4",
			endpoint:        "127.0.0.1:5433",
			transport:       confignet.TransportTypeTCP,
			expectedAddress: "127.0.0.1",
			expectedPort:    5433,
		},
		{
			name:            "IPv6",
			endpoint:        "[2001:db8::1]:5434",
			transport:       confignet.TransportTypeTCP,
			expectedAddress: "2001:db8::1",
			expectedPort:    5434,
		},
		{
			name:            "Unix socket",
			endpoint:        "var/run/postgresql:5435",
			transport:       confignet.TransportTypeUnix,
			expectedAddress: "/var/run/postgresql/.s.PGSQL.5435",
			expectedPort:    5435,
		},
		{
			name:      "invalid endpoint",
			endpoint:  "localhost",
			transport: confignet.TransportTypeTCP,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Endpoint = tt.endpoint
			cfg.Transport = tt.transport
			address, port, err := serverEndpointAttributes(cfg)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedAddress, address)
			assert.Equal(t, tt.expectedPort, port)
		})
	}
}

func TestSetupLegacyResourceBuilder(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	scraper := &postgreSQLScraper{
		logger:            zap.NewNop(),
		config:            cfg,
		mb:                metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, receivertest.NewNopSettings(metadata.Type)),
		serviceInstanceID: "localhost:5432",
		useOTelSemconv:    false,
	}

	rb := scraper.mb.NewResourceBuilder()
	scraper.setupLegacyResourceBuilder(rb, "mydb", "myschema", "mytable", "myindex")
	res := rb.Emit()

	instanceID, ok := res.Attributes().Get("service.instance.id")
	require.True(t, ok)
	assert.Equal(t, "localhost:5432", instanceID.Str())

	dbName, ok := res.Attributes().Get("postgresql.database.name")
	require.True(t, ok)
	assert.Equal(t, "mydb", dbName.Str())

	schemaName, ok := res.Attributes().Get("postgresql.schema.name")
	require.True(t, ok)
	assert.Equal(t, "myschema", schemaName.Str())

	tableName, ok := res.Attributes().Get("postgresql.table.name")
	require.True(t, ok)
	assert.Equal(t, "mytable", tableName.Str())

	indexName, ok := res.Attributes().Get("postgresql.index.name")
	require.True(t, ok)
	assert.Equal(t, "myindex", indexName.Str())

	_, hasServerAddr := res.Attributes().Get("server.address")
	assert.False(t, hasServerAddr)
	_, hasServerPort := res.Attributes().Get("server.port")
	assert.False(t, hasServerPort)
}
