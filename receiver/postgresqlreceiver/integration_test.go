// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package postgresqlreceiver

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"

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
