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

	"github.com/stretchr/testify/require"
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
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, false)()
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlConnectionPoolFeatureGate, false)()
	t.Run("single_db", integrationTest("single_db", []string{"otel"}, pre17TestVersion))
	t.Run("multi_db", integrationTest("multi_db", []string{"otel", "otel2"}, pre17TestVersion))
	t.Run("all_db", integrationTest("all_db", []string{}, pre17TestVersion))

	t.Run("single_db_post17", integrationTest("single_db_post17", []string{"otel"}, post17TestVersion))
}

func TestIntegrationWithSeparateSchemaAttr(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, true)()
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlConnectionPoolFeatureGate, false)()
	t.Run("single_db_schemaattr", integrationTest("single_db_schemaattr", []string{"otel"}, pre17TestVersion))
	t.Run("multi_db_schemaattr", integrationTest("multi_db_schemaattr", []string{"otel", "otel2"}, pre17TestVersion))
	t.Run("all_db_schemaattr", integrationTest("all_db_schemaattr", []string{}, pre17TestVersion))
}

func TestIntegrationWithConnectionPool(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlSeparateSchemaAttrFeatureGate, false)()
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlConnectionPoolFeatureGate, true)()
	t.Run("single_db_connpool", integrationTest("single_db_connpool", []string{"otel"}, pre17TestVersion))
	t.Run("multi_db_connpool", integrationTest("multi_db_connpool", []string{"otel", "otel2"}, pre17TestVersion))
	t.Run("all_db_connpool", integrationTest("all_db_connpool", []string{}, pre17TestVersion))
}

func TestIntegrationSemconv(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlUseOTelSemconvFeatureGate, true)()
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlConnectionPoolFeatureGate, false)()
	t.Run("multi_db", integrationTestSemconv("multi_db_semconv", []string{"otel", "otel2"}, pre17TestVersion))
}

func integrationTest(
	name string,
	databases []string,
	pgVersion string,
	additionalIgnoredResourceAttributeValues ...string,
) func(*testing.T) {
	compareOptions := []pmetrictest.CompareMetricsOption{
		pmetrictest.IgnoreResourceAttributeValue("service.instance.id"),
	}
	for _, attribute := range additionalIgnoredResourceAttributeValues {
		compareOptions = append(compareOptions, pmetrictest.IgnoreResourceAttributeValue(attribute))
	}
	compareOptions = append(compareOptions,
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
	)

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
				rCfg.ControllerConfig.CollectionInterval = time.Second
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
		scraperinttest.WithCompareOptions(compareOptions...),
	).Run
}

func integrationTestSemconv(name string, databases []string, pgVersion string) func(*testing.T) {
	return integrationTest(name, databases, pgVersion, "server.address", "server.port")
}

func TestGetDatabaseTableMetricsIgnoresAccessExclusiveLocks(t *testing.T) {
	ci, err := testcontainers.GenericContainer(
		t.Context(),
		testcontainers.GenericContainerRequest{
			ProviderType: testcontainers.ProviderPodman,
			ContainerRequest: testcontainers.ContainerRequest{
				Image: fmt.Sprintf("postgres:%s", pre17TestVersion),
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
			},
		})
	require.NoError(t, err)

	err = ci.Start(t.Context())
	require.NoError(t, err)
	defer testcontainers.CleanupContainer(t, ci)

	p, err := ci.MappedPort(t.Context(), postgresqlPort)
	require.NoError(t, err)

	lockDB, err := sql.Open("postgres", fmt.Sprintf("postgres://root:otel@localhost:%s/otel?sslmode=disable", p.Port()))
	require.NoError(t, err)
	defer lockDB.Close()

	tx, err := lockDB.Begin()
	require.NoError(t, err)
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.Exec("LOCK TABLE table1 IN ACCESS EXCLUSIVE MODE")
	require.NoError(t, err)

	clientDB, err := getDB(postgreSQLConfig{
		username: "otelu",
		password: "otelp",
		address: confignet.AddrConfig{
			Endpoint: net.JoinHostPort("localhost", p.Port()),
		},
		tls: configtls.ClientConfig{
			Insecure: true,
		},
	}, "otel")
	require.NoError(t, err)

	client := postgreSQLClient{client: clientDB, closeFn: clientDB.Close}
	defer func() {
		require.NoError(t, client.Close())
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	tableMetrics, err := client.getDatabaseTableMetrics(ctx, "otel")
	require.NoError(t, err)
	require.NotContains(t, tableMetrics, tableKey("otel", "public", "table1"))
	require.Contains(t, tableMetrics, tableKey("otel", "public", "table2"))
}

func TestGetIndexStatsIgnoresAccessExclusiveLocks(t *testing.T) {
	ci, err := testcontainers.GenericContainer(
		t.Context(),
		testcontainers.GenericContainerRequest{
			ProviderType: testcontainers.ProviderPodman,
			ContainerRequest: testcontainers.ContainerRequest{
				Image: fmt.Sprintf("postgres:%s", pre17TestVersion),
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
			},
		})
	require.NoError(t, err)

	err = ci.Start(t.Context())
	require.NoError(t, err)
	defer testcontainers.CleanupContainer(t, ci)

	p, err := ci.MappedPort(t.Context(), postgresqlPort)
	require.NoError(t, err)

	lockDB, err := sql.Open("postgres", fmt.Sprintf("postgres://root:otel@localhost:%s/otel?sslmode=disable", p.Port()))
	require.NoError(t, err)
	defer lockDB.Close()

	tx, err := lockDB.Begin()
	require.NoError(t, err)
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.Exec("REINDEX INDEX table1_pkey")
	require.NoError(t, err)

	clientDB, err := getDB(postgreSQLConfig{
		username: "otelu",
		password: "otelp",
		address: confignet.AddrConfig{
			Endpoint: net.JoinHostPort("localhost", p.Port()),
		},
		tls: configtls.ClientConfig{
			Insecure: true,
		},
	}, "otel")
	require.NoError(t, err)

	client := postgreSQLClient{client: clientDB, closeFn: clientDB.Close}
	defer func() {
		require.NoError(t, client.Close())
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	indexStats, err := client.getIndexStats(ctx, "otel")
	require.NoError(t, err)
	require.NotContains(t, indexStats, indexKey("otel", "public", "table1", "table1_pkey"))
	require.Contains(t, indexStats, indexKey("otel", "public", "table2", "table2_pkey"))
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
		LogsBuilderConfig: func() metadata.LogsBuilderConfig {
			cfg := metadata.DefaultLogsBuilderConfig()
			cfg.Events.DbServerQuerySample.Enabled = true
			cfg.Events.DbServerTopQuery.Enabled = true
			return cfg
		}(),
	}
	clientFactory := newDefaultClientFactory(&cfg)

	ns, err := newPostgreSQLScraper(receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.Must(zap.NewProduction()),
		},
	}, &cfg, clientFactory, newCache(1), newTTLCache[string](1000, time.Second))
	assert.NoError(t, err)
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

	firstTimeTopQueryPLogs, err := ns.scrapeTopQuery(t.Context(), 30, 30, 30, time.Minute)
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

	collectionInterval := time.Minute
	ns.lastExecutionTimestamp = ns.lastExecutionTimestamp.Add(-collectionInterval)
	secondTimeTopQueryPLogs, err := ns.scrapeTopQuery(t.Context(), 30, 30, 30, collectionInterval)
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

// pgvector image published on Docker Hub (same registry as the postgres image used
// by the other integration tests). Pinned for reproducibility.
const pgvectorTestImage = "pgvector/pgvector:0.8.0-pg17"

// startPostgresContainerForVector starts a PostgreSQL container with
// pg_stat_statements preloaded and the given init script mounted, then returns a
// database handle connected to the seeded database along with a cleanup func.
func startPostgresContainerForVector(t *testing.T, image, initScript string) (*sql.DB, func()) {
	t.Helper()
	ctx := t.Context()
	ci, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: image,
			Env: map[string]string{
				"POSTGRES_USER":     "root",
				"POSTGRES_PASSWORD": "otel",
				"POSTGRES_DB":       "otel",
			},
			Files: []testcontainers.ContainerFile{{
				HostFilePath:      filepath.Join("testdata", "integration", initScript),
				ContainerFilePath: "/docker-entrypoint-initdb.d/" + initScript,
				FileMode:          0o644,
			}},
			Cmd:          []string{"-c", "shared_preload_libraries=pg_stat_statements"},
			ExposedPorts: []string{postgresqlPort},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	host, err := ci.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	port, err := ci.MappedPort(ctx, postgresqlPort)
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}
	connStr := fmt.Sprintf("postgres://root:otel@%s/otel?sslmode=disable", net.JoinHostPort(host, port.Port()))
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("failed to ping db: %v", err)
	}
	return db, func() {
		db.Close()
		testcontainers.CleanupContainer(t, ci)
	}
}

// TestVectorSearchClassificationIntegration validates the SQL classification logic
// against a real pgvector-enabled PostgreSQL: every distance function is detected
// (no false negatives), while column names that merely contain distance-function
// substrings and pg_trgm word-similarity operators are excluded (no false positives).
func TestVectorSearchClassificationIntegration(t *testing.T) {
	db, cleanup := startPostgresContainerForVector(t, pgvectorTestImage, "03-init-vector.sql")
	defer cleanup()
	ctx := t.Context()

	// Legitimate pgvector searches: operator and function forms for all six
	// distance functions. These MUST be classified.
	legit := []string{
		"SELECT id FROM items ORDER BY embedding <=> '[0.1,0.2,0.3]' LIMIT 5",
		"SELECT id FROM items ORDER BY embedding <-> '[0.1,0.2,0.3]' LIMIT 5",
		"SELECT id FROM items ORDER BY embedding <#> '[0.1,0.2,0.3]' LIMIT 5",
		"SELECT id FROM items ORDER BY embedding <+> '[0.1,0.2,0.3]' LIMIT 5",
		"SELECT id FROM items ORDER BY cosine_distance(embedding, '[0.1,0.2,0.3]') LIMIT 5",
		"SELECT id FROM items ORDER BY l2_distance(embedding, '[0.1,0.2,0.3]') LIMIT 5",
		"SELECT id FROM bitems ORDER BY b <~> B'10101010' LIMIT 5",
		"SELECT id FROM bitems ORDER BY b <%> B'10101010' LIMIT 5",
	}
	// Traps that MUST NOT be classified as vector searches:
	//   - column names containing distance-function substrings (word-boundary guard)
	//   - pg_trgm word-similarity operators <<-> and <->> (refined <-> guard)
	traps := []string{
		"SELECT cosine_distance_threshold, inner_product_score, my_l2_distance_col FROM fp_cols WHERE id > 0",
		"SELECT id FROM docs ORDER BY body <<-> 'abc' LIMIT 5",
		"SELECT id FROM docs ORDER BY body <->> 'abc' LIMIT 5",
	}
	for _, q := range append(append([]string{}, legit...), traps...) {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("failed to execute workload query %q: %v", q, err)
		}
	}

	stats, err := (&postgreSQLClient{client: db}).getVectorSearchStats(ctx)
	if err != nil {
		t.Fatalf("getVectorSearchStats failed: %v", err)
	}

	got := make(map[string]int64, len(stats))
	for _, s := range stats {
		got[s.distanceFunction] = s.calls
	}

	// No false negatives: every distance function is detected with a positive count.
	for _, fn := range []string{"cosine", "l2", "inner_product", "l1", "hamming", "jaccard"} {
		assert.Contains(t, got, fn, "expected distance function %q to be classified", fn)
		assert.True(t, got[fn] > 0, "expected positive call count for distance function %q", fn)
	}
	// No false positives: only the six expected classifications are present. Any leaked
	// trap statement would add an extra row (or an empty-string classification).
	assert.Len(t, stats, 6)
	assert.NotContains(t, got, "", "no statement should produce an unclassified (NULL) distance function")
}

// TestVectorSearchExtensionGateIntegration validates that no vector-search metrics
// are produced when pgvector is not installed, even though pg_trgm (which overloads
// the <-> operator) is present and its distance operators appear in pg_stat_statements.
func TestVectorSearchExtensionGateIntegration(t *testing.T) {
	db, cleanup := startPostgresContainerForVector(t, fmt.Sprintf("postgres:%s", post17TestVersion), "04-init-trgm.sql")
	defer cleanup()
	ctx := t.Context()

	for _, q := range []string{
		"SELECT id FROM docs ORDER BY body <-> 'abc' LIMIT 5",
		"SELECT id FROM docs ORDER BY body <<-> 'abc' LIMIT 5",
	} {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("failed to execute workload query %q: %v", q, err)
		}
	}

	stats, err := (&postgreSQLClient{client: db}).getVectorSearchStats(ctx)
	if err != nil {
		t.Fatalf("getVectorSearchStats failed: %v", err)
	}
	assert.Empty(t, stats, "extension gate should suppress vector metrics when pgvector is not installed")
}

// TestVectorInsertStatsIntegration validates that inserts into pgvector tables are
// counted (no false negatives) while inserts into non-vector tables (including a
// table whose name is a prefix of a vector table) and non-insert statements over
// vector tables are excluded (no false positives).
func TestVectorInsertStatsIntegration(t *testing.T) {
	db, cleanup := startPostgresContainerForVector(t, pgvectorTestImage, "03-init-vector.sql")
	defer cleanup()
	ctx := t.Context()

	// Reset so the init-time seed inserts do not affect the deterministic counts below.
	if _, err := db.ExecContext(ctx, "SELECT pg_stat_statements_reset()"); err != nil {
		t.Fatalf("failed to reset pg_stat_statements: %v", err)
	}

	// Inserts into the vector table "items". SUM(rows) must be 6: three single-row
	// inserts plus one three-row insert.
	vectorInserts := []string{
		"INSERT INTO items (embedding) VALUES ('[0.1,0.2,0.3]')",
		"INSERT INTO items (embedding) VALUES ('[0.2,0.3,0.4]')",
		"INSERT INTO items (embedding) VALUES ('[0.9,0.8,0.7]')",
		"INSERT INTO items (embedding) VALUES ('[0.1,0.1,0.1]'), ('[0.2,0.2,0.2]'), ('[0.3,0.3,0.3]')",
	}
	// Statements that MUST NOT be counted:
	//   - inserts into non-vector tables (docs, fp_cols)
	//   - insert into items_archive (name is a prefix of the vector table "items")
	//   - a SELECT and an UPDATE over the vector table (not inserts)
	nonInserts := []string{
		"INSERT INTO docs (body) VALUES ('hello')",
		"INSERT INTO fp_cols (id) VALUES (99)",
		"INSERT INTO items_archive (note) VALUES ('archived')",
		"SELECT id FROM items ORDER BY embedding <-> '[0.1,0.2,0.3]' LIMIT 1",
		"UPDATE items SET embedding = '[0.5,0.5,0.5]' WHERE id = 1",
	}
	for _, q := range append(append([]string{}, vectorInserts...), nonInserts...) {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("failed to execute workload query %q: %v", q, err)
		}
	}

	stats, err := (&postgreSQLClient{client: db}).getVectorInsertStats(ctx)
	if err != nil {
		t.Fatalf("getVectorInsertStats failed: %v", err)
	}

	if len(stats) != 1 {
		t.Fatalf("expected exactly one aggregated insert-stats row, got %d", len(stats))
	}
	// No false negatives and no false positives: exactly the six rows inserted into the
	// vector table are counted.
	assert.Equal(t, int64(6), stats[0].rows)
	assert.True(t, stats[0].totalExecTime > 0, "expected positive cumulative insert duration")
}
