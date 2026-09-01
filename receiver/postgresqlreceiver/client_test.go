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

func TestGetBackends(t *testing.T) {
	const baseSQL = "SELECT datname, coalesce(backend_type, 'unknown') as backend_type, coalesce(state, 'unknown') as state, coalesce(wait_event_type, 'none') as wait_event_type, count(*) as count from pg_stat_activity"
	const groupBy = " GROUP BY datname, backend_type, state, wait_event_type;"
	columns := []string{"datname", "backend_type", "state", "wait_event_type", "count"}

	tests := []struct {
		name        string
		databases   []string
		expectedSQL string
		rows        *sqlmock.Rows
		queryErr    error
		expected    map[databaseName][]backendStateCount
		wantErr     bool
	}{
		{
			name:        "groups backends by backend type, state and wait event type",
			databases:   nil,
			expectedSQL: baseSQL + groupBy,
			rows: sqlmock.NewRows(columns).
				AddRow("otel", "client backend", "active", "none", 3).
				AddRow("otel", "client backend", "idle", "Client", 7).
				AddRow("otel", "autovacuum worker", "active", "none", 2).
				AddRow("telemetry", "client backend", "idle in transaction", "Lock", 1),
			expected: map[databaseName][]backendStateCount{
				"otel": {
					{backendType: "client backend", state: "active", waitEventType: "none", count: 3},
					{backendType: "client backend", state: "idle", waitEventType: "Client", count: 7},
					{backendType: "autovacuum worker", state: "active", waitEventType: "none", count: 2},
				},
				"telemetry": {
					{backendType: "client backend", state: "idle in transaction", waitEventType: "Lock", count: 1},
				},
			},
		},
		{
			name:        "filtered by database",
			databases:   []string{"otel"},
			expectedSQL: baseSQL + " WHERE datname IN ('otel')" + groupBy,
			rows: sqlmock.NewRows(columns).
				AddRow("otel", "client backend", "active", "none", 2),
			expected: map[databaseName][]backendStateCount{
				"otel": {{backendType: "client backend", state: "active", waitEventType: "none", count: 2}},
			},
		},
		{
			// The nullable columns are coalesced by the query itself, so a driver that returns the
			// coalesced values keeps those backends counted rather than dropping them. An unprivileged
			// monitoring user sees this for every backend but its own.
			name:        "coalesced null backend type, state and wait event type are counted",
			databases:   nil,
			expectedSQL: baseSQL + groupBy,
			rows: sqlmock.NewRows(columns).
				AddRow("otel", "unknown", "unknown", "none", 4),
			expected: map[databaseName][]backendStateCount{
				"otel": {{backendType: "unknown", state: "unknown", waitEventType: "none", count: 4}},
			},
		},
		{
			name:        "rows with empty datname are skipped",
			databases:   nil,
			expectedSQL: baseSQL + groupBy,
			rows: sqlmock.NewRows(columns).
				AddRow("", "client backend", "active", "none", 9).
				AddRow("otel", "client backend", "active", "none", 1),
			expected: map[databaseName][]backendStateCount{
				"otel": {{backendType: "client backend", state: "active", waitEventType: "none", count: 1}},
			},
		},
		{
			name:        "query error",
			databases:   nil,
			expectedSQL: baseSQL + groupBy,
			queryErr:    errors.New("permission denied for view pg_stat_activity"),
			expected:    nil,
			wantErr:     true,
		},
		{
			name:        "row scan error on non-numeric count",
			databases:   nil,
			expectedSQL: baseSQL + groupBy,
			rows: sqlmock.NewRows(columns).
				AddRow("otel", "client backend", "active", "none", "not-a-number"),
			expected: map[databaseName][]backendStateCount{},
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

			backends, err := client.getBackends(t.Context(), tc.databases)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expected, backends)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFilterQueryByDatabases(t *testing.T) {
	tests := []struct {
		name      string
		baseQuery string
		databases []string
		groupBy   []string
		expected  string
	}{
		{
			name:      "no databases and no group by",
			baseQuery: "SELECT datname FROM pg_stat_database",
			expected:  "SELECT datname FROM pg_stat_database;",
		},
		{
			name:      "no databases with group by",
			baseQuery: "SELECT datname, count(*) FROM pg_stat_activity",
			groupBy:   []string{"datname"},
			expected:  "SELECT datname, count(*) FROM pg_stat_activity GROUP BY datname;",
		},
		{
			name:      "single database",
			baseQuery: "SELECT datname FROM pg_stat_database",
			databases: []string{"otel"},
			expected:  "SELECT datname FROM pg_stat_database WHERE datname IN ('otel');",
		},
		{
			name:      "multiple databases with group by",
			baseQuery: "SELECT datname, count(*) FROM pg_stat_activity",
			databases: []string{"otel", "open"},
			groupBy:   []string{"datname"},
			expected:  "SELECT datname, count(*) FROM pg_stat_activity WHERE datname IN ('otel','open') GROUP BY datname;",
		},
		{
			name:      "existing WHERE clause is extended with AND",
			baseQuery: "SELECT datname FROM pg_catalog.pg_database WHERE datistemplate = false",
			databases: []string{"otel"},
			expected:  "SELECT datname FROM pg_catalog.pg_database WHERE datistemplate = false AND datname IN ('otel');",
		},
		{
			name:      "multi-column group by",
			baseQuery: "SELECT datname, state, count(*) FROM pg_stat_activity",
			databases: []string{"otel"},
			groupBy:   []string{"datname", "state"},
			expected:  "SELECT datname, state, count(*) FROM pg_stat_activity WHERE datname IN ('otel') GROUP BY datname, state;",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, filterQueryByDatabases(tc.baseQuery, tc.databases, tc.groupBy...))
		})
	}
}
