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

func TestQuoteDatabaseList(t *testing.T) {
	tests := []struct {
		name      string
		databases []string
		expected  string
	}{
		{name: "nil", databases: nil, expected: ""},
		{name: "empty", databases: []string{}, expected: ""},
		{name: "single", databases: []string{"rdsadmin"}, expected: "'rdsadmin'"},
		{name: "multiple preserves order", databases: []string{"b", "a"}, expected: "'b','a'"},
		{name: "doubles embedded single quotes", databases: []string{"o'brien"}, expected: "'o''brien'"},
		{name: "backslashes use E-escaped form", databases: []string{`db\name`}, expected: ` E'db\\name'`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, quoteDatabaseList(tc.databases))
		})
	}
}

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

func TestGetTableCount(t *testing.T) {
	const countSQLPre14 = `SELECT count(*) FROM pg_class
WHERE relkind IN ('r', 'm')
AND relnamespace NOT IN (
    SELECT oid FROM pg_namespace
    WHERE nspname = 'pg_catalog' OR nspname = 'information_schema' OR nspname ~ '^pg_toast'
);`
	const countSQLPost14 = `SELECT count(*) FROM pg_class
WHERE relkind IN ('r', 'm', 'p')
AND relnamespace NOT IN (
    SELECT oid FROM pg_namespace
    WHERE nspname = 'pg_catalog' OR nspname = 'information_schema' OR nspname ~ '^pg_toast'
);`

	tests := []struct {
		name          string
		serverVersion string
		expectedSQL   string
		count         int64
		versionErr    error
		queryErr      error
		wantErr       bool
	}{
		{
			name:          "PostgreSQL 13 excludes partitioned parents",
			serverVersion: "13.4",
			expectedSQL:   countSQLPre14,
			count:         5,
		},
		{
			name:          "PostgreSQL 14 includes partitioned parents",
			serverVersion: "14.0",
			expectedSQL:   countSQLPost14,
			count:         6,
		},
		{
			name:          "PostgreSQL 17 includes partitioned parents",
			serverVersion: "17.2",
			expectedSQL:   countSQLPost14,
			count:         6,
		},
		{
			name:          "error resolving server version",
			serverVersion: "",
			versionErr:    errors.New("connection reset"),
			wantErr:       true,
		},
		{
			name:          "unparsable server version",
			serverVersion: "not-a-version",
			wantErr:       true,
		},
		{
			name:          "count query fails",
			serverVersion: "16.0",
			expectedSQL:   countSQLPost14,
			queryErr:      errors.New("statement timeout"),
			wantErr:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			client := &postgreSQLClient{client: db, closeFn: func() error { return nil }}

			if tc.versionErr != nil {
				mock.ExpectQuery("SHOW server_version;").WillReturnError(tc.versionErr)
			} else {
				mock.ExpectQuery("SHOW server_version;").
					WillReturnRows(sqlmock.NewRows([]string{"server_version"}).AddRow(tc.serverVersion))
			}

			// No count query when the version fails to resolve or parse.
			if tc.versionErr == nil && tc.serverVersion != "not-a-version" {
				if tc.queryErr != nil {
					mock.ExpectQuery(tc.expectedSQL).WillReturnError(tc.queryErr)
				} else {
					mock.ExpectQuery(tc.expectedSQL).
						WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.count))
				}
			}

			count, err := client.getTableCount(t.Context())
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.count, count)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestLockQueries(t *testing.T) {
	// Both queries outer-join pg_class to keep non-relation targets, and split on
	// pg_locks.database so a lock is reported exactly once.
	const databaseLocksSQL = "SELECT COALESCE(relname, '') AS relation, mode, locktype,COUNT(*) " +
		"AS locks FROM pg_locks " +
		"LEFT JOIN pg_class ON pg_locks.relation = pg_class.oid " +
		"WHERE pg_locks.database = (SELECT oid FROM pg_database WHERE datname = current_database()) " +
		"GROUP BY relname, mode, locktype;"
	const serverScopedLocksSQL = "SELECT COALESCE(relname, '') AS relation, mode, locktype,COUNT(*) " +
		"AS locks FROM pg_locks " +
		"LEFT JOIN pg_class ON pg_locks.relation = pg_class.oid " +
		"WHERE pg_locks.database IS NULL " +
		"OR (pg_locks.database = 0 AND (pg_locks.relation IS NULL OR pg_class.relisshared)) " +
		"GROUP BY relname, mode, locktype;"

	columns := []string{"relation", "mode", "locktype", "locks"}

	tests := []struct {
		name        string
		serverScope bool
		rows        *sqlmock.Rows
		queryErr    error
		expected    []databaseLocks
		wantErr     bool
	}{
		{
			name: "database locks keep relation names and non-relation targets",
			rows: sqlmock.NewRows(columns).
				AddRow("pg_class", "AccessShareLock", "relation", 2).
				// COALESCE turns a non-relation target into an empty relation.
				AddRow("", "ExclusiveLock", "advisory", 1).
				AddRow("", "ShareUpdateExclusiveLock", "object", 1),
			expected: []databaseLocks{
				{relation: "pg_class", mode: "AccessShareLock", lockType: "relation", locks: 2},
				{relation: "", mode: "ExclusiveLock", lockType: "advisory", locks: 1},
				{relation: "", mode: "ShareUpdateExclusiveLock", lockType: "object", locks: 1},
			},
		},
		{
			name:        "server scoped locks report transaction id targets",
			serverScope: true,
			rows: sqlmock.NewRows(columns).
				AddRow("pg_database", "AccessShareLock", "relation", 1).
				AddRow("", "ExclusiveLock", "transactionid", 3).
				AddRow("", "ExclusiveLock", "virtualxid", 4),
			expected: []databaseLocks{
				{relation: "pg_database", mode: "AccessShareLock", lockType: "relation", locks: 1},
				{relation: "", mode: "ExclusiveLock", lockType: "transactionid", locks: 3},
				{relation: "", mode: "ExclusiveLock", lockType: "virtualxid", locks: 4},
			},
		},
		{
			name:        "distinct modes on the same lock type stay separate",
			serverScope: true,
			rows: sqlmock.NewRows(columns).
				AddRow("", "ExclusiveLock", "transactionid", 1).
				AddRow("", "ShareLock", "transactionid", 2),
			expected: []databaseLocks{
				{relation: "", mode: "ExclusiveLock", lockType: "transactionid", locks: 1},
				{relation: "", mode: "ShareLock", lockType: "transactionid", locks: 2},
			},
		},
		{
			name:     "query error is wrapped",
			queryErr: errors.New("permission denied for table pg_locks"),
			expected: nil,
			wantErr:  true,
		},
		{
			name:        "a NULL relation does not drop the remaining rows",
			serverScope: true,
			// COALESCE means the driver should never hand us a NULL relation, but a
			// scan failure must not discard the rows that did scan cleanly.
			rows: sqlmock.NewRows(columns).
				AddRow(nil, "ExclusiveLock", "transactionid", 1).
				AddRow("", "ExclusiveLock", "virtualxid", 2),
			expected: []databaseLocks{
				{relation: "", mode: "ExclusiveLock", lockType: "virtualxid", locks: 2},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			client := &postgreSQLClient{client: db, closeFn: func() error { return nil }}

			expectedSQL, queryLocks := databaseLocksSQL, client.getDatabaseLocks
			if tc.serverScope {
				expectedSQL, queryLocks = serverScopedLocksSQL, client.getServerScopedLocks
			}

			if tc.queryErr != nil {
				mock.ExpectQuery(expectedSQL).WillReturnError(tc.queryErr)
			} else {
				mock.ExpectQuery(expectedSQL).WillReturnRows(tc.rows)
			}

			locks, err := queryLocks(t.Context())
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.expected, locks)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
