// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package sqlserverreceiver

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

func basicConfig(portNumber uint) *Config {
	return &Config{
		Server:   "localhost",
		Port:     portNumber,
		Username: "otelcollectoruser",
		Password: "otel-password123",
		QuerySample: QuerySample{
			MaxRowsPerQuery: 100,
		},
		TopQueryCollection: TopQueryCollection{
			LookbackTime:        1 * time.Second,
			MaxQuerySampleCount: 100,
			TopQueryCount:       100,
			CollectionInterval:  1 * time.Millisecond,
		},
		isDirectDBConnectionEnabled: true,
		LogsBuilderConfig: metadata.LogsBuilderConfig{
			Events: metadata.EventsConfig{
				DbServerQuerySample: metadata.EventConfig{
					Enabled: false,
				},
				DbServerTopQuery: metadata.EventConfig{
					Enabled: false,
				},
			},
		},
	}
}

func setupContainer() (testcontainers.Container, error) {
	return testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image: "mcr.microsoft.com/mssql/server:2022-latest",
				Env: map[string]string{
					"ACCEPT_EULA":       "Y",
					"MSSQL_SA_PASSWORD": "^otelcol1234",
					"MSSQL_PID":         "Developer",
				},
				Files: []testcontainers.ContainerFile{{
					HostFilePath:      filepath.Join("testdata", "integration", "01-init.sh"),
					ContainerFilePath: "/init/01-init.sh",
					FileMode:          0o777,
				}},
				Cmd:          []string{"/bin/bash", "-c", "/opt/mssql/bin/sqlservr & /init/01-init.sh && sleep infinity"},
				ExposedPorts: []string{"1433/tcp"},
				WaitingFor:   wait.NewLogStrategy("Initialization complete."),
			},
		},
	)
}

func TestEventsScraper(t *testing.T) {
	ci, initErr := setupContainer()

	assert.NoError(t, initErr)

	initErr = ci.Start(t.Context())
	assert.NoError(t, initErr)
	defer testcontainers.CleanupContainer(t, ci)
	p, initErr := ci.MappedPort(t.Context(), "1433")
	assert.NoError(t, initErr)

	cases := []struct {
		name             string
		clientQuery      string
		configModifyFunc func(cfg *Config) *Config
		validateFunc     func(t *testing.T, scraper *sqlServerScraperHelper, queryCount *atomic.Int32, finished *atomic.Bool)
	}{
		{
			name:        "QuerySample",
			clientQuery: "WAITFOR DELAY '00:01:00' SELECT * FROM dbo.test_table",
			configModifyFunc: func(cfg *Config) *Config {
				cfg.Events.DbServerQuerySample.Enabled = true
				return cfg
			},

			validateFunc: func(t *testing.T, scraper *sqlServerScraperHelper, queryCount *atomic.Int32, finished *atomic.Bool) {
				assert.Eventually(t, func() bool {
					return queryCount.Load() > 0
				}, 10*time.Second, 100*time.Millisecond, "Query did not start in time")

				assert.EventuallyWithT(t, func(tt *assert.CollectT) {
					actualLog, err := scraper.ScrapeLogs(t.Context())
					assert.NoError(tt, err)
					assert.NotNil(tt, actualLog)

					if actualLog.ResourceLogs().Len() == 0 {
						assert.Fail(tt, "No resource logs found")
						return
					}

					logRecords := actualLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
					assert.GreaterOrEqual(tt, logRecords.Len(), 1)

					found := false
					for i := 0; i < logRecords.Len(); i++ {
						attributes := logRecords.At(i).Attributes().AsRaw()
						if attributes["db.namespace"] == "master" {
							continue
						}
						found = true
						query := attributes["db.query.text"].(string)
						// as the query is not a standard query, only the `WAITFOR` part can be returned from db.
						assert.True(tt, strings.HasPrefix(query, "WAITFOR"), "Expected query to start with WAITFOR, got: %s", query)
					}

					assert.True(tt, found, "Expected query not found in logs")
				}, 10*time.Second, 100*time.Millisecond)

				finished.Store(true)
			},
		},
		{
			name:        "TopQuery",
			clientQuery: "SELECT * FROM dbo.test_table",
			configModifyFunc: func(cfg *Config) *Config {
				cfg.Events.DbServerTopQuery.Enabled = true
				return cfg
			},
			validateFunc: func(t *testing.T, scraper *sqlServerScraperHelper, queryCount *atomic.Int32, finished *atomic.Bool) {
				assert.Eventually(t, func() bool {
					return queryCount.Load() > 1
				}, 10*time.Second, 100*time.Millisecond, "Query did not start in time")
				_, err := scraper.ScrapeLogs(t.Context())
				currentQueriesCount := queryCount.Load()
				assert.NoError(t, err)
				assert.Eventually(t, func() bool {
					// wait for the query to be executed at least once.
					// otherwise, the scraper will ignore this query as during the
					// collection interval it will not be considered as a top query.
					return queryCount.Load() > currentQueriesCount+1
				}, 10*time.Second, 2*time.Second, "Query did not execute enough times")

				var actualLog plog.Logs
				var found bool
				assert.EventuallyWithT(t, func(tt *assert.CollectT) {
					actualLog, err = scraper.ScrapeLogs(t.Context())
					assert.NoError(tt, err)
					assert.NotNil(tt, actualLog)

					if actualLog.LogRecordCount() == 0 {
						assert.Fail(tt, "No log records found")
						return
					}

					if actualLog.ResourceLogs().Len() == 0 {
						assert.Fail(tt, "No resource logs found")
						return
					}

					found = false
					logRecords := actualLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
					for i := 0; i < logRecords.Len(); i++ {
						attributes := logRecords.At(i).Attributes().AsRaw()
						query := attributes["db.query.text"].(string)
						if query == "SELECT * FROM dbo.test_table" {
							found = true
							break
						}
					}

					assert.True(tt, found, "Expected query not found in logs")
				}, 10*time.Second, 100*time.Millisecond)

				finished.Store(true)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// this connection is trying to simulate a client that is running queries against the SQL Server
			connStr := fmt.Sprintf("Server=localhost,%s;Database=mydb;User Id=myuser;Password=UserStrongPass1;", p.Port())
			db, err := sql.Open("sqlserver", connStr)
			assert.NoError(t, err)

			queryContext, cancel := context.WithCancel(t.Context())
			defer func() {
				db.Close()
				cancel()
			}()
			finished := atomic.Bool{}
			queriesCount := atomic.Int32{}
			queriesCount.Store(0)
			finished.Store(false)

			go func(ctx context.Context) {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						queriesCount.Add(1)
						// Simulate a long-running query
						_, queryErr := db.Exec(tc.clientQuery)
						if !finished.Load() {
							// only check this condition if the test is not finished
							assert.NoError(t, queryErr)
						}
					}
				}
			}(queryContext)

			portNumber, err := strconv.Atoi(p.Port())
			assert.NoError(t, err)

			cfg := basicConfig(uint(portNumber))
			cfg = tc.configModifyFunc(cfg)
			settings := receiver.Settings{
				TelemetrySettings: component.TelemetrySettings{
					Logger: zap.Must(zap.NewProduction()),
				},
			}
			scrapers := setupSQLServerLogsScrapers(settings, cfg)
			assert.Len(t, scrapers, 1)
			scraper := scrapers[0]
			assert.NoError(t, scraper.Start(t.Context(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, scraper.Shutdown(t.Context()))
			}()

			tc.validateFunc(t, scraper, &queriesCount, &finished)
		})
	}
}
