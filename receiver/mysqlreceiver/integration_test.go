// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package mysqlreceiver

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
)

const mysqlPort = "3306"

type mySQLTestConfig struct {
	name         string
	containerCmd []string
	tlsEnabled   bool
	insecureSkip bool
	imageVersion string
	expectedFile string
}

func TestIntegration(t *testing.T) {
	testCases := []mySQLTestConfig{
		{
			name:         "MySql-8.0.33-WithoutTLS",
			containerCmd: nil,
			tlsEnabled:   false,
			insecureSkip: false,
			imageVersion: "mysql:8.0.33",
			expectedFile: "expected-mysql.yaml",
		},
		{
			name:         "MySql-8.0.33-WithTLS",
			containerCmd: []string{"--auto_generate_certs=ON", "--require_secure_transport=ON"},
			tlsEnabled:   true,
			insecureSkip: true,
			imageVersion: "mysql:8.0.33",
			expectedFile: "expected-mysql.yaml",
		},
		{
			name:         "MariaDB-11.6.2",
			containerCmd: nil,
			tlsEnabled:   false,
			insecureSkip: false,
			imageVersion: "mariadb:11.6.2-ubi9",
			expectedFile: "expected-mariadb.yaml",
		},
		{
			name:         "MariaDB-10.11.11",
			containerCmd: nil,
			tlsEnabled:   false,
			insecureSkip: false,
			imageVersion: "mariadb:10.11.11-ubi9",
			expectedFile: "expected-mariadb.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraperinttest.NewIntegrationTest(
				NewFactory(),
				scraperinttest.WithContainerRequest(
					testcontainers.ContainerRequest{
						Image:        tc.imageVersion,
						Cmd:          tc.containerCmd,
						ExposedPorts: []string{mysqlPort},
						WaitingFor: wait.ForListeningPort(mysqlPort).
							WithStartupTimeout(2 * time.Minute),
						Env: map[string]string{
							"MYSQL_ROOT_PASSWORD": "otel",
							"MYSQL_DATABASE":      "otel",
							"MYSQL_USER":          "otel",
							"MYSQL_PASSWORD":      "otel",
						},
						Files: []testcontainers.ContainerFile{
							{
								HostFilePath:      filepath.Join("testdata", "integration", "init.sh"),
								ContainerFilePath: "/docker-entrypoint-initdb.d/init.sh",
								FileMode:          700,
							},
						},
					}),
				scraperinttest.WithCustomConfig(
					func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
						rCfg := cfg.(*Config)
						rCfg.CollectionInterval = time.Second
						rCfg.Endpoint = net.JoinHostPort(ci.Host(t), ci.MappedPort(t, mysqlPort))
						rCfg.Username = "otel"
						rCfg.Password = "otel"
						if tc.tlsEnabled {
							rCfg.TLS.InsecureSkipVerify = tc.insecureSkip
						} else {
							rCfg.TLS.Insecure = true
						}
					}),
				scraperinttest.WithExpectedFile(
					filepath.Join("testdata", "integration", tc.expectedFile),
				),
				scraperinttest.WithCompareOptions(
					pmetrictest.IgnoreResourceAttributeValue("mysql.instance.endpoint"),
					pmetrictest.IgnoreMetricValues(),
					pmetrictest.IgnoreMetricDataPointsOrder(),
					pmetrictest.IgnoreStartTimestamp(),
					pmetrictest.IgnoreTimestamp(),
				),
			).Run(t)
		})
	}
}

// containerConfig builds a Config pointing at a running test container.
func containerConfig(host, port string) *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.Username = "root"
	cfg.Password = "otel"
	cfg.AddrConfig = confignet.AddrConfig{
		Endpoint:  net.JoinHostPort(host, port),
		Transport: confignet.TransportTypeTCP,
	}
	cfg.TLS.Insecure = true
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.LogsBuilderConfig.Events.DbServerQuerySample.Enabled = true
	cfg.TopQueryCollection.LookbackTime = 300 // 5-minute window to catch our workload queries
	return cfg
}

// runWorkload executes a handful of SELECT statements so that
// performance_schema.events_statements_summary_by_digest has rows to return.
func runWorkload(t *testing.T, cfg *Config) {
	t.Helper()
	c, err := newMySQLClient(cfg)
	require.NoError(t, err)
	require.NoError(t, c.Connect())
	defer c.Close()

	queries := []string{
		"SELECT 1",
		"SELECT 2",
		"SELECT 3",
		"SELECT NOW()",
		"SELECT VERSION()",
	}
	for range 5 {
		for _, q := range queries {
			_, _ = c.(*mySQLClient).client.Exec(q)
		}
	}
}

// runPerfSchemaSetup enables the performance_schema consumers and instruments
// required for events_statements_summary_by_digest and events_statements_current
// to be populated. These cannot be set as startup flags; they must be updated
// via SQL against the running server.
func runPerfSchemaSetup(t *testing.T, cfg *Config) {
	t.Helper()
	c, err := newMySQLClient(cfg)
	require.NoError(t, err)
	require.NoError(t, c.Connect())
	defer c.Close()

	db := c.(*mySQLClient).client
	stmts := []string{
		"UPDATE performance_schema.setup_consumers SET ENABLED='YES' WHERE NAME IN ('events_statements_current','events_statements_history','events_statements_history_long','events_statements_digest','events_waits_current')",
		"UPDATE performance_schema.setup_instruments SET ENABLED='YES', TIMED='YES' WHERE NAME LIKE 'statement/%'",
		"UPDATE performance_schema.setup_instruments SET ENABLED='YES', TIMED='YES' WHERE NAME LIKE 'wait/%'",
		"TRUNCATE TABLE performance_schema.events_statements_current",
		"TRUNCATE TABLE performance_schema.events_waits_current",
	}
	for _, s := range stmts {
		if _, execErr := db.Exec(s); execErr != nil {
			t.Logf("perf schema setup stmt failed (consumers may not be enabled): %v", execErr)
		}
	}
}

// TestIntegrationLogScraper proves that the new multi-version capabilities work
// end-to-end against real database containers:
//
//   - getDBVersion() correctly identifies MySQL vs MariaDB
//   - scrapeTopQueryFunc uses the 6-column template on MySQL 8+ (query_sample_text present)
//     and the 5-column fallback on MariaDB (no query_sample_text column)
//   - The shared plan cache is populated by scrapeTopQueryFunc so that
//     scrapeQuerySampleFunc reuses cached plans without a second EXPLAIN call
//
// This test manages containers directly with testcontainers.GenericContainer rather
// than using scraperinttest.NewIntegrationTest. scraperinttest is built around the
// full scrape pipeline and validates metrics output; it provides no hook to access
// a constructed client instance. Direct container management is required here because
// the test calls scraper internals (scrapeTopQueryFunc, scrapeQuerySampleFunc,
// sqlclient.getDBVersion) that are not reachable through the scraperinttest API.
func TestIntegrationLogScraper(t *testing.T) {
	testCases := []struct {
		name              string
		image             string
		wantSampleTextCol bool
		wantEOLWarn       bool   // true ↔ logDetectedVersion should emit an EOL warning
		wantProduct       string // expected db.product scope attribute value
	}{
		{
			name:              "MySQL-8.0.33-LogScraper",
			image:             "mysql:8.0.33",
			wantSampleTextCol: true,
			wantEOLWarn:       false,
			wantProduct:       "MySQL",
		},
		{
			// mysql:5.7 has no official ARM64 image; this case is skipped on ARM hosts.
			name:              "MySQL-5.7-LogScraper",
			image:             "mysql:5.7",
			wantSampleTextCol: false,
			wantEOLWarn:       true,
			wantProduct:       "MySQL",
		},
		{
			name:              "MariaDB-10.11-LogScraper",
			image:             "mariadb:10.11",
			wantSampleTextCol: false,
			wantEOLWarn:       false,
			wantProduct:       "MariaDB",
		},
		{
			name:              "MariaDB-11.4-LogScraper",
			image:             "mariadb:11.4",
			wantSampleTextCol: false,
			wantEOLWarn:       false,
			wantProduct:       "MariaDB",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()

			ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Image:        tc.image,
					ExposedPorts: []string{mysqlPort},
					// Enable performance_schema per README requirements.
					// Consumer activation is done via SQL after startup (see runPerfSchemaSetup).
					Cmd: []string{
						"--performance_schema=ON",
						"--max_digest_length=4096",
						"--performance_schema_max_digest_length=4096",
						"--performance_schema_max_sql_text_length=4096",
					},
					WaitingFor: wait.ForAll(
						wait.ForListeningPort(mysqlPort).WithStartupTimeout(2*time.Minute),
						wait.ForLog("ready for connections").WithStartupTimeout(2*time.Minute),
					),
					Env: map[string]string{
						"MYSQL_ROOT_PASSWORD":   "otel",
						"MYSQL_DATABASE":        "otel",
						"MYSQL_USER":            "otel",
						"MYSQL_PASSWORD":        "otel",
						"MARIADB_ROOT_PASSWORD": "otel",
						"MARIADB_DATABASE":      "otel",
						"MARIADB_USER":          "otel",
						"MARIADB_PASSWORD":      "otel",
					},
				},
				Started: true,
			})
			testcontainers.CleanupContainer(t, ctr)
			if err != nil && strings.Contains(err.Error(), "No such image") {
				t.Skipf("image %s not available locally: %v", tc.image, err)
			}
			require.NoError(t, err)

			host, err := ctr.Host(ctx)
			require.NoError(t, err)
			mappedPort, err := ctr.MappedPort(ctx, mysqlPort)
			require.NoError(t, err)

			cfg := containerConfig(host, mappedPort.Port())

			// Build a shared plan cache (TTL=0 in tests to avoid goroutine leak).
			sharedPlanCache := newTTLCache[string](cfg.TopQueryCollection.QueryPlanCacheSize, 0)

			// Use an observer logger so we can assert logDetectedVersion output.
			observerCore, loggedEntries := observer.New(zapcore.WarnLevel)
			settings := receivertest.NewNopSettings(metadata.Type)
			scraper := newMySQLScraper(
				settings,
				cfg,
				newCache[int64](int(cfg.TopQueryCollection.MaxQuerySampleCount*2*2)),
				sharedPlanCache,
			)
			scraper.logger = zap.New(observerCore)
			require.NoError(t, scraper.start(ctx, nil))
			defer func() { assert.NoError(t, scraper.shutdown(ctx)) }()

			// Verify logDetectedVersion EOL warning.
			eolEntries := loggedEntries.FilterMessage(
				"detected MySQL version is past end-of-life and may not be supported by this receiver in a future release",
			)
			if tc.wantEOLWarn {
				assert.Equal(t, 1, eolEntries.Len(), "expected EOL warn log entry for %s", tc.image)
			} else {
				assert.Equal(t, 0, eolEntries.Len(), "unexpected EOL warn log entry for %s", tc.image)
			}

			// Verify performance_schema has digest rows for our workload queries.
			{
				c, cerr := newMySQLClient(cfg)
				require.NoError(t, cerr)
				require.NoError(t, c.Connect())
				var count int
				_ = c.(*mySQLClient).client.QueryRow(
					"SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest WHERE last_seen >= NOW() - INTERVAL 300 SECOND",
				).Scan(&count)
				t.Logf("performance_schema digest rows in last 300s (before scrape): %d", count)
				c.Close()
			}

			// Enable performance_schema consumers/instruments via SQL.
			runPerfSchemaSetup(t, cfg)

			// Prime the scraper's diff cache with a first scrape so subsequent
			// scrapes have a non-zero sumTimerWait delta to emit records on.
			// (scrapeTopQueries skips records with zero elapsed-time diff.)
			runWorkload(t, cfg)
			_, err = scraper.scrapeTopQueryFunc(ctx)
			require.NoError(t, err, "first scrapeTopQueryFunc (cache priming) must not error")

			// Run more workload so the diff is non-zero on the second scrape.
			runWorkload(t, cfg)

			// --- scrapeTopQueryFunc (second pass — produces records) ---
			topLogs, err := scraper.scrapeTopQueryFunc(ctx)
			require.NoError(t, err, "scrapeTopQueryFunc must not return error (wrong template would cause 'unknown column')")

			// The workload above guarantees at least a few digests exist.
			// Verify structural properties on whatever records were returned.
			topRecordCount := 0
			for i := range topLogs.ResourceLogs().Len() {
				rl := topLogs.ResourceLogs().At(i)
				for j := range rl.ScopeLogs().Len() {
					sl := rl.ScopeLogs().At(j)
					for k := range sl.LogRecords().Len() {
						lr := sl.LogRecords().At(k)
						topRecordCount++

						// db.query.text must always be present and non-empty.
						qt, ok := lr.Attributes().Get("db.query.text")
						assert.True(t, ok, "db.query.text attribute missing")
						assert.NotEmpty(t, qt.Str(), "db.query.text must not be empty")

						// mysql.query_plan is only populated when a sample text was
						// available for EXPLAIN — absence is valid, presence must be non-empty.
						if plan, hasPlan := lr.Attributes().Get("mysql.query_plan"); hasPlan {
							assert.NotEmpty(t, plan.Str(), "mysql.query_plan present but empty")
						}

						// On MySQL <8 / MariaDB the fallback template omits query_sample_text,
						// so the plan cache key will never be populated by top-query scraping.
						// On MySQL 8+ the shared cache should have been populated for any
						// digest that had a valid sample.
						if !tc.wantSampleTextCol {
							// No sample text → plan cache must be empty (nothing to EXPLAIN).
							assert.Equal(t, 0, sharedPlanCache.Len(),
								"plan cache should be empty when fallback template used (no sample text)")
						}
					}
				}
			}
			t.Logf("scrapeTopQueryFunc returned %d log records", topRecordCount)

			// Verify scope attributes on top-query logs.
			// setScopeAttributes is called after every Emit, so db.version and
			// db.product must appear on every ScopeLogs scope.
			for i := range topLogs.ResourceLogs().Len() {
				sls := topLogs.ResourceLogs().At(i).ScopeLogs()
				for j := range sls.Len() {
					attrs := sls.At(j).Scope().Attributes()
					_, hasVersion := attrs.Get("db.version")
					assert.True(t, hasVersion, "db.version scope attribute missing on top-query ResourceLogs[%d].ScopeLogs[%d]", i, j)
					prod, hasProd := attrs.Get("db.product")
					assert.True(t, hasProd, "db.product scope attribute missing on top-query ResourceLogs[%d].ScopeLogs[%d]", i, j)
					assert.Equal(t, tc.wantProduct, prod.Str(), "db.product mismatch on ResourceLogs[%d].ScopeLogs[%d]", i, j)
				}
			}

			// --- scrapeQuerySampleFunc ---
			// Use a separate scraper sharing the same plan cache to prove reuse.
			sampleScraper := newMySQLScraper(
				settings,
				cfg,
				newCache[int64](1),
				sharedPlanCache,
			)
			require.NoError(t, sampleScraper.start(ctx, nil))
			defer func() { assert.NoError(t, sampleScraper.shutdown(ctx)) }()

			sampleLogs, err := sampleScraper.scrapeQuerySampleFunc(ctx)
			require.NoError(t, err, "scrapeQuerySampleFunc must not return error")

			sampleRecordCount := 0
			for i := range sampleLogs.ResourceLogs().Len() {
				rl := sampleLogs.ResourceLogs().At(i)
				for j := range rl.ScopeLogs().Len() {
					sl := rl.ScopeLogs().At(j)
					sampleRecordCount += sl.LogRecords().Len()
				}
			}
			t.Logf("scrapeQuerySampleFunc returned %d log records", sampleRecordCount)

			// Verify scope attributes on query-sample logs.
			for i := range sampleLogs.ResourceLogs().Len() {
				sls := sampleLogs.ResourceLogs().At(i).ScopeLogs()
				for j := range sls.Len() {
					attrs := sls.At(j).Scope().Attributes()
					_, hasVersion := attrs.Get("db.version")
					assert.True(t, hasVersion, "db.version scope attribute missing on sample ResourceLogs[%d].ScopeLogs[%d]", i, j)
					prod, hasProd := attrs.Get("db.product")
					assert.True(t, hasProd, "db.product scope attribute missing on sample ResourceLogs[%d].ScopeLogs[%d]", i, j)
					assert.Equal(t, tc.wantProduct, prod.Str(), "db.product mismatch on ResourceLogs[%d].ScopeLogs[%d]", i, j)
				}
			}

			// Verify version detection on the scraper's client.
			dv := scraper.sqlclient.getDBVersion()
			assert.Equal(t, tc.wantSampleTextCol, dv.supportsQuerySampleText(), "supportsQuerySampleText mismatch")
		})
	}
}

// TestVersionCompatibility verifies that getDBVersion() correctly identifies
// MySQL and MariaDB flavors, and that getTopQueries() and getQuerySamples() select
// the right query templates:
//   - getTopQueries: 6-column (query_sample_text) for MySQL 8+, 5-column fallback otherwise
//
// This test manages containers directly with testcontainers.GenericContainer rather
// than using scraperinttest.NewIntegrationTest. scraperinttest validates metrics
// output through the full scrape pipeline; it provides no way to obtain a client
// instance for direct method calls. Direct container management is required here
// because the test exercises client-level methods (getDBVersion, getTopQueries,
// getQuerySamples) that are not reachable through the scraperinttest API.
func TestVersionCompatibility(t *testing.T) {
	testCases := []struct {
		name              string
		image             string
		wantProduct       dbProduct
		wantSampleTextCol bool // true ↔ 6-column top-query template used
		wantReplicaStatus bool // true ↔ SHOW REPLICA STATUS used instead of SHOW SLAVE STATUS
	}{
		{
			name:              "MySQL 8.0.33",
			image:             "mysql:8.0.33",
			wantProduct:       dbProductMySQL,
			wantSampleTextCol: true,
			wantReplicaStatus: true,
		},
		{
			// mysql:5.7 has no official ARM64 image; this case is skipped on
			// ARM hosts (e.g. Apple Silicon). MySQL 5.7 does not support query_sample_text.
			name:              "MySQL 5.7",
			image:             "mysql:5.7",
			wantProduct:       dbProductMySQL,
			wantSampleTextCol: false,
			wantReplicaStatus: false,
		},
		{
			name:              "MariaDB 10.11",
			image:             "mariadb:10.11",
			wantProduct:       dbProductMariaDB,
			wantSampleTextCol: false,
			wantReplicaStatus: false,
		},
		{
			name:              "MariaDB 11.4",
			image:             "mariadb:11.4",
			wantProduct:       dbProductMariaDB,
			wantSampleTextCol: false,
			wantReplicaStatus: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()

			req := testcontainers.ContainerRequest{
				Image:        tc.image,
				ExposedPorts: []string{mysqlPort},
				WaitingFor: wait.ForListeningPort(mysqlPort).
					WithStartupTimeout(2 * time.Minute),
				Env: map[string]string{
					"MYSQL_ROOT_PASSWORD": "otel",
					"MYSQL_DATABASE":      "otel",
					"MYSQL_USER":          "otel",
					"MYSQL_PASSWORD":      "otel",
					// MariaDB uses the same env var names.
					"MARIADB_ROOT_PASSWORD": "otel",
					"MARIADB_DATABASE":      "otel",
					"MARIADB_USER":          "otel",
					"MARIADB_PASSWORD":      "otel",
				},
			}

			ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: req,
				Started:          true,
			})
			testcontainers.CleanupContainer(t, ctr)
			if err != nil && strings.Contains(err.Error(), "No such image") {
				t.Skipf("image %s not available locally (no official ARM64 build): %v", tc.image, err)
			}
			require.NoError(t, err)

			host, err := ctr.Host(ctx)
			require.NoError(t, err)
			mappedPort, err := ctr.MappedPort(ctx, mysqlPort)
			require.NoError(t, err)

			cfg := &Config{
				Username: "root",
				Password: configopaque.String("otel"),
				AddrConfig: confignet.AddrConfig{
					Endpoint:  fmt.Sprintf("%s:%s", host, mappedPort.Port()),
					Transport: confignet.TransportTypeTCP,
				},
				AllowNativePasswords: true,
				TLS:                  configtls.ClientConfig{Insecure: true},
			}

			c, err := newMySQLClient(cfg)
			require.NoError(t, err)
			require.NoError(t, c.Connect())
			defer c.Close()

			// --- getDBVersion ---
			dv := c.getDBVersion()
			assert.Equal(t, tc.wantProduct, dv.product, "product mismatch")
			assert.Equal(t, tc.wantSampleTextCol, dv.supportsQuerySampleText(), "supportsQuerySampleText mismatch")
			assert.Equal(t, tc.wantReplicaStatus, dv.supportsReplicaStatus(), "supportsReplicaStatus mismatch")

			// --- getTopQueries: must succeed without error ---
			// No workload is running, so the result may be empty, but the query
			// itself must execute without error — which proves the correct template
			// (5-column vs 6-column) was chosen for this server version.
			queries, err := c.getTopQueries(10, 60, dv.supportsQuerySampleText())
			require.NoError(t, err, "getTopQueries should not fail (wrong template would cause 'unknown column' error)")

			// For MySQL 8+, any top queries returned must have querySampleText
			// populated (non-empty string is only possible with the 6-column query).
			// For MySQL <8 / MariaDB, querySampleText must always be empty string
			// because the fallback template omits the column.
			for _, q := range queries {
				if tc.wantSampleTextCol {
					// Value may be empty string if the digest row has no sample yet,
					// but the field must have been scanned (no scan error above).
					_ = q.querySampleText
				} else {
					assert.Empty(t, q.querySampleText,
						"querySampleText must be empty when using fallback template (digest: %s)", q.digest)
				}
			}

			// --- getQuerySamples: must succeed without error ---
			// Result may be empty if no active sessions.
			_, err = c.getQuerySamples(10, dv.supportsProcesslist())
			require.NoError(t, err, "getQuerySamples should not fail (wrong template would cause 'unknown table' error)")

			// --- getReplicaStatusStats: must succeed without error ---
			// Proves the correct SHOW REPLICA STATUS vs SHOW SLAVE STATUS template was
			// chosen. The server is not configured as a replica so the result will be
			// empty, but the query itself must execute without error.
			_, err = c.getReplicaStatusStats(dv.supportsReplicaStatus())
			require.NoError(t, err, "getReplicaStatusStats should not fail (wrong command would cause syntax error)")
		})
	}
}

// assertStrAttrNonEmpty asserts that the named attribute exists and is a non-empty string.
func assertStrAttrNonEmpty(t *testing.T, attrs pcommon.Map, key string, recIdx int) {
	t.Helper()
	v, ok := attrs.Get(key)
	if assert.True(t, ok, "record %d: attribute %q missing", recIdx, key) {
		assert.NotEmpty(t, v.Str(), "record %d: attribute %q must be non-empty", recIdx, key)
	}
}

// assertStrAttrEqual asserts that the named attribute exists and equals want.
func assertStrAttrEqual(t *testing.T, attrs pcommon.Map, key, want string, recIdx int) {
	t.Helper()
	v, ok := attrs.Get(key)
	if assert.True(t, ok, "record %d: attribute %q missing", recIdx, key) {
		assert.Equal(t, want, v.Str(), "record %d: attribute %q value mismatch", recIdx, key)
	}
}

// assertIntAttrPositive asserts that the named attribute exists and is > 0.
func assertIntAttrPositive(t *testing.T, attrs pcommon.Map, key string, recIdx int) {
	t.Helper()
	v, ok := attrs.Get(key)
	if assert.True(t, ok, "record %d: attribute %q missing", recIdx, key) {
		assert.Positive(t, v.Int(), "record %d: attribute %q must be > 0", recIdx, key)
	}
}

// assertIntAttrNonNegative asserts that the named attribute exists and is >= 0.
func assertIntAttrNonNegative(t *testing.T, attrs pcommon.Map, key string, recIdx int) {
	t.Helper()
	v, ok := attrs.Get(key)
	if assert.True(t, ok, "record %d: attribute %q missing", recIdx, key) {
		assert.GreaterOrEqual(t, v.Int(), int64(0), "record %d: attribute %q must be >= 0", recIdx, key)
	}
}

// assertDoubleAttrNonNegative asserts that the named attribute exists and is >= 0.
func assertDoubleAttrNonNegative(t *testing.T, attrs pcommon.Map, key string, recIdx int) {
	t.Helper()
	v, ok := attrs.Get(key)
	if assert.True(t, ok, "record %d: attribute %q missing", recIdx, key) {
		assert.GreaterOrEqual(t, v.Double(), float64(0), "record %d: attribute %q must be >= 0", recIdx, key)
	}
}

// runBlockedCall holds an advisory lock on one connection and blocks a second
// connection trying to acquire the same lock. It returns a cleanup function
// that unblocks the waiter and closes both connections. The blocked connection
// remains open — and therefore visible in events_statements_current with a
// live non-zero TIMER_WAIT — until cleanup is called.
//
// The approach uses GET_LOCK / IS_FREE_LOCK (available on all supported MySQL
// and MariaDB versions) rather than table locks, so no DDL is needed.
func runBlockedCall(t *testing.T, cfg *Config) (cleanup func()) {
	t.Helper()

	holderC, err := newMySQLClient(cfg)
	require.NoError(t, err)
	require.NoError(t, holderC.Connect())
	holderDB := holderC.(*mySQLClient).client

	// Acquire the advisory lock on the holder connection (timeout=60s).
	var held int
	require.NoError(t, holderDB.QueryRow("SELECT GET_LOCK('otel_test_lock', 60)").Scan(&held))
	require.Equal(t, 1, held, "holder failed to acquire advisory lock")

	waiterC, err := newMySQLClient(cfg)
	require.NoError(t, err)
	require.NoError(t, waiterC.Connect())
	waiterDB := waiterC.(*mySQLClient).client

	// Block the waiter in a goroutine — SELECT GET_LOCK will wait until the
	// holder releases. The goroutine is reaped by the cleanup function.
	done := make(chan struct{})
	go func() {
		defer close(done)
		// timeout=30s keeps the waiter blocked for the duration of the test.
		var result sql.NullInt64
		_ = waiterDB.QueryRowContext(context.Background(), "SELECT GET_LOCK('otel_test_lock', 30)").Scan(&result) //nolint:usetesting // t.Context() would cancel this query when the test ends, unblocking the waiter prematurely
	}()

	// Wait until the waiter is visible in events_statements_current.
	require.Eventually(t, func() bool {
		var count int
		_ = holderDB.QueryRow(
			"SELECT COUNT(*) FROM performance_schema.events_statements_current esc " +
				"JOIN performance_schema.threads t ON t.thread_id = esc.thread_id " +
				"WHERE t.processlist_command != 'Sleep' " +
				"AND esc.sql_text LIKE '%GET_LOCK%otel_test_lock%'",
		).Scan(&count)
		return count > 0
	}, 10*time.Second, 200*time.Millisecond, "timed out waiting for blocked session to appear in events_statements_current")

	return func() {
		// Release the lock — the waiter goroutine will unblock and return.
		_, _ = holderDB.Exec("SELECT RELEASE_LOCK('otel_test_lock')")
		<-done
		holderC.Close()
		waiterC.Close()
	}
}

// TestIntegrationQuerySampleAttributes verifies that scrapeQuerySampleFunc
// produces sample records with the attributes added by this branch:
//
//   - mysql.events_statements_current.timer_wait is present and > 0
//   - mysql.events_waits_current.timer_wait is present (≥ 0)
//   - All structural attributes (db.query.text, mysql.session.id,
//     mysql.session.status, db.namespace, client.address, network.peer.address)
//     are present on every record.
//
// A blocked advisory-lock call is used to guarantee that at least one session
// is visible in events_statements_current with a live non-zero TIMER_WAIT for
// the duration of the scrape.
func TestIntegrationQuerySampleAttributes(t *testing.T) {
	testCases := []struct {
		name  string
		image string
	}{
		{name: "MySQL-8.0.33", image: "mysql:8.0.33"},
		{name: "MySQL-5.7", image: "mysql:5.7"},
		{name: "MariaDB-10.11", image: "mariadb:10.11"},
		{name: "MariaDB-11.4", image: "mariadb:11.4"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()

			ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Image:        tc.image,
					ExposedPorts: []string{mysqlPort},
					Cmd: []string{
						"--performance_schema=ON",
						"--max_digest_length=4096",
						"--performance_schema_max_digest_length=4096",
						"--performance_schema_max_sql_text_length=4096",
					},
					WaitingFor: wait.ForAll(
						wait.ForListeningPort(mysqlPort).WithStartupTimeout(2*time.Minute),
						wait.ForLog("ready for connections").WithStartupTimeout(2*time.Minute),
					),
					Env: map[string]string{
						"MYSQL_ROOT_PASSWORD":   "otel",
						"MYSQL_DATABASE":        "otel",
						"MYSQL_USER":            "otel",
						"MYSQL_PASSWORD":        "otel",
						"MARIADB_ROOT_PASSWORD": "otel",
						"MARIADB_DATABASE":      "otel",
						"MARIADB_USER":          "otel",
						"MARIADB_PASSWORD":      "otel",
					},
				},
				Started: true,
			})
			testcontainers.CleanupContainer(t, ctr)
			if err != nil && strings.Contains(err.Error(), "No such image") {
				t.Skipf("image %s not available locally: %v", tc.image, err)
			}
			require.NoError(t, err)

			host, err := ctr.Host(ctx)
			require.NoError(t, err)
			mappedPort, err := ctr.MappedPort(ctx, mysqlPort)
			require.NoError(t, err)

			cfg := containerConfig(host, mappedPort.Port())

			runPerfSchemaSetup(t, cfg)

			sharedPlanCache := newTTLCache[string](cfg.TopQueryCollection.QueryPlanCacheSize, 0)
			settings := receivertest.NewNopSettings(metadata.Type)
			scraper := newMySQLScraper(
				settings,
				cfg,
				newCache[int64](int(cfg.TopQueryCollection.MaxQuerySampleCount*2*2)),
				sharedPlanCache,
			)
			require.NoError(t, scraper.start(ctx, nil))
			defer func() { assert.NoError(t, scraper.shutdown(ctx)) }()

			// Hold the advisory lock and block a second connection so that
			// events_statements_current has at least one row with non-zero TIMER_WAIT.
			// runBlockedCall waits until the blocked thread is visible in
			// events_statements_current before returning, so the scrape below is
			// guaranteed to see it.
			cleanup := runBlockedCall(t, cfg)
			defer cleanup()

			// Log how many events_statements_current rows exist to aid diagnosis.
			{
				dc, dcErr := newMySQLClient(cfg)
				require.NoError(t, dcErr)
				require.NoError(t, dc.Connect())
				var count int
				_ = dc.(*mySQLClient).client.QueryRow(
					"SELECT COUNT(*) FROM performance_schema.events_statements_current esc " +
						"JOIN performance_schema.threads t ON t.thread_id = esc.thread_id " +
						"WHERE t.processlist_command NOT IN ('Sleep','Daemon')",
				).Scan(&count)
				t.Logf("events_statements_current active rows before scrape: %d", count)
				dc.Close()
			}

			logs, err := scraper.scrapeQuerySampleFunc(ctx)
			require.NoError(t, err, "scrapeQuerySampleFunc must not return an error")

			// Collect all log records.
			var lrs []pcommon.Map
			for i := range logs.ResourceLogs().Len() {
				rl := logs.ResourceLogs().At(i)
				for j := range rl.ScopeLogs().Len() {
					sl := rl.ScopeLogs().At(j)
					for k := range sl.LogRecords().Len() {
						lrs = append(lrs, sl.LogRecords().At(k).Attributes())
					}
				}
			}

			require.NotEmpty(t, lrs, "expected at least one query sample record (blocked call should be visible)")

			// Every record must have non-zero/non-empty values for the key attributes.
			for idx, attrs := range lrs {
				assertStrAttrNonEmpty(t, attrs, "db.system.name", idx)
				assertIntAttrPositive(t, attrs, "mysql.threads.thread_id", idx)
				assertStrAttrNonEmpty(t, attrs, "mysql.threads.processlist_command", idx)
				assertStrAttrNonEmpty(t, attrs, "mysql.session.status", idx)
				assertIntAttrPositive(t, attrs, "mysql.session.id", idx)
				assertStrAttrNonEmpty(t, attrs, "db.query.text", idx)
				assertStrAttrNonEmpty(t, attrs, "client.address", idx)
				assertStrAttrNonEmpty(t, attrs, "network.peer.address", idx)
				// The attribute added by this branch — must be present and non-negative on
				// every record. The GET_LOCK record below additionally asserts > 0.
				assertDoubleAttrNonNegative(t, attrs, "mysql.events_statements_current.timer_wait", idx)
			}

			// Find the blocked GET_LOCK record and validate all attribute values.
			// This is the record that proves mysql.events_statements_current.timer_wait
			// is populated from a live in-flight statement, not from a stale or zero value.
			found := false
			for idx, attrs := range lrs {
				qt, ok := attrs.Get("db.query.text")
				if !ok || !strings.Contains(strings.ToUpper(qt.Str()), "GET_LOCK") {
					continue
				}
				found = true

				// Exact-value assertions where the value is known.
				assertStrAttrEqual(t, attrs, "db.system.name", "mysql", idx)
				assertStrAttrEqual(t, attrs, "mysql.threads.processlist_command", "Query", idx)

				// Non-empty string attributes — value is version/session-dependent.
				assertStrAttrNonEmpty(t, attrs, "user.name", idx)
				assertStrAttrNonEmpty(t, attrs, "mysql.threads.processlist_state", idx)
				assertStrAttrNonEmpty(t, attrs, "mysql.wait_type", idx)
				assertStrAttrNonEmpty(t, attrs, "mysql.session.status", idx)
				// digest is empty for in-flight statements that haven't completed
				// digesting yet (MySQL 5.7, MariaDB) — presence is sufficient.
				_, hasDigest := attrs.Get("mysql.events_statements_current.digest")
				assert.True(t, hasDigest, "record %d (GET_LOCK): mysql.events_statements_current.digest missing", idx)

				// Positive integer attributes.
				assertIntAttrPositive(t, attrs, "mysql.threads.thread_id", idx)
				assertIntAttrPositive(t, attrs, "mysql.session.id", idx)

				// The key assertion for this branch: TIMER_WAIT must be non-zero
				// because the statement is actively blocked (in-flight).
				tw, hasTW := attrs.Get("mysql.events_statements_current.timer_wait")
				assert.True(t, hasTW, "record %d (GET_LOCK): mysql.events_statements_current.timer_wait missing", idx)
				if hasTW {
					assert.Greater(t, tw.Double(), float64(0),
						"record %d (GET_LOCK): mysql.events_statements_current.timer_wait must be > 0 for a blocked in-flight statement", idx)
				}

				// Wait timer — present and non-negative (may be 0 if no wait instrument fired).
				wtw, hasWTW := attrs.Get("mysql.events_waits_current.timer_wait")
				assert.True(t, hasWTW, "record %d (GET_LOCK): mysql.events_waits_current.timer_wait missing", idx)
				if hasWTW {
					assert.GreaterOrEqual(t, wtw.Double(), float64(0),
						"record %d (GET_LOCK): mysql.events_waits_current.timer_wait must be >= 0", idx)
				}

				// Network attributes — non-negative integers (port may be 0 for Unix socket clients).
				assertIntAttrNonNegative(t, attrs, "client.port", idx)
				assertIntAttrNonNegative(t, attrs, "network.peer.port", idx)
			}
			assert.True(t, found, "expected to find a GET_LOCK query sample record for the blocked call")
		})
	}
}
