// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

func TestScraperWithServiceInstanceID(t *testing.T) {
	hostname, _ := os.Hostname()

	testCases := []struct {
		name                string
		config              *Config
		expectedInstanceID  string
		mockRows            []sqlquery.StringMap
	}{
		{
			name: "explicit_server_and_port",
			config: &Config{
				Server:               "myserver.example.com",
				Port:                 5432,
				Username:             "sa",
				Password:             configopaque.String("pass"),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedInstanceID: "myserver.example.com:5432",
		},
		{
			name: "explicit_server_default_port",
			config: &Config{
				Server:               "myserver.example.com",
				Port:                 0,
				Username:             "sa",
				Password:             configopaque.String("pass"),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedInstanceID: "myserver.example.com:1433",
		},
		{
			name: "datasource_with_port",
			config: &Config{
				DataSource:           "server=dbserver.example.com,5000;user id=sa;password=pass",
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedInstanceID: "dbserver.example.com:5000",
		},
		{
			name: "localhost_replacement",
			config: &Config{
				Server:               "localhost",
				Port:                 1433,
				Username:             "sa",
				Password:             configopaque.String("pass"),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedInstanceID: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "no_server_uses_hostname",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedInstanceID: fmt.Sprintf("%s:1433", hostname),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Enable the metrics that trigger database IO query
			tc.config.Metrics.SqlserverDatabaseIo.Enabled = true
			tc.config.Metrics.SqlserverDatabaseLatency.Enabled = true
			tc.config.Metrics.SqlserverDatabaseOperations.Enabled = true
			tc.config.isDirectDBConnectionEnabled = true

			// Create mock client
			clientProviderFunc := getMockClientProviderFunc([][]sqlquery.StringMap{
				{
					{
						"computer_name":     "test-computer",
						"database_name":     "master",
						"sql_instance":      "MSSQLSERVER",
						"physical_filename": "/var/opt/mssql/data/master.mdf",
						"logical_filename":  "master",
						"file_type":         "DATA",
						"read_latency_ms":   "1000",
						"write_latency_ms":  "500",
						"reads":             "100",
						"writes":            "50",
						"read_bytes":        "1048576",
						"write_bytes":       "524288",
					},
				},
			})

			dbProviderFunc := getMockDbProviderFunc()

			cache := newCache(1)
			scraper := newSQLServerScraper(
				component.MustNewID("sqlserver"),
				getSQLServerDatabaseIOQuery(""),
				sqlquery.TelemetryConfig{},
				dbProviderFunc,
				clientProviderFunc,
				receivertest.NewNopSettings(metadata.Type),
				tc.config,
				cache,
			)

			// Check that service instance ID was computed correctly
			assert.Equal(t, tc.expectedInstanceID, scraper.serviceInstanceID)

			// Start the scraper
			err := scraper.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			// Scrape metrics
			metrics, err := scraper.ScrapeMetrics(context.Background())
			require.NoError(t, err)

			// Verify service.instance.id is present in resource attributes
			assert.Equal(t, 1, metrics.ResourceMetrics().Len())
			resourceMetrics := metrics.ResourceMetrics().At(0)
			attrs := resourceMetrics.Resource().Attributes()
			
			serviceInstanceID, ok := attrs.Get("service.instance.id")
			assert.True(t, ok, "service.instance.id attribute should be present")
			assert.Equal(t, tc.expectedInstanceID, serviceInstanceID.Str())

			// Shutdown the scraper
			err = scraper.Shutdown(context.Background())
			require.NoError(t, err)
		})
	}
}

func getMockDbProviderFunc() sqlquery.DbProviderFunc {
	return func() (*sql.DB, error) {
		// Return nil to avoid closing a real database connection
		return nil, nil
	}
}

func getMockClientProviderFunc(rowGroups [][]sqlquery.StringMap) sqlquery.ClientProviderFunc {
	currentGroup := 0
	return func(sqlquery.Db, string, *zap.Logger, sqlquery.TelemetryConfig) sqlquery.DbClient {
		return &mockDbClient{
			rowGroups:    rowGroups,
			currentGroup: &currentGroup,
		}
	}
}

type mockDbClient struct {
	rowGroups    [][]sqlquery.StringMap
	currentGroup *int
}

func (c *mockDbClient) QueryRows(ctx context.Context, args ...any) ([]sqlquery.StringMap, error) {
	if *c.currentGroup < len(c.rowGroups) {
		rows := c.rowGroups[*c.currentGroup]
		*c.currentGroup++
		return rows, nil
	}
	return []sqlquery.StringMap{}, nil
}