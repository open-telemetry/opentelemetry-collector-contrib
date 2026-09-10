// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"path/filepath"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

var procedureCacheValue = map[string]int64{
	"EXECUTIONS":           200413,
	"CPU_TIME":             29821063,
	"ELAPSED_TIME":         38172810,
	"BUFFER_GETS":          3808197,
	"DISK_READS":           12,
	"DIRECT_WRITES":        6,
	"ROWS_PROCESSED":       200413,
	"PHYSICAL_READ_BYTES":  300,
	"PHYSICAL_WRITE_BYTES": 12,
}

// procedureFixtureCacheKey matches the single row in oracleProcedureMetricsData.txt.
const procedureFixtureCacheKey = "98765:ORCLPDB1:ORCLPDB1"

// elapsedDeltaSeconds is the procedureCacheValue-to-procedureRow delta: (99172810 - 38172810) microseconds.
const elapsedDeltaSeconds = 61.0

func newProcedureMetricsScraper(t *testing.T, dbclientFn clientProviderFunc, seed map[string]int64) *oracleScraper {
	t.Helper()
	return newProcedureMetricsScraperWithSeeds(t, dbclientFn, map[string]map[string]int64{procedureFixtureCacheKey: seed})
}

func newProcedureMetricsScraperWithSeeds(t *testing.T, dbclientFn clientProviderFunc, seeds map[string]map[string]int64) *oracleScraper {
	t.Helper()

	logsCfg := metadata.DefaultLogsBuilderConfig()
	logsCfg.ResourceAttributes.HostName.Enabled = true
	logsCfg.Events.DbServerTopProcedure.Enabled = true
	metricsCfg := metadata.NewDefaultMetricsBuilderConfig()

	lruCache, err := lru.New[string, map[string]int64](500)
	require.NoError(t, err)
	for key, seed := range seeds {
		if seed != nil {
			lruCache.Add(key, seed)
		}
	}

	return &oracleScraper{
		logger: zap.NewNop(),
		mb:     metadata.NewMetricsBuilder(metricsCfg, receivertest.NewNopSettings(metadata.Type)),
		lb:     metadata.NewLogsBuilder(logsCfg, receivertest.NewNopSettings(metadata.Type)),
		dbProviderFunc: func() (*sql.DB, error) {
			return nil, nil
		},
		clientProviderFunc:   dbclientFn,
		id:                   component.ID{},
		metricsBuilderConfig: metricsCfg,
		logsBuilderConfig:    logsCfg,
		procedureMetricCache: lruCache,
		procedureMetricsCfg:  ProcedureMetrics{MaxProcedureSampleCount: 1000, TopProcedureCount: 250},
		instanceName:         "oraclehost:1521/ORCL",
		hostName:             "oraclehost:1521",
		obfuscator:           newObfuscator(),
		serviceInstanceID:    getInstanceID("oraclehost:1521/ORCL", zap.NewNop()),
	}
}

func procedureMetricsDbClientFn(t *testing.T) clientProviderFunc {
	t.Helper()
	return func(*sql.DB, string, *zap.Logger) dbClient {
		var rows []metricRow
		require.NoError(t, json.Unmarshal(readFile("oracleProcedureMetricsData.txt"), &rows))
		return &fakeDbClient{Responses: [][]metricRow{rows}}
	}
}

func procedureRow(service string, overrides map[string]string) metricRow {
	row := metricRow{
		"SCHEMA_NAME": "ADMIN", "PROCEDURE_NAME": "ADMIN.MY_PROCEDURE",
		"PROCEDURE_TYPE": "PROCEDURE", "PROGRAM_ID": "98765",
		"DB_NAMESPACE": "ORCLPDB1", "SERVICE": service,
		"EXECUTIONS": "300413", "CPU_TIME": "39736887", "ELAPSED_TIME": "99172810",
		"BUFFER_GETS": "3997614", "DISK_READS": "15", "DIRECT_WRITES": "10",
		"ROWS_PROCESSED": "399856", "PHYSICAL_READ_BYTES": "400", "PHYSICAL_WRITE_BYTES": "18",
		"FIRST_LOAD_TIME": "2025-12-31/23:00:00", "LAST_ACTIVE_TIME": "2026-01-01T12:00:00Z",
	}
	maps.Copy(row, overrides)
	return row
}

func staticProcedureRowsFn(rows []metricRow) clientProviderFunc {
	return func(*sql.DB, string, *zap.Logger) dbClient {
		return &fakeDbClient{Responses: [][]metricRow{rows}}
	}
}

// procedureRecordAttrs returns the attributes of the single emitted log record.
func procedureRecordAttrs(t *testing.T, scrpr *oracleScraper) map[string]any {
	t.Helper()

	logs, err := scrpr.scrapeLogs(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, logs.ResourceLogs().Len())
	records := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 1, records.Len())
	return records.At(0).Attributes().AsRaw()
}

// DBA_PROCEDURES only exposes the connected container, so from a CDB root the inner join drops every
// PDB-owned procedure.
func TestBuildProcedureMetricsSQL(t *testing.T) {
	tests := []struct {
		name          string
		useCDB        bool
		wantView      string
		wantConIDJoin bool
	}{
		{name: "CDB root uses CDB_PROCEDURES", useCDB: true, wantView: "CDB_PROCEDURES", wantConIDJoin: true},
		{name: "non-root uses DBA_PROCEDURES", useCDB: false, wantView: "DBA_PROCEDURES", wantConIDJoin: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scrpr := oracleScraper{useCDBDictionaryViews: test.useCDB}
			got := scrpr.buildProcedureMetricsSQL()

			assert.Contains(t, got, test.wantView)
			if test.wantConIDJoin {
				assert.Contains(t, got, "P.CON_ID    = S.CON_ID",
					"CDB variant must match on CON_ID; object ids are only unique within a container")
				assert.NotContains(t, got, "FROM   DBA_PROCEDURES")
			} else {
				assert.NotContains(t, got, "CDB_PROCEDURES")
			}

			// Binds are passed positionally, so both variants must keep the same contract.
			assert.Contains(t, got, "NUMTODSINTERVAL(:1, 'SECOND')")
			assert.Contains(t, got, "FETCH FIRST :2 ROWS ONLY")
		})
	}
}

// An ORDER BY on a cumulative column would let the database pick rows by lifetime totals, so a procedure
// hot only in this interval could never reach the collector's delta ranking.
func TestProcedureMetricsSQLDoesNotRankInDatabase(t *testing.T) {
	for _, useCDB := range []bool{true, false} {
		scrpr := oracleScraper{useCDBDictionaryViews: useCDB}

		assert.NotContains(t, scrpr.buildProcedureMetricsSQL(), "ORDER BY",
			"ranking must happen in the collector over deltas, not in SQL over cumulative totals")
	}
}

func TestScraper_ScrapeProcedureMetricsLogs(t *testing.T) {
	tests := []struct {
		name       string
		dbclientFn clientProviderFunc
		errWanted  string
		// noRecordsWanted expects a successful scrape that emits nothing.
		noRecordsWanted bool
	}{
		{
			name:       "valid collection",
			dbclientFn: procedureMetricsDbClientFn(t),
		}, {
			// Nothing was active in the lookback window: a normal condition, not a scrape error.
			name: "No metrics collected",
			dbclientFn: func(*sql.DB, string, *zap.Logger) dbClient {
				return &fakeDbClient{Responses: [][]metricRow{nil}}
			},
			noRecordsWanted: true,
		}, {
			name: "Error on collecting metrics",
			dbclientFn: func(*sql.DB, string, *zap.Logger) dbClient {
				return &fakeDbClient{
					Responses: [][]metricRow{nil},
					Err:       errors.New("Mock error"),
				}
			},
			errWanted: "error executing oracleProcedureMetricsSQL: Mock error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scrpr := newProcedureMetricsScraper(t, test.dbclientFn, procedureCacheValue)

			require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, scrpr.shutdown(t.Context()))
			}()

			assert.True(t, scrpr.lastProcedureMetricsTimestamp.IsZero(), "No value exists on lastProcedureMetricsTimestamp before any collection.")

			logs, err := scrpr.scrapeLogs(t.Context())

			if test.errWanted != "" {
				require.EqualError(t, err, test.errWanted)
				return
			}

			if test.noRecordsWanted {
				require.NoError(t, err, "an empty result set should not be reported as a scrape error")
				assert.Equal(t, 0, logs.ResourceLogs().Len(), "no log records should be emitted when no procedures were active")
				return
			}

			expectedProcedureMetricsFile := filepath.Join("testdata", "expectedProcedureMetricsFile.yaml")
			// Uncomment line below to re-generate expected logs.
			// golden.WriteLogs(t, expectedProcedureMetricsFile, logs)
			expectedLogs, readErr := golden.ReadLogs(expectedProcedureMetricsFile)
			require.NoError(t, readErr)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreTimestamp()))
			assert.Equal(t, "db.server.top_procedure", logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).EventName())
			assert.False(t, scrpr.lastProcedureMetricsTimestamp.IsZero(), "lastProcedureMetricsTimestamp hasn't set after a successful collection.")
		})
	}
}

// The first scrape has no prior value to diff against, so it must only seed the cache.
func TestProcedureMetricsFirstScrapeSeedsCacheOnly(t *testing.T) {
	scrpr := newProcedureMetricsScraper(t, procedureMetricsDbClientFn(t), nil)

	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()

	logs, err := scrpr.scrapeLogs(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 0, logs.ResourceLogs().Len(), "first scrape should only seed the cache")
	assert.Equal(t, 1, scrpr.procedureMetricCache.Len(), "first scrape should have cached the procedure row")
}

// One procedure driven through two services returns both rows in one result set. A key omitting SERVICE
// collides: the second row diffs against the first's absolute counters, not against the previous scrape.
func TestProcedureMetricsCacheKeyIncludesService(t *testing.T) {
	// Both services have advanced by an identical, known amount since the seed.
	seeds := map[string]map[string]int64{
		"98765:ORCLPDB1:OLTP":  maps.Clone(procedureCacheValue),
		"98765:ORCLPDB1:BATCH": maps.Clone(procedureCacheValue),
	}
	rows := []metricRow{procedureRow("OLTP", nil), procedureRow("BATCH", nil)}

	scrpr := newProcedureMetricsScraperWithSeeds(t, staticProcedureRowsFn(rows), seeds)

	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()

	logs, err := scrpr.scrapeLogs(t.Context())
	require.NoError(t, err)

	require.Equal(t, 1, logs.ResourceLogs().Len())
	records := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 2, records.Len(), "both services must be reported; a colliding key would drop one as a false purge")

	// Identical inputs must produce identical deltas, not one correct row and one cross-service difference.
	seenServices := map[string]bool{}
	for i := 0; i < records.Len(); i++ {
		attrs := records.At(i).Attributes().AsRaw()
		service, ok := attrs["oracle.db.service"].(string)
		require.True(t, ok)
		seenServices[service] = true

		assert.InDelta(t, elapsedDeltaSeconds, attrs["oracledb.elapsed_time"], 0.001,
			"service %s should report its own delta against the previous scrape", service)
	}
	assert.Equal(t, map[string]bool{"OLTP": true, "BATCH": true}, seenServices)

	// Each service must retain its own cache entry for the next scrape.
	assert.Equal(t, 2, scrpr.procedureMetricCache.Len())
}

// A fresh child cursor starts at 1 and pulls MIN(EXECUTIONS) down, producing a negative delta that is not a
// purge. Plan changes and bind-sensitive cursors make this routine on the busiest procedures.
func TestProcedureMetricsNewChildCursorKeepsRow(t *testing.T) {
	// MIN(EXECUTIONS) dropped below the cached value while the SUM()-based counters kept climbing.
	rows := []metricRow{procedureRow("ORCLPDB1", map[string]string{"EXECUTIONS": "1"})}

	scrpr := newProcedureMetricsScraper(t, staticProcedureRowsFn(rows), maps.Clone(procedureCacheValue))
	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()

	attrs := procedureRecordAttrs(t, scrpr)

	assert.Equal(t, int64(0), attrs["oracledb.procedure_execution_count"],
		"an untrustworthy execution delta is clamped to 0, not emitted as negative")
	assert.InDelta(t, elapsedDeltaSeconds, attrs["oracledb.elapsed_time"], 0.001,
		"resource counters must still be reported")
}

// MIN(EXECUTIONS) can track a statement in a branch that did not run, so gating emission on it would drop a
// procedure that consumed CPU and elapsed time.
func TestProcedureMetricsEmittedWhenExecutionCountStalls(t *testing.T) {
	seed := maps.Clone(procedureCacheValue)
	seed["EXECUTIONS"] = 300413 // unchanged from the row below: zero execution delta

	scrpr := newProcedureMetricsScraper(t, staticProcedureRowsFn([]metricRow{procedureRow("ORCLPDB1", nil)}), seed)
	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()

	attrs := procedureRecordAttrs(t, scrpr)

	assert.Equal(t, int64(0), attrs["oracledb.procedure_execution_count"])
	assert.InDelta(t, elapsedDeltaSeconds, attrs["oracledb.elapsed_time"], 0.001,
		"a stalled execution count must not drop a procedure whose CPU and elapsed time moved")
}

// A procedure whose resource counters did not move must not be emitted as an empty event.
func TestProcedureMetricsDiscardedWhenIdle(t *testing.T) {
	// Seed with exactly the values the fixture row reports, so every delta is zero.
	idle := map[string]int64{
		"EXECUTIONS": 300413, "CPU_TIME": 39736887, "ELAPSED_TIME": 99172810,
		"BUFFER_GETS": 3997614, "DISK_READS": 15, "DIRECT_WRITES": 10,
		"ROWS_PROCESSED": 399856, "PHYSICAL_READ_BYTES": 400, "PHYSICAL_WRITE_BYTES": 18,
	}

	scrpr := newProcedureMetricsScraper(t, procedureMetricsDbClientFn(t), idle)
	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()

	logs, err := scrpr.scrapeLogs(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 0, logs.ResourceLogs().Len(), "an idle procedure should not be emitted")
}

// A negative delta on a SUM()-based counter really does mean a shared-pool purge, so the row is discarded.
func TestProcedureMetricsDiscardedOnPossiblePurge(t *testing.T) {
	// Seed with a resource counter HIGHER than the row reports.
	purged := maps.Clone(procedureCacheValue)
	purged["BUFFER_GETS"] = 999999999

	scrpr := newProcedureMetricsScraper(t, procedureMetricsDbClientFn(t), purged)
	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()

	logs, err := scrpr.scrapeLogs(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 0, logs.ResourceLogs().Len(),
		"rows with a negative resource delta should be discarded as a possible cursor purge")
}

// The smaller-lifetime, larger-delta procedure must win: the case a database-side ORDER BY on cumulative
// elapsed time could never surface.
func TestProcedureMetricsRanksByDeltaNotCumulative(t *testing.T) {
	seeds := map[string]map[string]int64{
		// Huge lifetime total, barely moved this interval.
		"1:ORCLPDB1:OLTP": {"EXECUTIONS": 1000, "CPU_TIME": 900000000, "ELAPSED_TIME": 900000000},
		// Modest lifetime total, hot right now.
		"2:ORCLPDB1:OLTP": {"EXECUTIONS": 10, "CPU_TIME": 1000, "ELAPSED_TIME": 1000},
	}
	rows := []metricRow{
		procedureRow("OLTP", map[string]string{
			"PROGRAM_ID": "1", "PROCEDURE_NAME": "ADMIN.LIFETIME_HEAVY",
			"EXECUTIONS": "1001", "CPU_TIME": "900001000", "ELAPSED_TIME": "900001000",
		}),
		procedureRow("OLTP", map[string]string{
			"PROGRAM_ID": "2", "PROCEDURE_NAME": "ADMIN.HOT_NOW",
			"EXECUTIONS": "20", "CPU_TIME": "50001000", "ELAPSED_TIME": "50001000",
		}),
	}

	scrpr := newProcedureMetricsScraperWithSeeds(t, staticProcedureRowsFn(rows), seeds)
	scrpr.procedureMetricsCfg.TopProcedureCount = 1 // only the top-ranked row survives
	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()

	attrs := procedureRecordAttrs(t, scrpr)

	assert.Equal(t, "ADMIN.HOT_NOW", attrs["oracledb.procedure_name"],
		"the procedure with the largest delta must be reported, not the largest lifetime total")
}

func TestScrapesProcedureMetricsLogsOnlyWhenIntervalHasElapsed(t *testing.T) {
	scrpr := newProcedureMetricsScraper(t, procedureMetricsDbClientFn(t), procedureCacheValue)
	scrpr.procedureMetricsCfg.CollectionInterval = 1 * time.Minute

	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()

	assert.True(t, scrpr.lastProcedureMetricsTimestamp.IsZero(), "No value should be set for lastProcedureMetricsTimestamp before a successful collection")
	logsCol1, _ := scrpr.scrapeLogs(t.Context())
	assert.Equal(t, 1, logsCol1.ResourceLogs().At(0).ScopeLogs().Len(), "Collection should run when lastProcedureMetricsTimestamp is not available")
	assert.False(t, scrpr.lastProcedureMetricsTimestamp.IsZero(), "A value should be set for lastProcedureMetricsTimestamp after a successful collection")

	// calculateLookbackSeconds adds vsqlRefreshLag, so the gate opens 10s early: 30s elapsed
	// reports 40s against a 60s interval and must still skip.
	scrpr.lastProcedureMetricsTimestamp = scrpr.lastProcedureMetricsTimestamp.Add(-30 * time.Second)
	skippedFrom := scrpr.lastProcedureMetricsTimestamp
	logsCol2, err := scrpr.scrapeLogs(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 0, logsCol2.ResourceLogs().Len(),
		"procedure_metrics should not be collected until %s elapsed", scrpr.procedureMetricsCfg.CollectionInterval)
	// Emitting nothing is also what a collection with zero deltas looks like, so assert the
	// timestamp did not move: only a collection that actually ran advances it.
	assert.Equal(t, skippedFrom, scrpr.lastProcedureMetricsTimestamp,
		"a skipped scrape must not advance lastProcedureMetricsTimestamp")
}

// A discarded interval is otherwise indistinguishable from "the procedure did not run", and one child
// cursor aging out of the shared pool discards the whole procedure, so the count must be observable.
func TestProcedureMetricsLogsDiscardedCount(t *testing.T) {
	purged := maps.Clone(procedureCacheValue)
	purged["BUFFER_GETS"] = 999999999 // negative resource delta -> possiblePurge

	core, observed := observer.New(zapcore.DebugLevel)
	scrpr := newProcedureMetricsScraper(t, procedureMetricsDbClientFn(t), purged)
	scrpr.logger = zap.New(core)
	require.NoError(t, scrpr.start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, scrpr.shutdown(t.Context()))
	}()

	_, err := scrpr.scrapeLogs(t.Context())
	require.NoError(t, err)

	entries := observed.FilterMessage("Procedure cache hits").All()
	require.Len(t, entries, 1, "a discarded interval must be reported once per scrape")
	assert.Equal(t, int64(1), entries[0].ContextMap()["discarded-hit-count"])
	assert.Equal(t, int64(0), entries[0].ContextMap()["hit-count"])
}
