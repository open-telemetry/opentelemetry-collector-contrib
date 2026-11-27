// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"database/sql/driver"
	"fmt"
	"regexp"
	"slices"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

func TestTopQuerySeparatesAndMergesTopLevel(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
	cfg.TopQueryCollection.MaxRowsPerQuery = 10
	cfg.TopQueryCollection.TopNQuery = 1
	cfg.TopQueryCollection.MaxExplainEachInterval = 1

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, db.Close()) })
	factory := &recordingClientFactory{mockSimpleClientFactory: mockSimpleClientFactory{db: db}}
	scraper, err := newTopQueryScraper(receivertest.NewNopSettings(metadata.Type), cfg, factory)
	require.NoError(t, err)

	type statement struct {
		id       string
		toplevel bool
		calls    int
		execTime int
	}
	plan := `[{"Plan":{"Node Type":"Result"}}]`
	for _, step := range []struct {
		name     string
		rows     []statement
		calls    int64
		execTime float64
		explain  bool
		topN     int64
	}{
		{
			name: "seed direct statements",
			rows: []statement{{"42", true, 100, 1000}, {"99", true, 10, 100}},
		},
		{
			name:  "new nested entry seeds its own baseline",
			rows:  []statement{{"42", true, 105, 1050}, {"42", false, 20, 200}, {"99", true, 10, 100}},
			calls: 5, execTime: 0.05, explain: true,
		},
		{
			// Each component is below query 99's 100 ms delta. Their sum wins.
			name:  "merge before top one selection",
			rows:  []statement{{"42", true, 110, 1100}, {"42", false, 22, 270}, {"99", true, 11, 200}},
			calls: 7, execTime: 0.12, topN: 1,
		},
		{
			name: "reordered unchanged rows",
			rows: []statement{{"99", true, 11, 200}, {"42", false, 22, 270}, {"42", true, 110, 1100}},
		},
		{
			name:  "only nested counters reset",
			rows:  []statement{{"42", true, 111, 1110}, {"42", false, 3, 30}, {"99", true, 11, 200}},
			calls: 4, execTime: 0.04,
		},
		{
			name:  "merged counters do not mutate cached baselines",
			rows:  []statement{{"42", false, 4, 40}, {"42", true, 112, 1120}, {"99", true, 11, 200}},
			calls: 2, execTime: 0.02,
		},
	} {
		t.Run(step.name, func(t *testing.T) {
			factory.requestedDatabases = nil
			columns := append(slices.Clone(topQueryColumns), "toplevel")
			rows := sqlmock.NewRows(columns)
			for _, statement := range step.rows {
				values := map[string]driver.Value{
					"datname": "postgres", "userid": "101", "rolname": "otel",
					"query": "SELECT 1", queryidColumnName: statement.id,
					"toplevel":              fmt.Sprint(statement.toplevel),
					totalExecTimeColumnName: fmt.Sprint(statement.execTime),
					totalPlanTimeColumnName: fmt.Sprint(statement.execTime / 2),
				}
				row := make([]driver.Value, len(columns))
				for i, column := range columns {
					if value, exists := values[column]; exists {
						row[i] = value
					} else {
						row[i] = fmt.Sprint(statement.calls)
					}
				}
				rows.AddRow(row...)
			}
			mock.ExpectQuery(regexp.QuoteMeta(statStatementsCapabilitiesQuery)).
				WillReturnRows(sqlmock.NewRows([]string{"reset", "toplevel"}).AddRow(false, true))
			mock.ExpectQuery("LIMIT 10").WillReturnRows(rows)
			if step.explain {
				mock.ExpectQuery(regexp.QuoteMeta("/* otel-collector-ignore */ SET plan_cache_mode = force_generic_plan;PREPARE otel_42 AS SELECT 1;")).
					WillReturnRows(sqlmock.NewRows([]string{"result"}))
				mock.ExpectQuery("SELECT COALESCE").WillReturnRows(sqlmock.NewRows([]string{"param_count"}).AddRow("0"))
				mock.ExpectQuery(regexp.QuoteMeta("EXPLAIN(FORMAT JSON) EXECUTE otel_42;")).
					WillReturnRows(sqlmock.NewRows([]string{"QUERY PLAN"}).AddRow(plan))
				mock.ExpectExec("DEALLOCATE PREPARE otel_42").WillReturnResult(sqlmock.NewResult(0, 0))
			}
			topN := step.topN
			if topN == 0 {
				topN = 10
			}
			logs, scrapeErr := scraper.scrapeTopQuery(t.Context(), 10, topN, 1, 0)
			require.NoError(t, scrapeErr)
			require.NoError(t, mock.ExpectationsWereMet())
			wantDatabases := []string{"postgres"}
			if step.explain {
				wantDatabases = append(wantDatabases, "postgres")
			}
			assert.Equal(t, wantDatabases, factory.requestedDatabases)
			if step.calls == 0 {
				require.Zero(t, logs.LogRecordCount())
				return
			}
			require.Equal(t, 1, logs.LogRecordCount())
			record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
			assert.Equal(t, "db.server.top_query", record.EventName())
			attrs := record.Attributes().AsRaw()
			assert.Equal(t, "42", attrs[dbAttributePrefix+queryidColumnName])
			for _, column := range []string{
				callsColumnName, rowsColumnName, sharedBlksDirtiedColumnName,
				sharedBlksHitColumnName, sharedBlksReadColumnName, sharedBlksWrittenColumnName,
				tempBlksReadColumnName, tempBlksWrittenColumnName,
			} {
				assert.Equal(t, step.calls, attrs[dbAttributePrefix+column], column)
			}
			assert.InDelta(t, step.execTime, attrs[dbAttributePrefix+totalExecTimeColumnName], 1e-12)
			assert.InDelta(t, step.execTime/2, attrs[dbAttributePrefix+totalPlanTimeColumnName], 1e-12)
			assert.JSONEq(t, plan, attrs[dbAttributePrefix+"query_plan"].(string))
			assert.NotContains(t, attrs, dbAttributePrefix+"toplevel")
		})
	}
	mock.ExpectClose()
}
