// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package sqlserverreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

func TestFactoryOtherOS(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "[metrics] Test direct connection with instance name",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig().(*Config)
				cfg.Username = "sa"
				cfg.Password = "password"
				cfg.Server = "0.0.0.0"
				cfg.Port = 1433
				cfg.InstanceName = "instanceName"
				cfg.Metrics.SqlserverDatabaseLatency.Enabled = true
				require.NoError(t, cfg.Validate())

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
