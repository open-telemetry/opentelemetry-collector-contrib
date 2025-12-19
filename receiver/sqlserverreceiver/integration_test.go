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

// allLogRecords flattens a plog.Logs into a single LogRecordSlice.
// It copies records into a new slice using AppendEmpty and CopyTo.
// Safe when there are zero Resource/Scope entries.
func allLogRecords(l plog.Logs) plog.LogRecordSlice {
	out := plog.NewLogRecordSlice()
	rls := l.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		sls := rls.At(i).ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			recs := sls.At(j).LogRecords()
			for k := 0; k < recs.Len(); k++ {
				rec := out.AppendEmpty()
				recs.At(k).CopyTo(rec)
			}
		}
	}
	return out
}

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
			// More forgiving settings to capture top queries reliably
			LookbackTime:        10000, // 10s lookback
			MaxQuerySampleCount: 200,
			TopQueryCount:       100,
			CollectionInterval:  100 * time.Millisecond, // avoid 1ms churn
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
				// Ensure the query has begun so sample can be captured
				assert.Eventually(t, func() bool {
					return queryCount.Load() > 0
				}, 10*time.Second, 100*time.Millisecond, "Query did not start in time")

				actualLog, err := scraper.ScrapeLogs(t.Context())
				assert.NoError(t, err)
				assert.NotNil(t, actualLog)

				records := allLogRecords(actualLog)
				// Expect exactly 2 records per original test
				assert.Equal(t, 2, records.Len(), "expected 2 query sample log records")

				foundNonMaster := false
				for i := 0; i < records.Len(); i++ {
					attrs := records.At(i).Attributes().AsRaw()
					if attrs["db.namespace"] == "master" {
						continue
					}
					foundNonMaster = true
					q, _ := attrs["db.query.text"].(string)
					// As the query is not standard, only the WAITFOR part may be returned
					assert.True(t, strings.HasPrefix(q, "WAITFOR"), "expected WAITFOR prefix, got: %q", q)
				}
				assert.True(t, foundNonMaster, "expected at least one non-master record")
				finished.Store(true)
			},
		},
		{
			name: "TopQuery",
			// Use a heavier, repeatable query to ensure it ranks in top queries
			clientQuery: `
SELECT TOP 100 *
FROM dbo.test_table
ORDER BY id
OPTION (RECOMPILE, MAXDOP 1)
`,
			configModifyFunc: func(cfg *Config) *Config {
				cfg.Events.DbServerTopQuery.Enabled = true
				// TopQueryCollection already set in basicConfig; keep as-is
				return cfg
			},
			validateFunc: func(t *testing.T, scraper *sqlServerScraperHelper, queryCount *atomic.Int32, finished *atomic.Bool) {
				// Ensure queries are running
				assert.Eventually(t, func() bool {
					return queryCount.Load() > 10
				}, 15*time.Second, 100*time.Millisecond, "Query did not start in time")

				// Prime the scraper once
				_, err := scraper.ScrapeLogs(t.Context())
				assert.NoError(t, err)

				// Keep generating workload for a bit to ensure it lands in the lookback window
				time.Sleep(2 * time.Second)

				var actualLog plog.Logs
				assert.EventuallyWithT(t, func(tt *assert.CollectT) {
					var e error
					actualLog, e = scraper.ScrapeLogs(t.Context())
					assert.NoError(tt, e)
					recs := allLogRecords(actualLog)
					assert.Positive(tt, recs.Len(), "no log records yet for top queries")

					found := false
					// Normalize and search for expected SQL text across common keys
					target := "SELECT TOP 100 * FROM dbo.test_table ORDER BY id"
					for i := 0; i < recs.Len(); i++ {
						attrs := recs.At(i).Attributes().AsRaw()

						// Optional: require correct event category/name if present
						if v, ok := attrs["event.name"].(string); ok && v != "" && v != "db.server.top_query" {
							continue
						}

						qs, _ := attrs["db.query.text"].(string)
						if qs == "" {
							qs, _ = attrs["db.statement"].(string)
						}
						if qs == "" {
							qs, _ = attrs["sql.query"].(string)
						}
						norm := strings.Join(strings.Fields(qs), " ")
						if strings.Contains(norm, target) {
							found = true
							break
						}
					}
					assert.True(tt, found, "expected top query to be present")
				}, 20*time.Second, 200*time.Millisecond)

				finished.Store(true)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Client connection simulating queries against SQL Server
			connStr := fmt.Sprintf("Server=localhost,%s;Database=mydb;User Id=myuser;Password=UserStrongPass1;", p.Port())
			db, err := sql.Open("sqlserver", connStr)
			assert.NoError(t, err)

			queryContext, cancel := context.WithCancel(t.Context())
			defer func() {
				_ = db.Close()
				cancel()
			}()

			finished := atomic.Bool{}
			queriesCount := atomic.Int32{}
			queriesCount.Store(0)
			finished.Store(false)

			// Generate workload in background
			go func(ctx context.Context) {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						queriesCount.Add(1)
						_, queryErr := db.Exec(tc.clientQuery)
						if !finished.Load() {
							// only assert while test is active to avoid masking shutdown errors
							assert.NoError(t, queryErr)
						}
						// small delay to keep throughput high but avoid overwhelming SQL Server
						time.Sleep(10 * time.Millisecond)
					}
				}
			}(queryContext)

			// Give container and DB some extra time to settle before starting scraper
			time.Sleep(10 * time.Second)

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
