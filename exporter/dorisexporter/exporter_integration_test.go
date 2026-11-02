// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration_doris

package dorisexporter

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// Run with:
//   go test -v -tags=integration_doris -run TestDorisIntegration ./exporter/dorisexporter/...
//
// Requires a local Doris reachable at:
//   HTTP  (stream load): localhost:8030
//   MySQL (schema ops):  localhost:9030
// with a user that can CREATE DATABASE / DROP DATABASE.

const (
	itHTTPEndpoint  = "http://localhost:8030"
	itMySQLEndpoint = "localhost:9030"
	itUsername      = "root"
	itPassword      = ""
)

// streamLoadEndpoint returns the HTTP endpoint used for stream load in the
// end-to-end test. Stream load through the FE redirects to the BE's own
// address, which may not be resolvable from the test host (e.g. a BE inside
// Kubernetes); point DORIS_IT_STREAM_ENDPOINT at a BE (http://host:8040) then.
func streamLoadEndpoint() string {
	if v := os.Getenv("DORIS_IT_STREAM_ENDPOINT"); v != "" {
		return v
	}
	return itHTTPEndpoint
}

func integrationConfig(db string, createHistoryDays int32) *Config {
	c := createDefaultConfig().(*Config)
	c.ClientConfig.Endpoint = itHTTPEndpoint
	c.MySQLEndpoint = itMySQLEndpoint
	c.Username = itUsername
	c.Password = ""
	c.Database = db
	c.CreateSchema = true
	c.CreateHistoryDays = createHistoryDays
	c.HistoryDays = createHistoryDays
	c.ReplicationNum = 1
	// Setting to 0 makes the progress-reporter goroutine exit immediately
	// (its loop condition is `for interval > 0`). This avoids a pre-existing
	// goroutine leak — shutdown() does not signal the reporter to stop —
	// which would otherwise be reported by goleak.
	c.LogProgressInterval = 0
	_ = c.Validate()
	return c
}

func openRootConn(t *testing.T) *sql.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/mysql", itUsername, itPassword, itMySQLEndpoint)
	conn, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	require.NoError(t, conn.Ping())
	return conn
}

func dropDB(t *testing.T, conn *sql.DB, db string) {
	// FORCE bypasses ROLLUP-in-progress state. A sync MV is built asynchronously
	// and leaves the base table in ROLLUP state briefly, which blocks a normal DROP.
	_, err := conn.ExecContext(t.Context(), "DROP DATABASE IF EXISTS "+db+" FORCE")
	require.NoError(t, err)
}

// waitForMVReady polls until the sync MV (an index on the base table) is in
// FINISHED state. Sync MV creation returns immediately but builds asynchronously.
func waitForMVReady(t *testing.T, conn *sql.DB, db, baseTable, mvName string) {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		rows, err := conn.QueryContext(t.Context(), fmt.Sprintf("SHOW ALTER TABLE ROLLUP FROM `%s` WHERE TableName = '%s'", db, baseTable))
		require.NoError(t, err)
		foundFinished := false
		foundAny := false
		for rows.Next() {
			cols, _ := rows.Columns()
			vals := make([]sql.NullString, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			require.NoError(t, rows.Scan(ptrs...))
			m := map[string]string{}
			for i, c := range cols {
				m[c] = vals[i].String
			}
			if m["RollupIndexName"] == mvName {
				foundAny = true
				if m["State"] == "FINISHED" {
					foundFinished = true
				}
			}
		}
		rows.Close()
		if foundFinished {
			return
		}
		if !foundAny {
			// SHOW ALTER only lists in-flight/recent alters; if the MV finished
			// long ago it may not appear at all, which is also fine.
			if mvVisibleInDesc(t, conn, db, baseTable, mvName) {
				return
			}
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("materialized view %s on %s.%s did not finish within timeout", mvName, db, baseTable)
}

func mvVisibleInDesc(t *testing.T, conn *sql.DB, db, baseTable, mvName string) bool {
	rows, err := conn.QueryContext(t.Context(), fmt.Sprintf("DESC `%s`.`%s` ALL", db, baseTable))
	require.NoError(t, err)
	defer rows.Close()
	cols, _ := rows.Columns()
	idxNameIdx := -1
	for i, c := range cols {
		if c == "IndexName" {
			idxNameIdx = i
			break
		}
	}
	require.NotEqual(t, -1, idxNameIdx)
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		require.NoError(t, rows.Scan(ptrs...))
		if vals[idxNameIdx].String == mvName {
			return true
		}
	}
	return false
}

func countRows(t *testing.T, conn *sql.DB, q string) int {
	rows, err := conn.QueryContext(t.Context(), q)
	require.NoError(t, err, q)
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	require.NoError(t, rows.Err())
	return n
}

func assertTableExists(t *testing.T, conn *sql.DB, db, tbl string) {
	rows, err := conn.QueryContext(t.Context(), fmt.Sprintf("SHOW TABLES FROM `%s` LIKE '%s'", db, tbl))
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next(), "table %s.%s not found", db, tbl)
}

func assertMVExists(t *testing.T, conn *sql.DB, db, baseTable, mvSuffix string) {
	mv := fmt.Sprintf("%s_%s", baseTable, mvSuffix)
	waitForMVReady(t, conn, db, baseTable, mv)
}

func TestDorisIntegrationMetrics(t *testing.T) {
	if os.Getenv("DORIS_INTEGRATION_TEST") != "1" {
		t.Skip("set DORIS_INTEGRATION_TEST=1 to enable")
	}

	db := fmt.Sprintf("otel_it_metrics_%d", time.Now().UnixNano())
	root := openRootConn(t)
	defer root.Close()
	defer dropDB(t, root, db)

	cfg := integrationConfig(db, 3) // expect 3 history + today + 1 future = 5 partitions
	logger := zaptest.NewLogger(t)
	exp := newMetricsExporter(logger, cfg, componenttest.NewNopTelemetrySettings())

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	require.NoError(t, exp.start(ctx, componenttest.NewNopHost()))
	defer func() { _ = exp.shutdown(ctx) }()

	expectedPartitions := cfg.expectedInitialPartitionCount()
	require.Equal(t, 5, expectedPartitions)

	metricsTables := []string{
		cfg.Table.Metrics + "_gauge",
		cfg.Table.Metrics + "_sum",
		cfg.Table.Metrics + "_histogram",
		cfg.Table.Metrics + "_exponential_histogram",
		cfg.Table.Metrics + "_summary",
	}
	for _, tbl := range metricsTables {
		assertTableExists(t, root, db, tbl)
		got := countRows(t, root, fmt.Sprintf("SHOW PARTITIONS FROM `%s`.`%s`", db, tbl))
		require.GreaterOrEqual(t, got, expectedPartitions,
			"table %s should have >= %d partitions, got %d", tbl, expectedPartitions, got)
		assertMVExists(t, root, db, tbl, "services")
	}
}

func TestDorisIntegrationLogs(t *testing.T) {
	if os.Getenv("DORIS_INTEGRATION_TEST") != "1" {
		t.Skip("set DORIS_INTEGRATION_TEST=1 to enable")
	}

	db := fmt.Sprintf("otel_it_logs_%d", time.Now().UnixNano())
	root := openRootConn(t)
	defer root.Close()
	defer dropDB(t, root, db)

	cfg := integrationConfig(db, 2)
	logger := zaptest.NewLogger(t)
	exp := newLogsExporter(logger, cfg, componenttest.NewNopTelemetrySettings())

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	require.NoError(t, exp.start(ctx, componenttest.NewNopHost()))
	defer func() { _ = exp.shutdown(ctx) }()

	expectedPartitions := cfg.expectedInitialPartitionCount()
	require.Equal(t, 4, expectedPartitions)

	assertTableExists(t, root, db, cfg.Table.Logs)
	got := countRows(t, root, fmt.Sprintf("SHOW PARTITIONS FROM `%s`.`%s`", db, cfg.Table.Logs))
	require.GreaterOrEqual(t, got, expectedPartitions)
	assertMVExists(t, root, db, cfg.Table.Logs, "services")
}

func TestDorisIntegrationTraces(t *testing.T) {
	if os.Getenv("DORIS_INTEGRATION_TEST") != "1" {
		t.Skip("set DORIS_INTEGRATION_TEST=1 to enable")
	}

	db := fmt.Sprintf("otel_it_traces_%d", time.Now().UnixNano())
	root := openRootConn(t)
	defer root.Close()
	defer dropDB(t, root, db)

	cfg := integrationConfig(db, 1)
	logger := zaptest.NewLogger(t)
	exp := newTracesExporter(logger, cfg, componenttest.NewNopTelemetrySettings())

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	require.NoError(t, exp.start(ctx, componenttest.NewNopHost()))
	defer func() { _ = exp.shutdown(ctx) }()

	expectedPartitions := cfg.expectedInitialPartitionCount()
	require.Equal(t, 3, expectedPartitions)

	assertTableExists(t, root, db, cfg.Table.Traces)
	assertTableExists(t, root, db, cfg.Table.Traces+"_graph")
	got := countRows(t, root, fmt.Sprintf("SHOW PARTITIONS FROM `%s`.`%s`", db, cfg.Table.Traces))
	require.GreaterOrEqual(t, got, expectedPartitions)
	assertMVExists(t, root, db, cfg.Table.Traces, "summary")
}

// TestWaitForPartitionsReadyTimeout exercises the timeout branch against a
// non-dynamic-partition table (no auto partitions), ensuring the helper fails
// rather than hanging.
func TestDorisIntegrationWaitForPartitionsTimeout(t *testing.T) {
	if os.Getenv("DORIS_INTEGRATION_TEST") != "1" {
		t.Skip("set DORIS_INTEGRATION_TEST=1 to enable")
	}

	db := fmt.Sprintf("otel_it_wait_%d", time.Now().UnixNano())
	root := openRootConn(t)
	defer root.Close()
	defer dropDB(t, root, db)

	_, err := root.ExecContext(t.Context(), "CREATE DATABASE "+db)
	require.NoError(t, err)
	_, err = root.ExecContext(t.Context(), fmt.Sprintf(
		"CREATE TABLE `%s`.`no_part` (id INT) DISTRIBUTED BY HASH(id) BUCKETS 1 PROPERTIES('replication_num'='1')", db,
	))
	require.NoError(t, err)

	// Temporarily shorten timeout so the test finishes quickly. We assert the
	// real helper errors out with a timeout message.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err = waitForPartitionsReady(ctx, root, zap.NewNop(), db, "no_part", 5)
	require.Error(t, err)
}

// TestDorisIntegrationTracesEndToEnd creates the schema through the exporter,
// loads spans, and checks the traces table shape the Grafana Doris app relies
// on: HASH(trace_id) distribution, is_root values, and that the trace-summary
// query is rewritten to the <traces>_summary materialized view.
func TestDorisIntegrationTracesEndToEnd(t *testing.T) {
	if os.Getenv("DORIS_INTEGRATION_TEST") != "1" {
		t.Skip("set DORIS_INTEGRATION_TEST=1 to enable")
	}

	db := fmt.Sprintf("otel_it_e2e_%d", time.Now().UnixNano())
	root := openRootConn(t)
	defer root.Close()
	defer dropDB(t, root, db)

	cfg := integrationConfig(db, 1)
	cfg.ClientConfig.Endpoint = streamLoadEndpoint()
	// no keep-alive: the stream-load connection would otherwise outlive the test and trip goleak
	cfg.ClientConfig.DisableKeepAlives = true
	require.NoError(t, cfg.Validate())
	logger := zaptest.NewLogger(t)
	exp := newTracesExporter(logger, cfg, componenttest.NewNopTelemetrySettings())

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	require.NoError(t, exp.start(ctx, componenttest.NewNopHost()))
	defer func() { _ = exp.shutdown(ctx) }()
	assertMVExists(t, root, db, cfg.Table.Traces, "summary")

	// table shape
	var tbl, createSQL string
	require.NoError(t, root.QueryRowContext(ctx, fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", db, cfg.Table.Traces)).Scan(&tbl, &createSQL))
	require.Contains(t, createSQL, "DISTRIBUTED BY RANDOM")
	require.Contains(t, createSQL, "DUPLICATE KEY(`service_name`, `timestamp`)")
	require.Contains(t, createSQL, "`is_root` tinyint")

	// one root span + child spans
	traces := simpleTraces(3)
	traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetParentSpanID(pcommon.NewSpanIDEmpty())
	require.NoError(t, exp.pushTraceData(ctx, traces))

	var roots, children int
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, root.QueryRowContext(ctx, fmt.Sprintf("SELECT SUM(is_root = 1), SUM(is_root = 0) FROM `%s`.`%s`", db, cfg.Table.Traces)).Scan(&roots, &children))
		if roots+children == 3 {
			break
		}
		time.Sleep(time.Second)
	}
	require.Equal(t, 1, roots)
	require.Equal(t, 2, children)

	// the Grafana trace-list query shape must hit the MV
	q := fmt.Sprintf(`EXPLAIN SELECT trace_id, date_trunc(timestamp,'day') AS day, MIN(timestamp) AS start_time, MAX(end_time) AS end_time,
COUNT(*) AS span_count, SUM(CASE WHEN status_code='STATUS_CODE_ERROR' THEN 1 ELSE 0 END) AS error_count,
MIN(CASE WHEN is_root=1 THEN CONCAT(DATE_FORMAT(timestamp,'%%Y%%m%%d%%H%%i%%s%%f'),'|',service_name,'|',span_name) END) AS first_root,
MAX(duration) AS max_span_duration
FROM `+"`%s`.`%s`"+` GROUP BY trace_id, date_trunc(timestamp,'day') HAVING MIN(timestamp) >= '2000-01-01' ORDER BY start_time DESC LIMIT 50`, db, cfg.Table.Traces)
	rows, err := root.QueryContext(ctx, q)
	require.NoError(t, err)
	defer rows.Close()
	plan := ""
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		plan += line + "\n"
	}
	require.Contains(t, plan, cfg.Table.Traces+"_summary", "trace summary query was not rewritten to the MV:\n%s", plan)
}
