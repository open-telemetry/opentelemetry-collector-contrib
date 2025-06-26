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
			LookbackTime:        1000,
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
	ci, err := setupContainer()

	assert.NoError(t, err)

	err = ci.Start(context.Background())
	assert.NoError(t, err)
	defer testcontainers.CleanupContainer(t, ci)
	p, err := ci.MappedPort(context.Background(), "1433")
	assert.NoError(t, err)

	connStr := fmt.Sprintf("Server=localhost,%s;Database=mydb;User Id=myuser;Password=UserStrongPass1;", p.Port())
	db, err := sql.Open("sqlserver", connStr)
	assert.NoError(t, err)

	queryContext, cancel := context.WithCancel(context.Background())
	finished := atomic.Bool{}
	finished.Store(false)

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Simulate a long-running query
				_, queryErr := db.Query("WAITFOR DELAY '00:00:20' SELECT * FROM dbo.test_table")
				if !finished.Load() {
					// only check this condition if the test is not finished
					assert.NoError(t, queryErr)
				}
			}
		}
	}(queryContext)

	defer func() {
		db.Close()
		cancel()
	}()
	portNumber, err := strconv.Atoi(p.Port())
	assert.NoError(t, err)

	cfg := basicConfig(uint(portNumber))
	cfg.Events.DbServerQuerySample.Enabled = true
	settings := receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.Must(zap.NewProduction()),
		},
	}
	scrapers := setupSQLServerLogsScrapers(settings, cfg)
	assert.Len(t, scrapers, 1)
	scraper := scrapers[0]
	defer scraper.Shutdown(context.Background()) //nolint:errcheck

	assert.NoError(t, scraper.Start(context.Background(), componenttest.NewNopHost()))
	time.Sleep(5 * time.Second) // Wait for the query to execute

	actualLog, err := scraper.ScrapeLogs(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, actualLog)
	logRecords := actualLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()

	assert.Equal(t, 2, logRecords.Len())
	found := false
	for i := 0; i < logRecords.Len(); i++ {
		attributes := logRecords.At(i).Attributes().AsRaw()
		if attributes["db.namespace"] == "master" {
			continue
		}
		found = true
		query := attributes["db.query.text"].(string)
		// as the query is not a standard query, only the `WITFOR` part can be returned from db.
		assert.True(t, strings.HasPrefix(query, "WAITFOR"))
	}
	assert.True(t, found)
	finished.Store(true)
}

func TestTopQueryScraper(t *testing.T) {
	ci, err := setupContainer()

	assert.NoError(t, err)

	err = ci.Start(context.Background())
	assert.NoError(t, err)
	defer testcontainers.CleanupContainer(t, ci)
	p, err := ci.MappedPort(context.Background(), "1433")
	assert.NoError(t, err)

	connStr := fmt.Sprintf("Server=localhost,%s;Database=mydb;User Id=myuser;Password=UserStrongPass1;", p.Port())
	db, err := sql.Open("sqlserver", connStr)
	assert.NoError(t, err)

	queryContext, cancel := context.WithCancel(context.Background())
	finished := atomic.Bool{}
	finished.Store(false)

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, queryErr := db.Query("SELECT * FROM dbo.test_table")
				if !finished.Load() {
					assert.NoError(t, queryErr)
				}
			}
		}
	}(queryContext)

	defer func() {
		db.Close()
		cancel()
	}()
	portNumber, err := strconv.Atoi(p.Port())
	assert.NoError(t, err)
	cfg := basicConfig(uint(portNumber))
	cfg.Events.DbServerTopQuery.Enabled = true
	settings := receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.Must(zap.NewProduction()),
		},
	}
	scrapers := setupSQLServerLogsScrapers(settings, cfg)
	assert.Len(t, scrapers, 1)
	scraper := scrapers[0]
	defer scraper.Shutdown(context.Background()) //nolint:errcheck
	assert.NoError(t, scraper.Start(context.Background(), componenttest.NewNopHost()))
	time.Sleep(5 * time.Second)
	_, err = scraper.ScrapeLogs(context.Background())
	assert.NoError(t, err)
	time.Sleep(5 * time.Second)
	actualLog, err := scraper.ScrapeLogs(context.Background())
	assert.NotNil(t, actualLog)
	assert.NoError(t, err)
	found := false
	logRecords := actualLog.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	for i := 0; i < logRecords.Len(); i++ {
		attributes := logRecords.At(i).Attributes().AsRaw()

		query := attributes["db.query.text"].(string)
		if query == "SELECT * FROM dbo.test_table" {
			found = true
		}
	}
	assert.True(t, found)
	finished.Store(true)
}
