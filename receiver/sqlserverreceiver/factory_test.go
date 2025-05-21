// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"context"
	"os"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

func TestFactory(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "creates a new factory with correct type",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				require.Equal(t, metadata.Type, factory.Type())
			},
		},
		{
			desc: "creates a new factory with valid default config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()

				var expectedCfg component.Config = &Config{
					ControllerConfig: scraperhelper.ControllerConfig{
						CollectionInterval: 10 * time.Second,
						InitialDelay:       time.Second,
					},
					TopQueryCollection: TopQueryCollection{
						Enabled:             false,
						LookbackTime:        uint(2 * 10),
						MaxQuerySampleCount: 1000,
						TopQueryCount:       200,
					},
					QuerySample: QuerySample{
						Enabled:         false,
						MaxRowsPerQuery: 100,
					},
					MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
					LogsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
				}

				require.Equal(t, expectedCfg, factory.CreateDefaultConfig())
			},
		},
		{
			desc: "creates a new factory and CreateMetrics returns error with incorrect config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				_, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					nil,
					consumertest.NewNop(),
				)
				require.ErrorIs(t, err, errConfigNotSQLServer)
			},
		},
		{
			desc: "creates a new factory and CreateMetrics returns no error",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				r, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				scrapers := setupSQLServerScrapers(receivertest.NewNopSettings(metadata.Type), cfg.(*Config))
				require.Empty(t, scrapers)
				require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
				require.NoError(t, r.Shutdown(context.Background()))
			},
		},
		{
			desc: "[metrics] Test direct connection",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig().(*Config)
				cfg.Username = "sa"
				cfg.Password = "password"
				cfg.Server = "0.0.0.0"
				cfg.Port = 1433
				require.NoError(t, cfg.Validate())
				cfg.Metrics.SqlserverDatabaseLatency.Enabled = true

				require.True(t, cfg.isDirectDBConnectionEnabled)
				require.Equal(t, "server=0.0.0.0;user id=sa;password=password;port=1433", getDBConnectionString(cfg))

				params := receivertest.NewNopSettings(metadata.Type)
				scrapers, err := setupScrapers(params, cfg)
				require.NoError(t, err)
				require.NotEmpty(t, scrapers)

				sqlScrapers := setupSQLServerScrapers(params, cfg)
				require.NotEmpty(t, sqlScrapers)

				databaseIOScraperFound := false
				for _, scraper := range sqlScrapers {
					if scraper.sqlQuery == getSQLServerDatabaseIOQuery(cfg.InstanceName) {
						databaseIOScraperFound = true
						break
					}
				}

				require.True(t, databaseIOScraperFound)
				cfg.InstanceName = "instanceName"
				sqlScrapers = setupSQLServerScrapers(params, cfg)
				require.NotEmpty(t, sqlScrapers)

				databaseIOScraperFound = false
				for _, scraper := range sqlScrapers {
					if scraper.sqlQuery == getSQLServerDatabaseIOQuery(cfg.InstanceName) {
						databaseIOScraperFound = true
						break
					}
				}

				require.True(t, databaseIOScraperFound)

				r, err := factory.CreateMetrics(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
				require.NoError(t, r.Shutdown(context.Background()))
			},
		},
		// Test cases for logs
		{
			desc: "creates a new factory and CreateLogs returns error with incorrect config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				_, err := factory.CreateLogs(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					nil,
					consumertest.NewNop())
				require.ErrorIs(t, err, errConfigNotSQLServer)
			},
		},
		{
			desc: "creates a new factory and CreateLogs returns no error",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				r, err := factory.CreateLogs(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				scrapers := setupSQLServerLogsScrapers(receivertest.NewNopSettings(metadata.Type), cfg.(*Config))
				require.Empty(t, scrapers)
				require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
				require.NoError(t, r.Shutdown(context.Background()))
			},
		},
		{
			desc: "[logs] Test direct connection",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig().(*Config)
				cfg.Username = "sa"
				cfg.Password = "password"
				cfg.Server = "0.0.0.0"
				cfg.Port = 1433
				require.NoError(t, cfg.Validate())
				cfg.Metrics.SqlserverDatabaseLatency.Enabled = true

				require.True(t, cfg.isDirectDBConnectionEnabled)
				require.Equal(t, "server=0.0.0.0;user id=sa;password=password;port=1433", getDBConnectionString(cfg))

				params := receivertest.NewNopSettings(metadata.Type)
				scrapers, err := setupLogsScrapers(params, cfg)
				require.NoError(t, err)
				require.Empty(t, scrapers)

				sqlScrapers := setupSQLServerLogsScrapers(params, cfg)
				require.Empty(t, sqlScrapers)

				cfg.InstanceName = "instanceName"
				cfg.TopQueryCollection.Enabled = true
				scrapers, err = setupLogsScrapers(params, cfg)
				require.NoError(t, err)
				require.NotEmpty(t, scrapers)

				sqlScrapers = setupSQLServerLogsScrapers(params, cfg)
				require.NotEmpty(t, sqlScrapers)

				q := getSQLServerQueryTextAndPlanQuery()

				databaseTopQueryScraperFound := false
				for _, scraper := range sqlScrapers {
					if scraper.sqlQuery == q {
						databaseTopQueryScraperFound = true
						break
					}
				}

				require.True(t, databaseTopQueryScraperFound)

				r, err := factory.CreateLogs(
					context.Background(),
					receivertest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
				require.NoError(t, r.Shutdown(context.Background()))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestNewCache(t *testing.T) {
	var cache *lru.Cache[string, int64]
	// even when size is less than 0, cache should be created with size 1.
	// Also noticed that the cache returned would never be nil, only
	// cache.lru could be nil, which is invisible to us. So we can
	// test the cache.Values() method to check if the cache is created.
	cache = newCache(10)
	require.NotNil(t, cache.Values())
	cache = newCache(-1)
	require.NotNil(t, cache.Values())
	cache = newCache(0)
	require.NotNil(t, cache.Values())
}

func TestSetupQueries(t *testing.T) {
	var metadata map[string]any

	yamlFile, err := os.ReadFile("./metadata.yaml")
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(yamlFile, &metadata))
	require.NotNil(t, metadata["metrics"])

	metricsMetadata, ok := metadata["metrics"].(map[string]any)
	require.True(t, ok)
	require.Len(t, metricsMetadata, 48,
		"Every time metrics are added or removed, the function `setupQueries` must "+
			"be modified to properly account for the change. Please update `setupQueries` and then, "+
			"and only then, update the expected metric count here.")
}
