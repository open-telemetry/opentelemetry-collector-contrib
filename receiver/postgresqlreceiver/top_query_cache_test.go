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

func TestTopQueryCacheRetainsFetchedStatements(t *testing.T) {
	columns := []string{
		callsColumnName, "datname", sharedBlksDirtiedColumnName, sharedBlksHitColumnName,
		sharedBlksReadColumnName, sharedBlksWrittenColumnName, tempBlksReadColumnName,
		tempBlksWrittenColumnName, "query", queryidColumnName, "rolname", rowsColumnName,
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
		{name: "first scrape", ids: ids, expectedCount: 50},
		{name: "unchanged counters", ids: ids},
		{name: "reordered statements", ids: reversed},
		{name: "changed statement membership", ids: changed, expectedCount: 50},
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
		tempBlksWrittenColumnName, "query", queryidColumnName, "rolname", rowsColumnName,
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

	rows := sqlmock.NewRows(columns)
	for _, identity := range []struct {
		database string
		role     string
	}{
		{database: "db_a", role: "app"},
		{database: "db_b", role: "app"},
		{database: "db_a", role: "reporting"},
	} {
		values := map[string]driver.Value{
			"datname":         identity.database,
			"query":           "SELECT count(*) FROM pg_class",
			queryidColumnName: "42",
			"rolname":         identity.role,
		}
		for _, column := range columns {
			if _, exists := values[column]; !exists {
				values[column] = "10000"
			}
		}
		row := make([]driver.Value, len(columns))
		for i, column := range columns {
			row[i] = values[column]
		}
		rows.AddRow(row...)
	}

	mock.ExpectQuery("LIMIT 3").WillReturnRows(rows)
	logs, err := scraper.scrapeTopQuery(t.Context(), cfg.TopQueryCollection.MaxRowsPerQuery, cfg.TopQueryCollection.TopNQuery, cfg.TopQueryCollection.MaxExplainEachInterval, 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, 3, logs.LogRecordCount())
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
		plan     string
	}{
		{database: "db_a", role: "app", plan: `[{"Plan":{"Node Type":"Seq Scan"}}]`},
		{database: "db_b", role: "app", plan: `[{"Plan":{"Node Type":"Index Scan"}}]`},
		{database: "db_a", role: "reporting", plan: `[{"Plan":{"Node Type":"Index Only Scan"}}]`},
	}
	expectedPlans := make(map[[2]string]string, len(statements))
	for _, statement := range statements {
		expectedPlans[[2]string{statement.database, statement.role}] = statement.plan
	}

	for scrape, name := range []string{"first scrape", "reordered active statements reuse cached plans"} {
		t.Run(name, func(t *testing.T) {
			factory.requestedDatabases = nil
			order := []int{0, 1, 2}
			if scrape > 0 {
				slices.Reverse(order)
			}
			rows := sqlmock.NewRows(topQueryColumns)
			for _, i := range order {
				statement := statements[i]
				values := map[string]driver.Value{
					"datname":               statement.database,
					"rolname":               statement.role,
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
			mock.ExpectQuery("LIMIT 3").WillReturnRows(rows)

			expectedDatabases := []string{defaultPostgreSQLDatabase}
			if scrape == 0 {
				// Distinct execution times make the first scrape's EXPLAIN order
				// deterministic. The second scrape reverses that priority as well.
				for _, statement := range statements {
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
