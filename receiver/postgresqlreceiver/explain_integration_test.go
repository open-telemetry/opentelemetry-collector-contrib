// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package postgresqlreceiver

import (
	"database/sql"
	_ "embed"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

//go:embed explain.sql
var explainFunctionSQL string

func TestExplainFunctionIntegration(t *testing.T) {
	for _, version := range []string{pre17TestVersion, post17TestVersion} {
		t.Run(version, func(t *testing.T) {
			ci, err := testcontainers.GenericContainer(t.Context(), testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Image:        "postgres:" + version,
					Cmd:          []string{"-c", "shared_preload_libraries=pg_stat_statements"},
					Env:          map[string]string{"POSTGRES_PASSWORD": "password", "POSTGRES_DB": "otel"},
					ExposedPorts: []string{postgresqlPort},
					WaitingFor: wait.ForLog("database system is ready to accept connections").
						WithOccurrence(2).WithStartupTimeout(2 * time.Minute),
				},
				Started: true,
			})
			require.NoError(t, err)
			testcontainers.CleanupContainer(t, ci)
			host, err := ci.Host(t.Context())
			require.NoError(t, err)
			port, err := ci.MappedPort(t.Context(), postgresqlPort)
			require.NoError(t, err)
			endpoint := net.JoinHostPort(host, port.Port())
			connect := func(user, password string) *sql.DB {
				t.Helper()
				dsn := url.URL{Scheme: "postgres", User: url.UserPassword(user, password), Host: endpoint, Path: "otel", RawQuery: "sslmode=disable"}
				db, openErr := sql.Open("postgres", dsn.String())
				require.NoError(t, openErr)
				db.SetMaxOpenConns(1)
				t.Cleanup(func() { assert.NoError(t, db.Close()) })
				return db
			}
			admin := connect("postgres", "password")
			_, err = admin.ExecContext(t.Context(), `
CREATE ROLE explain_owner NOLOGIN;
CREATE EXTENSION pg_stat_statements;
CREATE SCHEMA otel AUTHORIZATION explain_owner;
CREATE ROLE monitor LOGIN PASSWORD 'monitor';
CREATE ROLE outsider;
GRANT pg_monitor TO monitor;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE TEMPORARY ON DATABASE otel FROM PUBLIC;
CREATE TABLE public.explain_items(id integer PRIMARY KEY, value text);
INSERT INTO public.explain_items VALUES (1, 'unchanged');
GRANT USAGE ON SCHEMA public TO explain_owner;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.explain_items TO explain_owner;
GRANT SELECT ON public.explain_items TO monitor;
CREATE FUNCTION public.slow_plan() RETURNS integer LANGUAGE plpgsql IMMUTABLE
AS $$ BEGIN PERFORM pg_sleep(1); RETURN 1; END; $$;
SET ROLE explain_owner;`)
			require.NoError(t, err)
			_, err = admin.ExecContext(t.Context(), explainFunctionSQL)
			require.NoError(t, err)
			_, err = admin.ExecContext(t.Context(), `RESET ROLE;
GRANT USAGE ON SCHEMA otel TO monitor;
GRANT EXECUTE ON FUNCTION otel.explain_statement(text) TO monitor;`)
			require.NoError(t, err)
			monitor := connect("monitor", "monitor")
			client := &postgreSQLClient{client: monitor}
			quoted := quoteExplainFunctionName("otel.explain_statement")
			require.NoError(t, client.probeExplainFunction(t.Context(), quoted))

			t.Run("write and locking plans without write grants", func(t *testing.T) {
				queries := []struct{ query, node string }{
					{"UPDATE explain_items SET value = $1 WHERE id = $2", "ModifyTable"},
					{"INSERT INTO explain_items VALUES ($1, $2)", "ModifyTable"},
					{"DELETE FROM explain_items WHERE id = $1", "ModifyTable"},
					{"SELECT * FROM explain_items WHERE id = $1 FOR UPDATE", "LockRows"},
					{"SELECT * FROM explain_items FOR SHARE;\n", "LockRows"},
				}
				if version == post17TestVersion {
					queries = append(queries, struct{ query, node string }{
						"MERGE INTO explain_items t USING (SELECT $1::integer AS id, $2::text AS value) s ON t.id = s.id WHEN MATCHED THEN UPDATE SET value = s.value", "ModifyTable",
					})
				}
				for i, tc := range queries {
					_, inlineErr := client.explainQuery(t.Context(), tc.query, fmt.Sprint(i), "", zap.NewNop())
					var permissionErr *pq.Error
					require.ErrorAs(t, inlineErr, &permissionErr)
					assert.Equal(t, "42501", permissionErr.SQLState())
					plan, planErr := client.explainQuery(t.Context(), tc.query, fmt.Sprint(i), quoted, zap.NewNop())
					require.NoError(t, planErr)
					assert.Contains(t, plan, tc.node)
					assert.NotContains(t, plan, "One-Time Filter", "NULL arguments must not produce a no-op custom plan")
				}
				var value string
				require.NoError(t, admin.QueryRowContext(t.Context(), "SELECT value FROM public.explain_items WHERE id = 1").Scan(&value))
				assert.Equal(t, "unchanged", value)
				var count int
				require.NoError(t, admin.QueryRowContext(t.Context(), "SELECT count(*) FROM public.explain_items").Scan(&count))
				assert.Equal(t, 1, count)
			})

			t.Run("refuses callers that can create temporary functions", func(t *testing.T) {
				_, err := admin.ExecContext(t.Context(), "GRANT TEMPORARY ON DATABASE otel TO monitor")
				require.NoError(t, err)
				// Planning folds this IMMUTABLE call and would run it as the helper owner.
				_, err = monitor.ExecContext(t.Context(), `
CREATE FUNCTION pg_temp.leak(integer) RETURNS integer LANGUAGE plpgsql IMMUTABLE
AS $$ BEGIN RAISE EXCEPTION 'leaked %', (SELECT value FROM public.explain_items WHERE id = 1); END; $$;`)
				require.NoError(t, err)
				_, err = client.explainQuery(t.Context(), "SELECT pg_temp.leak(1)", "leak", quoted, zap.NewNop())
				var queryErr *pq.Error
				require.ErrorAs(t, err, &queryErr)
				assert.Equal(t, "42501", queryErr.SQLState())
				assert.NotContains(t, queryErr.Message, "leaked")
				_, err = monitor.ExecContext(t.Context(), "DROP FUNCTION pg_temp.leak(integer)")
				require.NoError(t, err)
				_, err = admin.ExecContext(t.Context(), "REVOKE TEMPORARY ON DATABASE otel FROM monitor")
				require.NoError(t, err)
			})

			t.Run("rejects additional commands at the function boundary", func(t *testing.T) {
				var plan string
				err := monitor.QueryRowContext(t.Context(), "SELECT otel.explain_statement($1)",
					"SELECT 1; SELECT pg_advisory_lock(918273645)").Scan(&plan)
				var queryErr *pq.Error
				require.ErrorAs(t, err, &queryErr)
				assert.Equal(t, "22023", queryErr.SQLState())
				var locks int
				require.NoError(t, monitor.QueryRowContext(t.Context(),
					"SELECT count(*) FROM pg_locks WHERE pid = pg_backend_pid() AND locktype = 'advisory'").Scan(&locks))
				assert.Zero(t, locks)
			})

			t.Run("cancellation leaves the connection usable", func(t *testing.T) {
				_, err := monitor.ExecContext(t.Context(), "SET statement_timeout = '30ms'")
				require.NoError(t, err)
				_, err = client.explainQuery(t.Context(), "SELECT public.slow_plan()", "slow", quoted, zap.NewNop())
				var queryErr *pq.Error
				require.ErrorAs(t, err, &queryErr)
				assert.Equal(t, "57014", queryErr.SQLState())
				_, err = monitor.ExecContext(t.Context(), "SET statement_timeout = 0")
				require.NoError(t, err)
				require.NoError(t, client.probeExplainFunction(t.Context(), quoted))
				var prepared int
				require.NoError(t, monitor.QueryRowContext(t.Context(),
					"SELECT count(*) FROM pg_prepared_statements WHERE name = 'otel_explain_stmt'").Scan(&prepared))
				assert.Zero(t, prepared)
			})

			t.Run("preserves caller prepared statements on failure", func(t *testing.T) {
				_, err := monitor.ExecContext(t.Context(), "PREPARE otel_explain_stmt AS SELECT 2")
				require.NoError(t, err)
				err = client.probeExplainFunction(t.Context(), quoted)
				var queryErr *pq.Error
				require.ErrorAs(t, err, &queryErr)
				assert.Equal(t, "42P05", queryErr.SQLState())
				var value int
				require.NoError(t, monitor.QueryRowContext(t.Context(), "EXECUTE otel_explain_stmt").Scan(&value))
				assert.Equal(t, 2, value)
				_, err = monitor.ExecContext(t.Context(), "DEALLOCATE otel_explain_stmt")
				require.NoError(t, err)
			})

			t.Run("does not leak function settings or public execute access", func(t *testing.T) {
				_, err := monitor.ExecContext(t.Context(), "SET plan_cache_mode = auto")
				require.NoError(t, err)
				require.NoError(t, client.probeExplainFunction(t.Context(), quoted))
				var mode string
				require.NoError(t, monitor.QueryRowContext(t.Context(), "SHOW plan_cache_mode").Scan(&mode))
				assert.Equal(t, "auto", mode)
				var publicAccess bool
				require.NoError(t, admin.QueryRowContext(t.Context(),
					"SELECT has_function_privilege('outsider', 'otel.explain_statement(text)', 'EXECUTE')").Scan(&publicAccess))
				assert.False(t, publicAccess)
			})

			t.Run("emits real query plan events", func(t *testing.T) {
				_, err := admin.ExecContext(t.Context(), "UPDATE public.explain_items SET value = 'unchanged' WHERE id = 1")
				require.NoError(t, err)
				for _, pooled := range []bool{false, true} {
					t.Run(fmt.Sprintf("pool=%t", pooled), func(t *testing.T) {
						defer testutil.SetFeatureGateForTest(t, metadata.ReceiverPostgresqlConnectionPoolFeatureGate, pooled)()
						cfg := createDefaultConfig().(*Config)
						cfg.Username = "monitor"
						cfg.Password = "monitor"
						cfg.AddrConfig.Endpoint = endpoint
						cfg.ClientConfig.Insecure = true
						cfg.ConnectDatabase = "otel"
						cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
						cfg.LogsBuilderConfig.Events.DbServerQueryPlan.Enabled = true
						cfg.TopQueryCollection.ExplainFunctionName = "otel.explain_statement"
						p, err := newTopQueryScraper(receivertest.NewNopSettings(metadata.Type), cfg, newClientFactory(cfg))
						require.NoError(t, err)
						t.Cleanup(func() { assert.NoError(t, p.shutdown(t.Context())) })
						logs, err := p.scrapeTopQuery(t.Context(), 100, 100, 100, time.Minute)
						require.NoError(t, err)
						require.Equal(t, 1, logs.ResourceLogs().Len())
						scopes := logs.ResourceLogs().At(0).ScopeLogs()
						require.Equal(t, 1, scopes.Len())
						records := scopes.At(0).LogRecords()
						found := false
						for i := 0; i < records.Len(); i++ {
							record := records.At(i)
							plan, ok := record.Attributes().Get("postgresql.query_plan")
							if record.EventName() == "db.server.query_plan" && ok && strings.Contains(plan.Str(), `"Relation Name":"explain_items"`) && strings.Contains(plan.Str(), `"Node Type":"ModifyTable"`) {
								found = true
							}
						}
						assert.True(t, found, "a write query plan must reach the emitted logs")
					})
				}
			})
		})
	}
}
