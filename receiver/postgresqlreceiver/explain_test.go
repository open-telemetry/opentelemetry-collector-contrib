// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"database/sql"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

const (
	explainProbeSQL = `/* otel-collector-ignore */ SELECT "otel"."explain_statement"('SELECT 1')`
	explainCallSQL  = `/* otel-collector-ignore */ SELECT "otel"."explain_statement"($1) AS query_plan`
)

func TestExplainFunctionConfig(t *testing.T) {
	for _, tc := range []struct {
		name, ttl, wantErr string
	}{
		{name: "otel.explain_statement", ttl: "5m"},
		{name: "custom_explain", ttl: "1s"},
		{name: "", ttl: "0s"},
		{name: "otel.explain_statement; SELECT 1", ttl: "5m", wantErr: "function_name"},
		{name: "a.b.c", ttl: "5m", wantErr: "function_name"},
		{name: "otel.explain_statement", ttl: "0s", wantErr: "cache_ttl"},
		{name: "otel.explain_statement", ttl: "-1s", wantErr: "cache_ttl"},
	} {
		t.Run(tc.name+"/"+tc.ttl, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cm := confmap.NewFromStringMap(map[string]any{
				"username": "monitor", "password": "password",
				"top_query_collection": map[string]any{
					"explain_function_name": tc.name, "explain_function_cache_ttl": tc.ttl,
				},
			})
			require.NoError(t, cm.Unmarshal(cfg))
			err := cfg.Validate()
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProbeExplainFunction(t *testing.T) {
	probeErr := errors.New("function failed while returning its result")
	for _, tc := range []struct {
		name string
		rows *sqlmock.Rows
		err  error
	}{
		{name: "success", rows: sqlmock.NewRows([]string{"explain_statement"}).AddRow(`[{"Plan":{}}]`)},
		{name: "late error", rows: sqlmock.NewRows([]string{"explain_statement"}).AddRow("").RowError(0, probeErr), err: probeErr},
		{name: "empty result", rows: sqlmock.NewRows([]string{"explain_statement"}), err: sql.ErrNoRows},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectQuery(explainProbeSQL).WillReturnRows(tc.rows).RowsWillBeClosed()
			client := &postgreSQLClient{client: db}
			require.ErrorIs(t, client.probeExplainFunction(t.Context(), `"otel"."explain_statement"`), tc.err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestExplainFunctionProbeCache(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		cfg := createDefaultConfig().(*Config)
		cfg.TopQueryCollection.ExplainFunctionName = "otel.explain_statement"
		p, err := newPostgreSQLScraper(receivertest.NewNopSettings(metadata.Type), cfg,
			mockSimpleClientFactory{db: db}, newCache(1), newTTLCache[string](1, 0))
		require.NoError(t, err)
		client := &postgreSQLClient{client: db}
		quoted := `"otel"."explain_statement"`
		mock.ExpectQuery(explainProbeSQL).WillReturnRows(sqlmock.NewRows([]string{"plan"}).AddRow(`[{"Plan":{}}]`))
		assert.Equal(t, quoted, p.explainFunction(t.Context(), "first", client))
		assert.Equal(t, quoted, p.explainFunction(t.Context(), "first", client))
		mock.ExpectQuery(explainProbeSQL).WillReturnError(errors.New("connection refused"))
		assert.Empty(t, p.explainFunction(t.Context(), "second", client))
		assert.Empty(t, p.explainFunction(t.Context(), "second", client))
		time.Sleep(cfg.TopQueryCollection.ExplainFunctionCacheTTL)
		mock.ExpectQuery(explainProbeSQL).WillReturnRows(sqlmock.NewRows([]string{"plan"}).AddRow(`[{"Plan":{}}]`))
		assert.Equal(t, quoted, p.explainFunction(t.Context(), "second", client))
		cfg.TopQueryCollection.ExplainFunctionName = ""
		assert.Empty(t, p.explainFunction(t.Context(), "second", client))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestExplainQueryViaFunction(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	query := "UPDATE items SET value = $1 WHERE id = $2"
	mock.ExpectQuery(explainCallSQL).WithArgs(query).WillReturnRows(sqlmock.NewRows([]string{"query_plan"}).AddRow(
		`[{"Plan":{"Node Type":"Seq Scan","Relation Name":"items","Filter":"(value = 'private value')"}}]`,
	))
	client := &postgreSQLClient{client: db}
	plan, err := client.explainQuery(t.Context(), query, "123", `"otel"."explain_statement"`, zap.NewNop())
	require.NoError(t, err)
	assert.Contains(t, plan, "Seq Scan")
	assert.NotContains(t, plan, "private value")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestScrapeTopQueriesViaFunction(t *testing.T) {
	for _, available := range []bool{true, false} {
		name := "available"
		if !available {
			name = "missing"
		}
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()
			cfg := createDefaultConfig().(*Config)
			cfg.LogsBuilderConfig.Events.DbServerTopQuery.Enabled = true
			cfg.TopQueryCollection.ExplainFunctionName = "otel.explain_statement"
			p, err := newTopQueryScraper(receivertest.NewNopSettings(metadata.Type), cfg, mockSimpleClientFactory{db: db})
			require.NoError(t, err)
			query := "SELECT $1"
			mock.ExpectQuery(expectedScrapeTopQuery).WillReturnRows(newSQLMockRows(topQueryColumns, map[string]any{
				callsColumnName: "1", "datname": "postgres", "query": query, queryidColumnName: "1",
				"userid": "10", "rolname": "app", rowsColumnName: "1",
				totalExecTimeColumnName: "1000", totalPlanTimeColumnName: "1000",
				sharedBlksDirtiedColumnName: "0", sharedBlksHitColumnName: "0", sharedBlksReadColumnName: "0",
				sharedBlksWrittenColumnName: "0", tempBlksReadColumnName: "0", tempBlksWrittenColumnName: "0",
			}))
			plan := `[{"Plan":{"Node Type":"Result"}}]`
			if available {
				mock.ExpectQuery(explainProbeSQL).WillReturnRows(sqlmock.NewRows([]string{"plan"}).AddRow(plan))
				mock.ExpectQuery(explainCallSQL).WithArgs(query).WillReturnRows(sqlmock.NewRows([]string{"query_plan"}).AddRow(plan))
			} else {
				mock.ExpectQuery(explainProbeSQL).WillReturnError(errors.New("function does not exist"))
				mock.ExpectQuery("/* otel-collector-ignore */ SET plan_cache_mode = force_generic_plan;PREPARE otel_1 AS SELECT $1;").
					WillReturnRows(sqlmock.NewRows([]string{"result"}))
				mock.ExpectQuery("/* otel-collector-ignore */ SELECT COALESCE(array_length(parameter_types, 1), 0) AS param_count FROM pg_prepared_statements WHERE name = 'otel_1';").
					WillReturnRows(sqlmock.NewRows([]string{"param_count"}).AddRow("1"))
				mock.ExpectQuery("EXPLAIN(FORMAT JSON) EXECUTE otel_1(null);").WillReturnRows(sqlmock.NewRows([]string{"QUERY PLAN"}).AddRow(plan))
				mock.ExpectExec("/* otel-collector-ignore */ DEALLOCATE PREPARE otel_1").WillReturnResult(sqlmock.NewResult(0, 0))
			}
			logs, err := p.scrapeTopQuery(t.Context(), 31, 1, 1, time.Minute)
			require.NoError(t, err)
			records := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
			require.Equal(t, 1, records.Len())
			attr, ok := records.At(0).Attributes().Get("postgresql.query_plan")
			require.True(t, ok)
			assert.JSONEq(t, plan, attr.Str())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
