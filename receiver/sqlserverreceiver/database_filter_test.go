// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"context"
	"database/sql"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

func TestDatabaseFilterAllows(t *testing.T) {
	tests := []struct {
		name     string
		include  []string
		exclude  []string
		allowed  []string
		rejected []string
	}{
		{
			name:    "no filter allows every database",
			allowed: []string{"master", "sales", ""},
		},
		{
			name:     "include keeps only the listed databases",
			include:  []string{"sales", "hr"},
			allowed:  []string{"sales", "hr"},
			rejected: []string{"master", ""},
		},
		{
			name:     "exclude drops the listed databases",
			exclude:  []string{"rdsadmin"},
			allowed:  []string{"master", "sales", ""},
			rejected: []string{"rdsadmin"},
		},
		{
			name:     "exclude wins over include",
			include:  []string{"sales", "hr"},
			exclude:  []string{"hr"},
			allowed:  []string{"sales"},
			rejected: []string{"hr", "master"},
		},
		{
			name:     "names match case-insensitively",
			include:  []string{"Sales"},
			exclude:  []string{"HR"},
			allowed:  []string{"sales", "SALES"},
			rejected: []string{"hr"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := newDatabaseFilter(&Config{Databases: tt.include, ExcludeDatabases: tt.exclude})
			for _, db := range tt.allowed {
				assert.True(t, filter.allows(db), "database %q should be allowed", db)
			}
			for _, db := range tt.rejected {
				assert.False(t, filter.allows(db), "database %q should be rejected", db)
			}
		})
	}
}

func TestDatabaseFilterApply(t *testing.T) {
	const query = "SELECT * FROM t AS st{database_filter WHERE DB_NAME(st.dbid)};\nSELECT * FROM u WHERE x = 1{database_filter AND [name]};"
	tests := []struct {
		name     string
		filter   databaseFilter
		expected string
		args     []any
	}{
		{
			name:     "no filter removes the placeholders",
			expected: "SELECT * FROM t AS st;\nSELECT * FROM u WHERE x = 1;",
		},
		{
			name:   "include only",
			filter: newDatabaseFilter(&Config{Databases: []string{"Sales", "hr"}}),
			expected: "SELECT * FROM t AS st WHERE LOWER(ISNULL(DB_NAME(st.dbid), N'')) IN (@includeDatabase0, @includeDatabase1);\n" +
				"SELECT * FROM u WHERE x = 1 AND LOWER(ISNULL([name], N'')) IN (@includeDatabase0, @includeDatabase1);",
			args: []any{sql.Named("includeDatabase0", "sales"), sql.Named("includeDatabase1", "hr")},
		},
		{
			name:   "exclude only",
			filter: newDatabaseFilter(&Config{ExcludeDatabases: []string{"rdsadmin"}}),
			expected: "SELECT * FROM t AS st WHERE LOWER(ISNULL(DB_NAME(st.dbid), N'')) NOT IN (@excludeDatabase0);\n" +
				"SELECT * FROM u WHERE x = 1 AND LOWER(ISNULL([name], N'')) NOT IN (@excludeDatabase0);",
			args: []any{sql.Named("excludeDatabase0", "rdsadmin")},
		},
		{
			name:   "include and exclude",
			filter: newDatabaseFilter(&Config{Databases: []string{"sales"}, ExcludeDatabases: []string{"it's"}}),
			expected: "SELECT * FROM t AS st WHERE LOWER(ISNULL(DB_NAME(st.dbid), N'')) IN (@includeDatabase0) AND LOWER(ISNULL(DB_NAME(st.dbid), N'')) NOT IN (@excludeDatabase0);\n" +
				"SELECT * FROM u WHERE x = 1 AND LOWER(ISNULL([name], N'')) IN (@includeDatabase0) AND LOWER(ISNULL([name], N'')) NOT IN (@excludeDatabase0);",
			args: []any{sql.Named("includeDatabase0", "sales"), sql.Named("excludeDatabase0", "it's")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.filter.apply(query))
			if tt.args == nil {
				assert.Empty(t, tt.filter.args())
			} else {
				assert.Equal(t, tt.args, tt.filter.args())
			}
		})
	}
}

func TestDatabaseFilteredQueries(t *testing.T) {
	filter := newDatabaseFilter(&Config{ExcludeDatabases: []string{"rdsadmin"}})
	tests := []struct {
		name      string
		query     string
		condition string
	}{
		{
			name:      "top query",
			query:     getSQLServerQueryTextAndPlanQuery(),
			condition: "AS st1 WHERE LOWER(ISNULL(DB_NAME(st1.dbid), N'')) NOT IN (@excludeDatabase0)\n",
		},
		{
			name:      "query sample",
			query:     getSQLServerQuerySamplesQuery(),
			condition: "AND r.session_id != @@SPID AND LOWER(ISNULL(DB_NAME(r.database_id), N'')) NOT IN (@excludeDatabase0);",
		},
		{
			name:      "top procedure",
			query:     getSQLServerTopProcedureQuery(),
			condition: "GETDATE()) AND LOWER(ISNULL(DB_NAME(ps.database_id), N'')) NOT IN (@excludeDatabase0)\n",
		},
		{
			name:      "index physical stats",
			query:     getSQLServerIndexPhysicalStatsQuery(),
			condition: "[database_id] > 4 AND LOWER(ISNULL([name], N'')) NOT IN (@excludeDatabase0);",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Len(t, databaseFilterPlaceholder.FindAllString(tt.query, -1), 1)
			rendered := filter.apply(tt.query)
			assert.NotContains(t, rendered, "{database_filter")
			assert.Contains(t, rendered, tt.condition)
		})
	}
}

// newDatabaseFilterTestScraper returns a scraper for query whose client answers
// with rows and records the arguments of each call into args.
func newDatabaseFilterTestScraper(cfg *Config, query string, rows []sqlquery.StringMap, args *[]any) *sqlServerScraperHelper {
	s := newSQLServerScraper(component.MustNewID("sqlserver"), query, sqlquery.TelemetryConfig{}, nil, nil,
		receivertest.NewNopSettings(metadata.Type), cfg, newCache(1))
	s.client = queryRowsFuncClient{queryRowsFunc: func(_ context.Context, a ...any) ([]sqlquery.StringMap, error) {
		*args = a
		return rows, nil
	}}
	return s
}

// reportedDatabases returns the sorted, distinct sqlserver.database.name values
// of the resources that carry metrics.
func reportedDatabases(md pmetric.Metrics) []string {
	var names []string
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		if rm.ScopeMetrics().Len() == 0 {
			continue
		}
		if db, ok := rm.Resource().Attributes().Get("sqlserver.database.name"); ok && !slices.Contains(names, db.AsString()) {
			names = append(names, db.AsString())
		}
	}
	slices.Sort(names)
	return names
}

func TestDatabaseFilterMetrics(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		fixture  string
		include  []string
		exclude  []string
		expected []string
	}{
		{
			name:     "database io without a filter",
			query:    getSQLServerDatabaseIOQuery(),
			fixture:  "database_io_scraped_data.txt",
			expected: []string{"master", "model", "msdb", "tempdb"},
		},
		{
			name:     "database io",
			query:    getSQLServerDatabaseIOQuery(),
			fixture:  "database_io_scraped_data.txt",
			include:  []string{"MASTER", "msdb"},
			exclude:  []string{"msdb"},
			expected: []string{"master"},
		},
		{
			name:     "availability group",
			query:    getSQLServerAvailabilityGroupQuery(),
			fixture:  "availabilityGroupQueryData.txt",
			exclude:  []string{"UserDB2"},
			expected: []string{"UserDB", "UserDB3"},
		},
		{
			name:    "index physical stats",
			query:   getSQLServerIndexPhysicalStatsQuery(),
			fixture: "indexPhysicalQueryData.txt",
			exclude: []string{"testdb"},
		},
		{
			name:     "databases performance counters",
			query:    getSQLServerPerformanceCounterQuery(),
			fixture:  "perfCounterQueryData.txt",
			include:  []string{"master", "tempdb", "msdb"},
			exclude:  []string{"msdb"},
			expected: []string{"master", "tempdb"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			configureAllScraperMetricsAndEvents(cfg, true)
			cfg.Databases = tt.include
			cfg.ExcludeDatabases = tt.exclude

			rows, err := readFile(tt.fixture)
			require.NoError(t, err)
			var args []any
			s := newDatabaseFilterTestScraper(cfg, tt.query, rows, &args)

			md, err := s.ScrapeMetrics(t.Context())
			require.NoError(t, err)
			assert.Equal(t, tt.expected, reportedDatabases(md))
		})
	}
}

func TestDatabaseFilterPerformanceCountersKeepsInstanceMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	configureAllScraperMetricsAndEvents(cfg, true)
	cfg.Databases = []string{"no_such_database"}

	rows, err := readFile("perfCounterQueryData.txt")
	require.NoError(t, err)
	var args []any
	s := newDatabaseFilterTestScraper(cfg, getSQLServerPerformanceCounterQuery(), rows, &args)

	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)
	assert.Empty(t, reportedDatabases(md))
	assert.Positive(t, md.DataPointCount(), "counters that are not per database are still reported")
}

func TestDatabaseFilterBindsQueryArguments(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		record func(*sqlServerScraperHelper, context.Context) error
	}{
		{
			name:  "index physical stats",
			query: getSQLServerIndexPhysicalStatsQuery(),
			record: func(s *sqlServerScraperHelper, ctx context.Context) error {
				return s.recordIndexPhysicalMetrics(ctx)
			},
		},
		{
			name:  "top query",
			query: getSQLServerQueryTextAndPlanQuery(),
			record: func(s *sqlServerScraperHelper, ctx context.Context) error {
				_, err := s.recordDatabaseQueryTextAndPlan(ctx)
				return err
			},
		},
		{
			name:  "query sample",
			query: getSQLServerQuerySamplesQuery(),
			record: func(s *sqlServerScraperHelper, ctx context.Context) error {
				_, err := s.recordDatabaseSampleQuery(ctx)
				return err
			},
		},
		{
			name:  "top procedure",
			query: getSQLServerTopProcedureQuery(),
			record: func(s *sqlServerScraperHelper, ctx context.Context) error {
				_, err := s.recordDatabaseTopProcedure(ctx)
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Databases = []string{"Sales"}
			cfg.ExcludeDatabases = []string{"hr"}

			var args []any
			s := newDatabaseFilterTestScraper(cfg, tt.query, nil, &args)
			require.NoError(t, tt.record(s, t.Context()))

			assert.Subset(t, args, []any{sql.Named("includeDatabase0", "sales"), sql.Named("excludeDatabase0", "hr")})
		})
	}
}

func TestDatabaseFilterAppliedToClientQuery(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ExcludeDatabases = []string{"rdsadmin"}

	var clientQuery string
	s := newSQLServerScraper(component.MustNewID("sqlserver"), getSQLServerQuerySamplesQuery(), sqlquery.TelemetryConfig{},
		func() (*sql.DB, error) { return nil, nil },
		func(_ sqlquery.Db, query string, _ *zap.Logger, _ sqlquery.TelemetryConfig) sqlquery.DbClient {
			clientQuery = query
			return mockEmptyResultClient{}
		},
		receivertest.NewNopSettings(metadata.Type), cfg, newCache(1))
	require.NoError(t, s.Start(t.Context(), nil))

	assert.Equal(t, newDatabaseFilter(cfg).apply(getSQLServerQuerySamplesQuery()), clientQuery)
	assert.Contains(t, clientQuery, "NOT IN (@excludeDatabase0)")
	// The scraper keeps the template, which is what picks the recorder.
	assert.Equal(t, getSQLServerQuerySamplesQuery(), s.sqlQuery)
}
