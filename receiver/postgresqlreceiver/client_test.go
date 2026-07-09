// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
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
