// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package postgresqlreceiver

import (
	"context"
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
					HostFilePath:      filepath.Join("testdata", "integration", "init.sql"),
					ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
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
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreSubsequentDataPoints("postgresql.backends"),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run
}

func TestScrapeLogsFromContainer(t *testing.T) {
	ci, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ProviderType: testcontainers.ProviderPodman,
			ContainerRequest: testcontainers.ContainerRequest{
				Image: fmt.Sprintf("postgres:%s", post17TestVersion),
				Env: map[string]string{
					"POSTGRES_USER":     "root",
					"POSTGRES_PASSWORD": "otel",
					"POSTGRES_DB":       "otel",
				},
				Files: []testcontainers.ContainerFile{{
					HostFilePath:      filepath.Join("testdata", "integration", "init.sql"),
					ContainerFilePath: "/docker-entrypoint-initdb.d/init.sql",
					FileMode:          700,
				}},
				ExposedPorts: []string{postgresqlPort},
				WaitingFor: wait.ForListeningPort(postgresqlPort).
					WithStartupTimeout(2 * time.Minute),
			},
		})
	assert.NoError(t, err)

	err = ci.Start(context.Background())
	assert.NoError(t, err)
	defer testcontainers.CleanupContainer(t, ci)
	p, err := ci.MappedPort(context.Background(), postgresqlPort)
	assert.NoError(t, err)
	connStr := fmt.Sprintf("postgres://root:otel@localhost:%s/otel2?sslmode=disable", p.Port())
	db, err := sql.Open("postgres", connStr)
	assert.NoError(t, err)

	_, err = db.Query("Select * from test2 where id = 67")
	assert.NoError(t, err)
	db.Close()

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
		QuerySampleCollection: QuerySampleCollection{
			Enabled: true,
		},
	}
	clientFactory := newDefaultClientFactory(&cfg)

	ns := newPostgreSQLScraper(receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.Must(zap.NewProduction()),
		},
	}, &cfg, clientFactory)
	plogs, err := ns.scrapeQuerySamples(context.Background(), 30)
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
}
