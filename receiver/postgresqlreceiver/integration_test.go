// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package postgresqlreceiver

import (
	"database/sql"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tj/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

const postgresqlPort = "5432"

const (
	pre17TestVersion  = "13.18"
	post17TestVersion = "17.2"
)

func TestIntegration(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, separateSchemaAttrGate, false)()
	defer testutil.SetFeatureGateForTest(t, connectionPoolGate, false)()
	t.Run("single_db", integrationTest("single_db", []string{"otel"}, pre17TestVersion))
	t.Run("multi_db", integrationTest("multi_db", []string{"otel", "otel2"}, pre17TestVersion))
	t.Run("all_db", integrationTest("all_db", []string{}, pre17TestVersion))

	t.Run("single_db_post17", integrationTest("single_db_post17", []string{"otel"}, post17TestVersion))
}

func TestIntegrationWithSeparateSchemaAttr(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, separateSchemaAttrGate, true)()
	defer testutil.SetFeatureGateForTest(t, connectionPoolGate, false)()
	t.Run("single_db_schemaattr", integrationTest("single_db_schemaattr", []string{"otel"}, pre17TestVersion))
	t.Run("multi_db_schemaattr", integrationTest("multi_db_schemaattr", []string{"otel", "otel2"}, pre17TestVersion))
	t.Run("all_db_schemaattr", integrationTest("all_db_schemaattr", []string{}, pre17TestVersion))
}

func TestIntegrationWithConnectionPool(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, separateSchemaAttrGate, false)()
	defer testutil.SetFeatureGateForTest(t, connectionPoolGate, true)()
	t.Run("single_db_connpool", integrationTest("single_db_connpool", []string{"otel"}, pre17TestVersion))
	t.Run("multi_db_connpool", integrationTest("multi_db_connpool", []string{"otel", "otel2"}, pre17TestVersion))
	t.Run("all_db_connpool", integrationTest("all_db_connpool", []string{}, pre17TestVersion))
}

func integrationTest(name string, databases []string, pgVersion string) func(*testing.T) {
	expectedFile := filepath.Join("testdata", "integration", "expected_"+name+".yaml")
	return scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithContainerRequest(
			testcontainers.ContainerRequest{
				Image: fmt.Sprintf("postgres:%s", pgVersion),
				Env: map[string]string{
					"POSTGRES_USER":     "root",
					"POSTGRES_PASSWORD": "otel",
					"POSTGRES_DB":       "otel",
				},
				Files: []testcontainers.ContainerFile{{
					HostFilePath:      filepath.Join("testdata", "integration", "01-init.sql"),
					ContainerFilePath: "/docker-entrypoint-initdb.d/01-init.sql",
					FileMode:          700,
				}},
				ExposedPorts: []string{postgresqlPort},
				WaitingFor: wait.ForListeningPort(postgresqlPort).
					WithStartupTimeout(2 * time.Minute),
			}),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = time.Second
				rCfg.Endpoint = net.JoinHostPort(ci.Host(t), ci.MappedPort(t, postgresqlPort))
				rCfg.Databases = databases
				rCfg.Username = "otelu"
				rCfg.Password = "otelp"
				rCfg.Insecure = true
				rCfg.Metrics.PostgresqlWalDelay.Enabled = true
				rCfg.Metrics.PostgresqlDeadlocks.Enabled = true
				rCfg.Metrics.PostgresqlTempIo.Enabled = true
				rCfg.Metrics.PostgresqlTempFiles.Enabled = true
				rCfg.Metrics.PostgresqlTupUpdated.Enabled = true
				rCfg.Metrics.PostgresqlTupReturned.Enabled = true
				rCfg.Metrics.PostgresqlTupFetched.Enabled = true
				rCfg.Metrics.PostgresqlTupInserted.Enabled = true
				rCfg.Metrics.PostgresqlTupDeleted.Enabled = true
				rCfg.Metrics.PostgresqlBlksHit.Enabled = true
				rCfg.Metrics.PostgresqlBlksRead.Enabled = true
				rCfg.Metrics.PostgresqlSequentialScans.Enabled = true
				rCfg.Metrics.PostgresqlDatabaseLocks.Enabled = true
			}),
		scraperinttest.WithExpectedFile(expectedFile),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricValues(
				"postgresql.backends",
				"postgresql.bgwriter.buffers.allocated",
				"postgresql.bgwriter.buffers.writes",
				"postgresql.bgwriter.checkpoint.count",
				"postgresql.bgwriter.duration",
				"postgresql.bgwriter.maxwritten",
				"postgresql.blks_hit",
				"postgresql.blks_read",
				"postgresql.blocks_read",
				"postgresql.commits",
				"postgresql.connection.max",
				"postgresql.database.count",
				"postgresql.database.locks",
				"postgresql.db_size",
				"postgresql.deadlocks",
				"postgresql.index.scans",
				"postgresql.index.size",
				"postgresql.operations",
				"postgresql.replication.data_delay",
				"postgresql.rollbacks",
				"postgresql.rows",
				"postgresql.sequential_scans",
				"postgresql.table.count",
				"postgresql.table.size",
				"postgresql.table.vacuum.count",
				"postgresql.tup_deleted",
				"postgresql.tup_fetched",
				"postgresql.tup_inserted",
				"postgresql.tup_returned",
				"postgresql.tup_updated",
				"postgresql.wal.age",
				"postgresql.wal.delay",
				"postgresql.wal.lag",
			),
			pmetrictest.IgnoreSubsequentDataPoints("postgresql.backends"),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run
}

func TestScrapeLogsFromContainer(t *testing.T) {
	ci, err := testcontainers.GenericContainer(
		t.Context(),
		testcontainers.GenericContainerRequest{
			ProviderType: testcontainers.ProviderPodman,
			ContainerRequest: testcontainers.ContainerRequest{
				Image: fmt.Sprintf("postgres:%s", post17TestVersion),
				Env: map[string]string{
					"POSTGRES_USER":     "root",
					"POSTGRES_PASSWORD": "otel",
					"POSTGRES_DB":       "otel",
				},
				Files: []testcontainers.ContainerFile{
					{
						HostFilePath:      filepath.Join("testdata", "integration", "01-init.sql"),
						ContainerFilePath: "/docker-entrypoint-initdb.d/01-init.sql",
						FileMode:          700,
					},
					{
						HostFilePath:      filepath.Join("testdata", "integration", "02-create-extension.sh"),
						ContainerFilePath: "/docker-entrypoint-initdb.d/02-create-extension.sh",
						FileMode:          700,
					},
				},
				ExposedPorts: []string{postgresqlPort},
				Cmd: []string{
					"-c",
					"shared_preload_libraries=pg_stat_statements",
				},
				WaitingFor: wait.ForLog(".*port 5432").
					AsRegexp().
					WithOccurrence(1),
			},
		})
	assert.NoError(t, err)

	err = ci.Start(t.Context())
	assert.NoError(t, err)
	defer testcontainers.CleanupContainer(t, ci)
	p, err := ci.MappedPort(t.Context(), postgresqlPort)
	assert.NoError(t, err)
	connStr := fmt.Sprintf("postgres://root:otel@localhost:%s/otel2?sslmode=disable", p.Port())
	db, err := sql.Open("postgres", connStr)
	assert.NoError(t, err)

	_, err = db.Exec("Select * from test2 where id = 67")
	assert.NoError(t, err)
	defer db.Close()

	cfg := Config{
		Databases: []string{"postgres"},
		Username:  "otelu",
		Password:  "otelp",
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Second,
		},
		ClientConfig: configtls.ClientConfig{
			Insecure: true,
		},
		AddrConfig: confignet.AddrConfig{
			Endpoint: net.JoinHostPort("localhost", p.Port()),
		},
		LogsBuilderConfig: metadata.DefaultLogsBuilderConfig(),
	}
	clientFactory := newDefaultClientFactory(&cfg)

	ns := newPostgreSQLScraper(receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.Must(zap.NewProduction()),
		},
	}, &cfg, clientFactory, newCache(1), newTTLCache[string](1000, time.Second))
	plogs, err := ns.scrapeQuerySamples(t.Context(), 30)
	assert.NoError(t, err)
	logRecords := plogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	found := false
	for _, record := range logRecords.All() {
		attributes := record.Attributes().AsRaw()
		queryAttribute, ok := attributes["db.query.text"]
		query := strings.ToLower(queryAttribute.(string))
		assert.True(t, ok)
		if !strings.HasPrefix(query, "select * from test2") {
			continue
		}
		assert.Equal(t, "select * from test2 where id = ?", query)
		databaseAttribute, ok := attributes["db.namespace"]
		assert.True(t, ok)
		assert.Equal(t, "otel2", databaseAttribute.(string))
		found = true
	}
	assert.True(t, found, "Expected to find a log record with the query text")
	assert.True(t, ns.newestQueryTimestamp > 0)

	firstTimeTopQueryPLogs, err := ns.scrapeTopQuery(t.Context(), 30, 30, 30)
	assert.NoError(t, err)
	logRecords = firstTimeTopQueryPLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	found = false
	for _, record := range logRecords.All() {
		attributes := record.Attributes().AsRaw()
		queryAttribute, ok := attributes["db.query.text"]
		query := strings.ToLower(queryAttribute.(string))
		assert.True(t, ok)
		if !strings.HasPrefix(query, "select * from test2 where") {
			continue
		}
		assert.Equal(t, "select * from test2 where id = ?", query)
		databaseAttribute, ok := attributes["db.namespace"]
		assert.True(t, ok)
		assert.Equal(t, "otel2", databaseAttribute.(string))
		calls, ok := attributes["postgresql.calls"]
		assert.True(t, ok)
		assert.Equal(t, int64(1), calls.(int64))
		assert.NotEmpty(t, attributes["postgresql.query_plan"])
		found = true
	}
	assert.True(t, found, "Expected to find a log record with the query text from the first time top query")

	_, err = db.Exec("Select * from test2 where id = 67")
	assert.NoError(t, err)

	secondTimeTopQueryPLogs, err := ns.scrapeTopQuery(t.Context(), 30, 30, 30)
	assert.NoError(t, err)
	logRecords = secondTimeTopQueryPLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	found = false
	for _, record := range logRecords.All() {
		attributes := record.Attributes().AsRaw()
		queryAttribute, ok := attributes["db.query.text"]
		query := strings.ToLower(queryAttribute.(string))
		assert.True(t, ok)
		if !strings.HasPrefix(query, "select * from test2 where") {
			continue
		}
		assert.Equal(t, "select * from test2 where id = ?", query)
		databaseAttribute, ok := attributes["db.namespace"]
		assert.True(t, ok)
		assert.Equal(t, "otel2", databaseAttribute.(string))
		calls, ok := attributes["postgresql.calls"]
		assert.True(t, ok)
		assert.Equal(t, int64(2), calls.(int64))
		found = true
	}
	assert.True(t, found, "Expected to find a log record with the query text from the first time top query")
}
