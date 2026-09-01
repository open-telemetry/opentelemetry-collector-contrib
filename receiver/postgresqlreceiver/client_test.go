// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDatabaseConflicts(t *testing.T) {
	conflictColumns := []string{"datname", "confl_tablespace", "confl_lock", "confl_snapshot", "confl_bufferpin", "confl_deadlock"}

	tests := []struct {
		name        string
		databases   []string
		expectedSQL string
		rows        *sqlmock.Rows
		expected    map[databaseName]databaseConflictStats
	}{
		{
			name:        "all databases",
			databases:   nil,
			expectedSQL: "SELECT datname, confl_tablespace, confl_lock, confl_snapshot, confl_bufferpin, confl_deadlock FROM pg_stat_database_conflicts;",
			rows: sqlmock.NewRows(conflictColumns).
				AddRow("otel", 1, 2, 3, 4, 5).
				AddRow("telemetry", 6, 7, 8, 9, 10),
			expected: map[databaseName]databaseConflictStats{
				"otel":      {conflTablespace: 1, conflLock: 2, conflSnapshot: 3, conflBufferpin: 4, conflDeadlock: 5},
				"telemetry": {conflTablespace: 6, conflLock: 7, conflSnapshot: 8, conflBufferpin: 9, conflDeadlock: 10},
			},
		},
		{
			name:        "filtered by database",
			databases:   []string{"otel"},
			expectedSQL: "SELECT datname, confl_tablespace, confl_lock, confl_snapshot, confl_bufferpin, confl_deadlock FROM pg_stat_database_conflicts WHERE datname IN ('otel');",
			rows: sqlmock.NewRows(conflictColumns).
				AddRow("otel", 0, 0, 0, 0, 0),
			expected: map[databaseName]databaseConflictStats{
				"otel": {},
			},
		},
		{
			name:        "rows with empty datname are skipped",
			databases:   nil,
			expectedSQL: "SELECT datname, confl_tablespace, confl_lock, confl_snapshot, confl_bufferpin, confl_deadlock FROM pg_stat_database_conflicts;",
			rows: sqlmock.NewRows(conflictColumns).
				AddRow("", 1, 1, 1, 1, 1).
				AddRow("otel", 2, 2, 2, 2, 2),
			expected: map[databaseName]databaseConflictStats{
				"otel": {conflTablespace: 2, conflLock: 2, conflSnapshot: 2, conflBufferpin: 2, conflDeadlock: 2},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			client := &postgreSQLClient{client: db, closeFn: func() error { return nil }}

			mock.ExpectQuery(tc.expectedSQL).WillReturnRows(tc.rows)

			conflicts, err := client.getDatabaseConflicts(t.Context(), tc.databases)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, conflicts)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetExecutionTimeStats(t *testing.T) {
	const baseSQL = "SELECT pd.datname AS datname, SUM(pss.total_exec_time) / 1000.0 AS execution_time_seconds FROM pg_stat_statements pss JOIN pg_database pd ON pss.dbid = pd.oid"
	columns := []string{"datname", "execution_time_seconds"}

	tests := []struct {
		name        string
		databases   []string
		expectedSQL string
		rows        *sqlmock.Rows
		queryErr    error
		expected    map[databaseName]float64
		wantErr     bool
	}{
		{
			name:        "all databases",
			databases:   nil,
			expectedSQL: baseSQL + " GROUP BY datname;",
			// total_exec_time is reported in milliseconds; the query divides by 1000 to return seconds.
			rows: sqlmock.NewRows(columns).
				AddRow("otel", 1.5).
				AddRow("telemetry", 42.25),
			expected: map[databaseName]float64{
				"otel":      1.5,
				"telemetry": 42.25,
			},
		},
		{
			name:        "filtered by database",
			databases:   []string{"otel"},
			expectedSQL: baseSQL + " WHERE datname IN ('otel') GROUP BY datname;",
			rows: sqlmock.NewRows(columns).
				AddRow("otel", 0.0),
			expected: map[databaseName]float64{
				"otel": 0.0,
			},
		},
		{
			name:        "rows with empty datname are skipped",
			databases:   nil,
			expectedSQL: baseSQL + " GROUP BY datname;",
			rows: sqlmock.NewRows(columns).
				AddRow("", 9.9).
				AddRow("otel", 2.5),
			expected: map[databaseName]float64{
				"otel": 2.5,
			},
		},
		{
			name:        "query error when pg_stat_statements is unavailable",
			databases:   nil,
			expectedSQL: baseSQL + " GROUP BY datname;",
			queryErr:    errors.New(`relation "pg_stat_statements" does not exist`),
			expected:    nil,
			wantErr:     true,
		},
		{
			name:        "row scan error on non-numeric value",
			databases:   nil,
			expectedSQL: baseSQL + " GROUP BY datname;",
			rows: sqlmock.NewRows(columns).
				AddRow("otel", "not-a-number"),
			expected: map[databaseName]float64{},
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			client := &postgreSQLClient{client: db, closeFn: func() error { return nil }}

			if tc.queryErr != nil {
				mock.ExpectQuery(tc.expectedSQL).WillReturnError(tc.queryErr)
			} else {
				mock.ExpectQuery(tc.expectedSQL).WillReturnRows(tc.rows)
			}

			stats, err := client.getExecutionTimeStats(t.Context(), tc.databases)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expected, stats)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRepairNormalizedQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "extract with parameter is rewritten to date_part",
			query:    "SELECT * FROM orders WHERE EXTRACT($1 FROM order_date) = $2",
			expected: "SELECT * FROM orders WHERE date_part($1, order_date) = $2",
		},
		{
			name:     "extract rewrite preserves surrounding parameter numbering",
			query:    "SELECT CAST($1 AS varchar(50)) FROM orders WHERE EXTRACT($2 FROM order_date) BETWEEN $3 AND $4",
			expected: "SELECT CAST($1 AS varchar(50)) FROM orders WHERE date_part($2, order_date) BETWEEN $3 AND $4",
		},
		{
			name:     "extract rewrite is balanced when the source is a nested call",
			query:    "SELECT EXTRACT($1 FROM greatest(a, b))",
			expected: "SELECT date_part($1, greatest(a, b))",
		},
		{
			name:     "extract rewrite tolerates extra whitespace",
			query:    "SELECT extract(  $1   FROM   order_date )",
			expected: "SELECT date_part($1, order_date )",
		},
		{
			name:     "extract rewrite is case insensitive",
			query:    "SELECT Extract($1 From order_date)",
			expected: "SELECT date_part($1, order_date)",
		},
		{
			name:     "multiple extracts are all rewritten",
			query:    "SELECT EXTRACT($1 FROM a), EXTRACT($2 FROM b)",
			expected: "SELECT date_part($1, a), date_part($2, b)",
		},
		{
			name:     "extract with a literal field is already valid and left alone",
			query:    "SELECT EXTRACT(YEAR FROM order_date)",
			expected: "SELECT EXTRACT(YEAR FROM order_date)",
		},
		{
			name:     "typed interval literal is rewritten to a cast",
			query:    "SELECT now() - interval $1",
			expected: "SELECT now() - $1::interval",
		},
		{
			name:     "typed timestamp literal is rewritten to a cast",
			query:    "SELECT * FROM events WHERE created_at > timestamp $1",
			expected: "SELECT * FROM events WHERE created_at > $1::timestamp",
		},
		{
			name:     "identifier prefixed with a type name is not rewritten",
			query:    "SELECT * FROM t WHERE interval_col = $1",
			expected: "SELECT * FROM t WHERE interval_col = $1",
		},
		{
			name:     "already valid query is returned unchanged",
			query:    "SELECT a, b FROM t WHERE id = $1 ORDER BY b DESC",
			expected: "SELECT a, b FROM t WHERE id = $1 ORDER BY b DESC",
		},
		{
			name:     "date_part with a parameter is already valid and left alone",
			query:    "SELECT date_part($1, order_date)",
			expected: "SELECT date_part($1, order_date)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, repairNormalizedQuery(tc.query))
		})
	}
}
