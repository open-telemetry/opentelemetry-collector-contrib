// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

func TestTopQueryCounterHistory(t *testing.T) {
	type observation struct {
		name       string
		queryID    string
		total      string
		overrides  map[string]string
		want       map[string]any
		restart    bool
		reset      int64
		queryFails bool
		resetFails bool
		resetAfter int64
		afterFails bool
	}
	for _, tt := range []struct {
		name      string
		cacheSize int
		steps     []observation
	}{
		{
			name:      "startup and resets",
			cacheSize: 20,
			steps: []observation{
				{name: "startup seeds lifetime totals", total: "10000"},
				{name: "unchanged counters", total: "10000"},
				{
					name: "activity after startup", total: "10000",
					overrides: map[string]string{callsColumnName: "10003", rowsColumnName: "10007", totalExecTimeColumnName: "12500", totalPlanTimeColumnName: "10500"},
					want:      map[string]any{callsColumnName: int64(3), rowsColumnName: int64(7), totalExecTimeColumnName: 2.5, totalPlanTimeColumnName: 0.5},
				},
				{
					// Only calls decreases. All other counters must report their current totals.
					name: "one counter decrease resets the statement", total: "10000",
					overrides: map[string]string{callsColumnName: "2", rowsColumnName: "10007", totalExecTimeColumnName: "15000", totalPlanTimeColumnName: "11000"},
					want: map[string]any{
						callsColumnName: int64(2), rowsColumnName: int64(10007),
						sharedBlksDirtiedColumnName: int64(10000), sharedBlksHitColumnName: int64(10000),
						sharedBlksReadColumnName: int64(10000), sharedBlksWrittenColumnName: int64(10000),
						tempBlksReadColumnName: int64(10000), tempBlksWrittenColumnName: int64(10000),
						totalExecTimeColumnName: 15.0, totalPlanTimeColumnName: 11.0,
					},
				},
				{
					name: "activity after nonzero reset", total: "10000",
					overrides: map[string]string{callsColumnName: "5", rowsColumnName: "10010", totalExecTimeColumnName: "16000", totalPlanTimeColumnName: "12000"},
					want:      map[string]any{callsColumnName: int64(3), rowsColumnName: int64(3), totalExecTimeColumnName: 1.0, totalPlanTimeColumnName: 1.0},
				},
				{name: "reset to zero", total: "0"},
				{
					name: "recovery below old totals", total: "0",
					overrides: map[string]string{callsColumnName: "3", rowsColumnName: "7", totalExecTimeColumnName: "2500", totalPlanTimeColumnName: "500"},
					want:      map[string]any{callsColumnName: int64(3), rowsColumnName: int64(7), totalExecTimeColumnName: 2.5, totalPlanTimeColumnName: 0.5},
				},
				{
					name: "unchanged after recovery", total: "0",
					overrides: map[string]string{callsColumnName: "3", rowsColumnName: "7", totalExecTimeColumnName: "2500", totalPlanTimeColumnName: "500"},
				},
				{name: "receiver restart seeds a new baseline", total: "20000", restart: true},
				{
					name: "activity after restart", total: "20000",
					overrides: map[string]string{callsColumnName: "20001", totalExecTimeColumnName: "21000"},
					want:      map[string]any{callsColumnName: int64(1), totalExecTimeColumnName: 1.0},
				},
			},
		},
		{
			name:      "eviction keeps counters together",
			cacheSize: 1,
			steps: []observation{
				{name: "seed first statement", queryID: "1", total: "10000"},
				{
					name: "activity fits in one cache entry", queryID: "1", total: "10000",
					overrides: map[string]string{callsColumnName: "10003", totalExecTimeColumnName: "12500"},
					want:      map[string]any{callsColumnName: int64(3), totalExecTimeColumnName: 2.5},
				},
				{name: "second statement evicts first", queryID: "2", total: "50000"},
				{name: "returning statement seeds all counters", queryID: "1", total: "20000"},
				{
					name: "activity after eviction", queryID: "1", total: "20000",
					overrides: map[string]string{callsColumnName: "20002", totalExecTimeColumnName: "20500"},
					want:      map[string]any{callsColumnName: int64(2), totalExecTimeColumnName: 0.5},
				},
			},
		},
		{
			name:      "global reset history",
			cacheSize: 20,
			steps: []observation{
				{
					name: "seed A", queryID: "1", total: "0", reset: 1,
					overrides: map[string]string{callsColumnName: "100", totalExecTimeColumnName: "1000"},
				},
				{
					name: "seed B", queryID: "2", total: "0", reset: 1,
					overrides: map[string]string{callsColumnName: "100", totalExecTimeColumnName: "1000"},
				},
				{
					name: "reset followed by growth past old totals", queryID: "1", total: "0", reset: 2,
					overrides: map[string]string{callsColumnName: "150", totalExecTimeColumnName: "2000"},
					want:      map[string]any{callsColumnName: int64(150), totalExecTimeColumnName: 2.0},
				},
				{
					name: "B returns after missing reset collection", queryID: "2", total: "0", reset: 2,
					overrides: map[string]string{callsColumnName: "200", totalExecTimeColumnName: "3000"},
					want:      map[string]any{callsColumnName: int64(200), totalExecTimeColumnName: 3.0},
				},
				{
					name: "unchanged after reset", queryID: "2", total: "0", reset: 2,
					overrides: map[string]string{callsColumnName: "200", totalExecTimeColumnName: "3000"},
				},
				{name: "statement fetch fails after another reset", reset: 3, queryFails: true},
				{
					name: "reset survives failed statement fetch", queryID: "1", total: "0", reset: 3,
					overrides: map[string]string{callsColumnName: "400", totalExecTimeColumnName: "4000"},
					want:      map[string]any{callsColumnName: int64(400), totalExecTimeColumnName: 4.0},
				},
				{name: "reset timestamp read fails", reset: 4, resetFails: true},
				{
					name: "reset survives failed timestamp read", queryID: "2", total: "0", reset: 4,
					overrides: map[string]string{callsColumnName: "500", totalExecTimeColumnName: "5000"},
					want:      map[string]any{callsColumnName: int64(500), totalExecTimeColumnName: 5.0},
				},
				{
					name: "subsequent activity uses new baseline", queryID: "2", total: "0", reset: 4,
					overrides: map[string]string{callsColumnName: "502", totalExecTimeColumnName: "5500"},
					want:      map[string]any{callsColumnName: int64(2), totalExecTimeColumnName: 0.5},
				},
				{
					name: "reset during statement fetch", queryID: "1", total: "0", reset: 5, resetAfter: 6,
					overrides: map[string]string{callsColumnName: "700", totalExecTimeColumnName: "7000"},
				},
				{
					name: "next collection observes the completed reset", queryID: "1", total: "0", reset: 6,
					overrides: map[string]string{callsColumnName: "800", totalExecTimeColumnName: "8000"},
					want:      map[string]any{callsColumnName: int64(800), totalExecTimeColumnName: 8.0},
				},
				{
					name: "reset verification read fails", queryID: "2", total: "0", reset: 7, afterFails: true,
					overrides: map[string]string{callsColumnName: "900", totalExecTimeColumnName: "9000"},
				},
				{
					name: "next collection preserves reset after failed verification", queryID: "2", total: "0", reset: 7,
					overrides: map[string]string{callsColumnName: "1000", totalExecTimeColumnName: "10000"},
					want:      map[string]any{callsColumnName: int64(1000), totalExecTimeColumnName: 10.0},
				},
				{
					name: "new statement after reset still seeds a baseline", queryID: "3", total: "0", reset: 7,
					overrides: map[string]string{callsColumnName: "1000", totalExecTimeColumnName: "10000"},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, db.Close()) })
			newScraper := func() *postgreSQLScraper {
				scraper, err := newTopQueryScraper(receivertest.NewNopSettings(metadata.Type), cfg, mockSimpleClientFactory{db: db})
				require.NoError(t, err)
				scraper.cache = newCache(tt.cacheSize)
				return scraper
			}
			scraper := newScraper()
			for _, step := range tt.steps {
				t.Run(step.name, func(t *testing.T) {
					if step.restart {
						scraper = newScraper()
					}
					values := map[string]any{
						"datname": "postgres", "userid": "101", "rolname": "otel",
						"query": "SELECT 1", queryidColumnName: step.queryID,
					}
					for _, column := range topQueryColumns {
						if _, exists := values[column]; !exists {
							values[column] = step.total
						}
					}
					for column, value := range step.overrides {
						values[column] = value
					}
					if step.reset == 0 {
						mock.ExpectQuery(regexp.QuoteMeta(statStatementsResetAvailableQuery)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
					}
					if step.reset != 0 {
						mock.ExpectQuery(regexp.QuoteMeta(statStatementsResetAvailableQuery)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
						resetQuery := mock.ExpectQuery("SELECT stats_reset FROM pg_stat_statements_info")
						if step.resetFails {
							resetQuery.WillReturnError(errors.New("reset timestamp read failed"))
						} else {
							resetQuery.WillReturnRows(sqlmock.NewRows([]string{"stats_reset"}).AddRow(time.Unix(step.reset, 0)))
						}
					}
					if !step.resetFails {
						query := mock.ExpectQuery("LIMIT")
						if step.queryFails {
							query.WillReturnError(errors.New("statement fetch failed"))
						} else {
							query.WillReturnRows(newSQLMockRows(topQueryColumns, values))
							if step.reset != 0 {
								afterQuery := mock.ExpectQuery("SELECT stats_reset FROM pg_stat_statements_info")
								if step.afterFails {
									afterQuery.WillReturnError(errors.New("reset verification failed"))
								} else {
									after := step.reset
									if step.resetAfter != 0 {
										after = step.resetAfter
									}
									afterQuery.WillReturnRows(sqlmock.NewRows([]string{"stats_reset"}).AddRow(time.Unix(after, 0)))
								}
							}
						}
					}
					logs, err := scraper.scrapeTopQuery(t.Context(), 10, 10, 0, 0)
					require.NoError(t, err)
					require.NoError(t, mock.ExpectationsWereMet())
					if step.want == nil {
						require.Zero(t, logs.LogRecordCount())
						return
					}
					require.Equal(t, 1, logs.LogRecordCount())
					record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
					assert.Equal(t, "db.server.top_query", record.EventName())
					attrs := record.Attributes().AsRaw()
					assert.Equal(t, step.queryID, attrs[dbAttributePrefix+queryidColumnName])
					for _, column := range topQueryColumns {
						var zero any = int64(0)
						switch column {
						case totalExecTimeColumnName, totalPlanTimeColumnName:
							zero = 0.0
						case "datname", "userid", "rolname", "query", queryidColumnName:
							continue
						}
						want := zero
						if value, exists := step.want[column]; exists {
							want = value
						}
						assert.Equal(t, want, attrs[dbAttributePrefix+column], column)
					}
				})
			}
			mock.ExpectClose()
		})
	}
}

func TestTopQueryCacheRetainsFetchedStatements(t *testing.T) {
	columns := []string{
		callsColumnName, "datname", sharedBlksDirtiedColumnName, sharedBlksHitColumnName,
		sharedBlksReadColumnName, sharedBlksWrittenColumnName, tempBlksReadColumnName,
		tempBlksWrittenColumnName, "query", queryidColumnName, "userid", "rolname", rowsColumnName,
		totalExecTimeColumnName, totalPlanTimeColumnName,
	}
	cfg := createDefaultConfig().(*Config)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.MaxRowsPerQuery = 500
	cfg.TopQueryCollection.TopNQuery = 50
	cfg.TopQueryCollection.MaxExplainEachInterval = 0

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	scraper, err := newTopQueryScraper(receivertest.NewNopSettings(metadata.Type), cfg, mockSimpleClientFactory{db: db})
	require.NoError(t, err)

	ids := make([]int, 500)
	for i := range ids {
		ids[i] = i
	}
	reversed := slices.Clone(ids)
	slices.Reverse(reversed)
	changed := slices.Clone(ids)
	for i := range changed[:250] {
		changed[i] = 500 + i
	}

	for _, tt := range []struct {
		name          string
		ids           []int
		increment     bool
		expectedCount int
	}{
		{name: "first scrape", ids: ids},
		{name: "unchanged counters", ids: ids},
		{name: "reordered statements", ids: reversed},
		{name: "changed statement membership", ids: changed},
		{name: "unchanged after membership change", ids: changed},
		{name: "one active statement", ids: changed, increment: true, expectedCount: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := sqlmock.NewRows(columns)
			for _, id := range tt.ids {
				values := map[string]driver.Value{
					"datname":         "postgres",
					"query":           "SELECT 1",
					"rolname":         "otel",
					queryidColumnName: fmt.Sprint(id),
				}
				for _, column := range columns {
					if _, exists := values[column]; !exists {
						values[column] = "10000"
					}
				}
				if tt.increment && id == 499 {
					values[callsColumnName] = "10003"
					values[rowsColumnName] = "10007"
					values[sharedBlksHitColumnName] = "10009"
					values[totalExecTimeColumnName] = "12500"
					values[totalPlanTimeColumnName] = "10500"
				}
				row := make([]driver.Value, len(columns))
				for i, column := range columns {
					row[i] = values[column]
				}
				rows.AddRow(row...)
			}
			mock.ExpectQuery(regexp.QuoteMeta(statStatementsResetAvailableQuery)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			mock.ExpectQuery("LIMIT 500").WillReturnRows(rows)
			logs, scrapeErr := scraper.scrapeTopQuery(t.Context(), cfg.TopQueryCollection.MaxRowsPerQuery, cfg.TopQueryCollection.TopNQuery, cfg.TopQueryCollection.MaxExplainEachInterval, 0)
			require.NoError(t, scrapeErr)
			require.NoError(t, mock.ExpectationsWereMet())
			require.Equal(t, tt.expectedCount, logs.LogRecordCount())
			if tt.increment {
				attrs := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw()
				assert.Equal(t, "499", attrs[dbAttributePrefix+queryidColumnName])
				assert.Equal(t, int64(3), attrs[dbAttributePrefix+callsColumnName])
				assert.Equal(t, int64(7), attrs[dbAttributePrefix+rowsColumnName])
				assert.Equal(t, int64(9), attrs[dbAttributePrefix+sharedBlksHitColumnName])
				assert.Equal(t, 2.5, attrs[dbAttributePrefix+totalExecTimeColumnName])
				assert.Equal(t, 0.5, attrs[dbAttributePrefix+totalPlanTimeColumnName])
				assert.Equal(t, int64(0), attrs[dbAttributePrefix+sharedBlksReadColumnName])
			}
		})
	}
	mock.ExpectClose()
}

func TestTopQueryCacheSeparatesDatabaseAndRole(t *testing.T) {
	columns := []string{
		callsColumnName, "datname", sharedBlksDirtiedColumnName, sharedBlksHitColumnName,
		sharedBlksReadColumnName, sharedBlksWrittenColumnName, tempBlksReadColumnName,
		tempBlksWrittenColumnName, "query", queryidColumnName, "userid", "rolname", rowsColumnName,
		totalExecTimeColumnName, totalPlanTimeColumnName,
	}
	cfg := createDefaultConfig().(*Config)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.MaxRowsPerQuery = 3
	cfg.TopQueryCollection.TopNQuery = 3
	cfg.TopQueryCollection.MaxExplainEachInterval = 0

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	scraper, err := newTopQueryScraper(receivertest.NewNopSettings(metadata.Type), cfg, mockSimpleClientFactory{db: db})
	require.NoError(t, err)

	for scrape := range 2 {
		rows := sqlmock.NewRows(columns)
		for _, identity := range []struct {
			database string
			role     string
			userid   string
		}{
			// app is the same role cluster-wide, so it carries the same userid in both
			// databases: database, not userid, is what disambiguates these two rows.
			{database: "db_a", role: "app", userid: "101"},
			{database: "db_b", role: "app", userid: "101"},
			{database: "db_a", role: "reporting", userid: "102"},
		} {
			values := map[string]driver.Value{
				"datname":         identity.database,
				"query":           "SELECT count(*) FROM pg_class",
				queryidColumnName: "42",
				"userid":          identity.userid,
				"rolname":         identity.role,
			}
			for _, column := range columns {
				if _, exists := values[column]; !exists {
					values[column] = fmt.Sprint(10000 + scrape*1000)
				}
			}
			row := make([]driver.Value, len(columns))
			for i, column := range columns {
				row[i] = values[column]
			}
			rows.AddRow(row...)
		}

		mock.ExpectQuery(regexp.QuoteMeta(statStatementsResetAvailableQuery)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		mock.ExpectQuery("LIMIT 3").WillReturnRows(rows)
		logs, err := scraper.scrapeTopQuery(t.Context(), cfg.TopQueryCollection.MaxRowsPerQuery, cfg.TopQueryCollection.TopNQuery, cfg.TopQueryCollection.MaxExplainEachInterval, 0)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
		if scrape == 0 {
			require.Zero(t, logs.LogRecordCount())
		} else {
			require.Equal(t, 3, logs.LogRecordCount())
			for _, record := range logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().All() {
				assert.Equal(t, int64(1000), record.Attributes().AsRaw()[dbAttributePrefix+callsColumnName])
			}
		}
	}
	mock.ExpectClose()
}

// TestTopQueryCacheSeparatesDroppedRoles guards the counter-cache key against
// two dropped roles (empty rolname) colliding and sharing one delta.
func TestTopQueryCacheSeparatesDroppedRoles(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.MaxRowsPerQuery = 3
	cfg.TopQueryCollection.TopNQuery = 3
	cfg.TopQueryCollection.MaxExplainEachInterval = 0

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	scraper, err := newTopQueryScraper(receivertest.NewNopSettings(metadata.Type), cfg, mockSimpleClientFactory{db: db})
	require.NoError(t, err)

	buildRows := func(callsA, callsB, totalExecTimeA, totalExecTimeB string) *sqlmock.Rows {
		rows := sqlmock.NewRows(topQueryColumns)
		for _, r := range []struct {
			userid        string
			calls         string
			totalExecTime string
		}{
			{userid: "16415", calls: callsA, totalExecTime: totalExecTimeA},
			{userid: "16416", calls: callsB, totalExecTime: totalExecTimeB},
		} {
			values := map[string]driver.Value{
				"datname":               "postgres",
				"query":                 "SELECT count(*) FROM pg_class",
				queryidColumnName:       "42",
				"userid":                r.userid,
				"rolname":               "",
				callsColumnName:         r.calls,
				totalExecTimeColumnName: r.totalExecTime,
			}
			for _, column := range topQueryColumns {
				if _, exists := values[column]; !exists {
					values[column] = "0"
				}
			}
			row := make([]driver.Value, len(topQueryColumns))
			for i, column := range topQueryColumns {
				row[i] = values[column]
			}
			rows.AddRow(row...)
		}
		return rows
	}

	mock.ExpectQuery(regexp.QuoteMeta(statStatementsResetAvailableQuery)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	mock.ExpectQuery("LIMIT 3").WillReturnRows(buildRows("100", "5000", "1000", "2000"))
	_, err = scraper.scrapeTopQuery(t.Context(), cfg.TopQueryCollection.MaxRowsPerQuery, cfg.TopQueryCollection.TopNQuery, cfg.TopQueryCollection.MaxExplainEachInterval, 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	// Second scrape: each role's cumulative calls advances by 10.
	mock.ExpectQuery(regexp.QuoteMeta(statStatementsResetAvailableQuery)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("LIMIT 3").WillReturnRows(buildRows("110", "5010", "1100", "2200"))
	logs, err := scraper.scrapeTopQuery(t.Context(), cfg.TopQueryCollection.MaxRowsPerQuery, cfg.TopQueryCollection.TopNQuery, cfg.TopQueryCollection.MaxExplainEachInterval, 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	gotCalls := make(map[string]int64, 2)
	for _, rl := range logs.ResourceLogs().All() {
		for _, sl := range rl.ScopeLogs().All() {
			for _, lr := range sl.LogRecords().All() {
				attrs := lr.Attributes().AsRaw()
				userid := attrs[dbAttributePrefix+"userid"].(string)
				gotCalls[userid] = attrs[dbAttributePrefix+callsColumnName].(int64)
			}
		}
	}
	assert.Equal(t, map[string]int64{"16415": 10, "16416": 10}, gotCalls,
		"each dropped role's calls delta must be computed against its own cumulative value")
	mock.ExpectClose()
}

func TestTopQueryPlanCacheSeparatesDatabaseAndRole(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.MaxRowsPerQuery = 3
	cfg.TopQueryCollection.TopNQuery = 3
	cfg.TopQueryCollection.MaxExplainEachInterval = 3

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	factory := &recordingClientFactory{mockSimpleClientFactory: mockSimpleClientFactory{db: db}}
	scraper, err := newTopQueryScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory)
	require.NoError(t, err)

	// Distinct mocked plans let us verify attribution independently of how the
	// database would plan the query under the receiver's connection credentials.
	statements := []struct {
		database string
		role     string
		userid   string
		plan     string
	}{
		// app is the same role cluster-wide, so it carries the same userid in both
		// databases: database, not userid, is what disambiguates these two rows.
		{database: "db_a", role: "app", userid: "101", plan: `[{"Plan":{"Node Type":"Seq Scan"}}]`},
		{database: "db_b", role: "app", userid: "101", plan: `[{"Plan":{"Node Type":"Index Scan"}}]`},
		{database: "db_a", role: "reporting", userid: "102", plan: `[{"Plan":{"Node Type":"Index Only Scan"}}]`},
	}
	expectedPlans := make(map[[2]string]string, len(statements))
	for _, statement := range statements {
		expectedPlans[[2]string{statement.database, statement.role}] = statement.plan
	}

	for scrape, name := range []string{"seed counters", "active statements fetch plans", "reordered active statements reuse cached plans"} {
		t.Run(name, func(t *testing.T) {
			factory.requestedDatabases = nil
			order := []int{0, 1, 2}
			if scrape > 1 {
				slices.Reverse(order)
			}
			rows := sqlmock.NewRows(topQueryColumns)
			for _, i := range order {
				statement := statements[i]
				values := map[string]driver.Value{
					"datname":               statement.database,
					"rolname":               statement.role,
					"userid":                statement.userid,
					"query":                 "SELECT count(*) FROM pg_class",
					queryidColumnName:       "42",
					totalExecTimeColumnName: fmt.Sprint((3-i)*10000 + scrape*(i+1)*1000),
				}
				row := make([]driver.Value, len(topQueryColumns))
				for j, column := range topQueryColumns {
					if value, exists := values[column]; exists {
						row[j] = value
					} else {
						row[j] = fmt.Sprint(10000 + scrape*(i+1))
					}
				}
				rows.AddRow(row...)
			}
			mock.ExpectQuery(regexp.QuoteMeta(statStatementsResetAvailableQuery)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			mock.ExpectQuery("LIMIT 3").WillReturnRows(rows)

			expectedDatabases := []string{defaultPostgreSQLDatabase}
			if scrape == 1 {
				// Execution time deltas set the EXPLAIN order.
				for _, i := range []int{2, 1, 0} {
					statement := statements[i]
					expectedDatabases = append(expectedDatabases, statement.database)
					mock.ExpectQuery(regexp.QuoteMeta("/* otel-collector-ignore */ SET plan_cache_mode = force_generic_plan;PREPARE otel_42 AS SELECT count(*) FROM pg_class;")).
						WillReturnRows(sqlmock.NewRows([]string{"result"}))
					mock.ExpectQuery(regexp.QuoteMeta("/* otel-collector-ignore */ SELECT COALESCE(array_length(parameter_types, 1), 0) AS param_count FROM pg_prepared_statements WHERE name = 'otel_42';")).
						WillReturnRows(sqlmock.NewRows([]string{"param_count"}).AddRow("0"))
					mock.ExpectQuery(regexp.QuoteMeta("EXPLAIN(FORMAT JSON) EXECUTE otel_42;")).
						WillReturnRows(sqlmock.NewRows([]string{"QUERY PLAN"}).AddRow(statement.plan))
					mock.ExpectExec(regexp.QuoteMeta("/* otel-collector-ignore */ DEALLOCATE PREPARE otel_42")).
						WillReturnResult(sqlmock.NewResult(0, 0))
				}
			}

			logs, scrapeErr := scraper.scrapeTopQuery(t.Context(), cfg.TopQueryCollection.MaxRowsPerQuery, cfg.TopQueryCollection.TopNQuery, cfg.TopQueryCollection.MaxExplainEachInterval, 0)
			require.NoError(t, scrapeErr)
			assert.Equal(t, expectedDatabases, factory.requestedDatabases)
			if scrape == 0 {
				require.Zero(t, logs.LogRecordCount())
				require.NoError(t, mock.ExpectationsWereMet())
				return
			}
			require.Equal(t, len(statements), logs.LogRecordCount())
			assert.Equal(t, len(statements), scraper.queryPlanCache.Len())

			seen := make(map[[2]string]bool, len(statements))
			for _, resourceLogs := range logs.ResourceLogs().All() {
				for _, scopeLogs := range resourceLogs.ScopeLogs().All() {
					for _, record := range scopeLogs.LogRecords().All() {
						attrs := record.Attributes().AsRaw()
						identity := [2]string{attrs["db.namespace"].(string), attrs[dbAttributePrefix+"rolname"].(string)}
						expectedPlan, exists := expectedPlans[identity]
						require.True(t, exists, "unexpected statement identity: %v", identity)
						assert.NotContains(t, seen, identity)
						seen[identity] = true
						assert.Equal(t, "42", attrs[dbAttributePrefix+queryidColumnName])
						assert.JSONEq(t, expectedPlan, attrs[dbAttributePrefix+"query_plan"].(string))
					}
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
	mock.ExpectClose()
}

// TestTopQueryPlanCacheSeparatesDroppedRoles guards the plan cache key against
// two dropped roles (empty rolname) colliding and sharing one cached plan.
func TestTopQueryPlanCacheSeparatesDroppedRoles(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.MaxRowsPerQuery = 3
	cfg.TopQueryCollection.TopNQuery = 3
	cfg.TopQueryCollection.MaxExplainEachInterval = 3

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })

	scraper, err := newTopQueryScraper(receivertest.NewNopSettings(metadata.Type), cfg, mockSimpleClientFactory{db: db})
	require.NoError(t, err)

	statements := []struct {
		userid string
		plan   string
	}{
		{userid: "16415", plan: `[{"Plan":{"Node Type":"Seq Scan"}}]`},
		{userid: "16416", plan: `[{"Plan":{"Node Type":"Index Scan"}}]`},
	}
	expectedPlans := make(map[string]string, len(statements))
	for _, statement := range statements {
		expectedPlans[statement.userid] = statement.plan
		scraper.cache.Add("postgres\x00"+statement.userid+"\x0042", statementSnapshot{counters: statementCounters{
			callsColumnName: 0, rowsColumnName: 0,
			sharedBlksDirtiedColumnName: 0, sharedBlksHitColumnName: 0,
			sharedBlksReadColumnName: 0, sharedBlksWrittenColumnName: 0,
			tempBlksReadColumnName: 0, tempBlksWrittenColumnName: 0,
			totalExecTimeColumnName: 0, totalPlanTimeColumnName: 0,
		}})
	}

	rows := sqlmock.NewRows(topQueryColumns)
	for i, statement := range statements {
		values := map[string]driver.Value{
			"datname":               "postgres",
			"userid":                statement.userid,
			"rolname":               "", // both roles are dropped: rolname alone cannot tell them apart
			"query":                 "SELECT count(*) FROM pg_class",
			queryidColumnName:       "42",
			totalExecTimeColumnName: fmt.Sprint((2 - i) * 10000),
		}
		row := make([]driver.Value, len(topQueryColumns))
		for j, column := range topQueryColumns {
			if value, exists := values[column]; exists {
				row[j] = value
			} else {
				row[j] = "10000"
			}
		}
		rows.AddRow(row...)
	}
	mock.ExpectQuery(regexp.QuoteMeta(statStatementsResetAvailableQuery)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery("LIMIT 3").WillReturnRows(rows)

	for _, statement := range statements {
		mock.ExpectQuery(regexp.QuoteMeta("/* otel-collector-ignore */ SET plan_cache_mode = force_generic_plan;PREPARE otel_42 AS SELECT count(*) FROM pg_class;")).
			WillReturnRows(sqlmock.NewRows([]string{"result"}))
		mock.ExpectQuery(regexp.QuoteMeta("/* otel-collector-ignore */ SELECT COALESCE(array_length(parameter_types, 1), 0) AS param_count FROM pg_prepared_statements WHERE name = 'otel_42';")).
			WillReturnRows(sqlmock.NewRows([]string{"param_count"}).AddRow("0"))
		mock.ExpectQuery(regexp.QuoteMeta("EXPLAIN(FORMAT JSON) EXECUTE otel_42;")).
			WillReturnRows(sqlmock.NewRows([]string{"QUERY PLAN"}).AddRow(statement.plan))
		mock.ExpectExec(regexp.QuoteMeta("/* otel-collector-ignore */ DEALLOCATE PREPARE otel_42")).
			WillReturnResult(sqlmock.NewResult(0, 0))
	}

	logs, err := scraper.scrapeTopQuery(t.Context(), cfg.TopQueryCollection.MaxRowsPerQuery, cfg.TopQueryCollection.TopNQuery, cfg.TopQueryCollection.MaxExplainEachInterval, 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, len(statements), logs.LogRecordCount())
	assert.Equal(t, len(statements), scraper.queryPlanCache.Len(),
		"each dropped role must get its own plan cache entry, not share one keyed on empty rolname")

	seen := make(map[string]bool, len(statements))
	for _, resourceLogs := range logs.ResourceLogs().All() {
		for _, scopeLogs := range resourceLogs.ScopeLogs().All() {
			for _, record := range scopeLogs.LogRecords().All() {
				attrs := record.Attributes().AsRaw()
				userid := attrs[dbAttributePrefix+"userid"].(string)
				expectedPlan, exists := expectedPlans[userid]
				require.True(t, exists, "unexpected userid: %v", userid)
				assert.NotContains(t, seen, userid)
				seen[userid] = true
				assert.JSONEq(t, expectedPlan, attrs[dbAttributePrefix+"query_plan"].(string))
			}
		}
	}
	mock.ExpectClose()
}
