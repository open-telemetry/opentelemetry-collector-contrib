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
		name               string
		config             *Config
		expectedInstanceID string
		mockRows           []sqlquery.StringMap
	}{
		{
			name: "explicit server and port",
			config: &Config{
				Server:               "myserver.example.com",
				Port:                 5432,
				Username:             "sa",
				Password:             configopaque.String("pass"),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				LogsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
			},
			expectedInstanceID: "myserver.example.com:5432",
		},
		{
			name: "explicit server default port",
			config: &Config{
				Server:               "myserver.example.com",
				Port:                 0,
				Username:             "sa",
				Password:             configopaque.String("pass"),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				LogsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
			},
			expectedInstanceID: "myserver.example.com:1433",
		},
		{
			name: "datasource with port",
			config: &Config{
				DataSource:           "server=dbserver.example.com,5000;user id=sa;password=pass",
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				LogsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
			},
			expectedInstanceID: "dbserver.example.com:5000",
		},
		{
			name: "localhost replacement",
			config: &Config{
				Server:               "localhost",
				Port:                 1433,
				Username:             "sa",
				Password:             configopaque.String("pass"),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				LogsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
			},
			expectedInstanceID: fmt.Sprintf("%s:1433", hostname),
		},
		{
			name: "no server uses hostname",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				LogsBuilderConfig:    metadata.DefaultLogsBuilderConfig(),
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

			// Enable logs events
			tc.config.Events.DbServerQuerySample.Enabled = true

			dbProviderFunc := getMockDbProviderFunc()

			// Test metrics scraping
			t.Run("metrics", func(t *testing.T) {
				// Create mock client for metrics
				metricsClientProviderFunc := getMockClientProviderFunc([][]sqlquery.StringMap{
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

				scraper := newSQLServerScraper(
					component.MustNewID("sqlserver"),
					getSQLServerDatabaseIOQuery(""),
					sqlquery.TelemetryConfig{},
					dbProviderFunc,
					metricsClientProviderFunc,
					receivertest.NewNopSettings(metadata.Type),
					tc.config,
					newCache(1),
				)

				// Check that service instance ID was computed correctly
				assert.Equal(t, tc.expectedInstanceID, scraper.serviceInstanceID)

				// Start the scraper
				err := scraper.Start(t.Context(), componenttest.NewNopHost())
				require.NoError(t, err)

				// Scrape metrics
				metrics, err := scraper.ScrapeMetrics(t.Context())
				require.NoError(t, err)

				// Verify service.instance.id is present in resource attributes
				assert.Equal(t, 1, metrics.ResourceMetrics().Len())
				resourceMetrics := metrics.ResourceMetrics().At(0)
				attrs := resourceMetrics.Resource().Attributes()

				serviceInstanceID, ok := attrs.Get("service.instance.id")
				assert.True(t, ok, "service.instance.id attribute should be present in metrics")
				assert.Equal(t, tc.expectedInstanceID, serviceInstanceID.Str())

				// Shutdown the scraper
				err = scraper.Shutdown(t.Context())
				require.NoError(t, err)
			})

			// Test log scraping
			t.Run("logs", func(t *testing.T) {
				// Create mock client for logs with query sample data
				logsClientProviderFunc := getMockClientProviderFunc([][]sqlquery.StringMap{
					{
						{
							"computer_name":          "test-computer",
							"sql_instance":           "MSSQLSERVER",
							"request_id":             "1234",
							"session_id":             "56",
							"execution_count":        "10",
							"plan_generation_num":    "1",
							"creation_time":          "2023-01-01 10:00:00",
							"last_execution_time":    "2023-01-01 11:00:00",
							"total_worker_time":      "5000",
							"min_worker_time":        "100",
							"max_worker_time":        "1000",
							"total_physical_reads":   "1024",
							"min_physical_reads":     "10",
							"max_physical_reads":     "100",
							"total_logical_writes":   "512",
							"min_logical_writes":     "5",
							"max_logical_writes":     "50",
							"total_logical_reads":    "2048",
							"min_logical_reads":      "20",
							"max_logical_reads":      "200",
							"total_elapsed_time":     "10000",
							"min_elapsed_time":       "500",
							"max_elapsed_time":       "2000",
							"sql_text":               "SELECT * FROM users WHERE id = 1",
							"plan_handle":            "0x123456",
							"sql_handle":             "0x789ABC",
							"query_hash":             "0xDEF123",
							"query_plan_hash":        "0x456789",
							"statement_start_offset": "0",
							"statement_end_offset":   "-1",
							"database_name":          "testdb",
							// Additional required fields
							"client_port":                 "12345",
							"blocking_session_id":         "0",
							"cpu_time":                    "1000",
							"deadlock_priority":           "0",
							"estimated_completion_time":   "0",
							"lock_timeout":                "-1",
							"logical_reads":               "2048",
							"open_transaction_count":      "0",
							"percent_complete":            "100",
							"reads":                       "1024",
							"row_count":                   "10",
							"transaction_id":              "0",
							"transaction_isolation_level": "2",
							"wait_time":                   "0",
							"writes":                      "512",
						},
					},
				})

				scraper := newSQLServerScraper(
					component.MustNewID("sqlserverlogs"),
					getSQLServerQuerySamplesQuery(),
					sqlquery.TelemetryConfig{},
					dbProviderFunc,
					logsClientProviderFunc,
					receivertest.NewNopSettings(metadata.Type),
					tc.config,
					newCache(1),
				)

				// Check that service instance ID was computed correctly
				assert.Equal(t, tc.expectedInstanceID, scraper.serviceInstanceID)

				// Start the scraper
				err := scraper.Start(t.Context(), componenttest.NewNopHost())
				require.NoError(t, err)

				// Scrape logs
				logs, err := scraper.ScrapeLogs(t.Context())
				require.NoError(t, err)

				// Verify service.instance.id is present in resource attributes
				assert.Positive(t, logs.ResourceLogs().Len(), "Should have at least one resource log")
				resourceLogs := logs.ResourceLogs().At(0)
				logAttrs := resourceLogs.Resource().Attributes()

				logServiceInstanceID, ok := logAttrs.Get("service.instance.id")
				assert.True(t, ok, "service.instance.id attribute should be present in logs")
				assert.Equal(t, tc.expectedInstanceID, logServiceInstanceID.Str())

				// Shutdown the scraper
				err = scraper.Shutdown(t.Context())
				require.NoError(t, err)
			})
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
