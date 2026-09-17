// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"errors"
	"strings"
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

func TestRepairNormalizedQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "extract with parameter is rewritten to a function call",
			query:    "SELECT * FROM orders WHERE EXTRACT($1 FROM order_date) = $2",
			expected: "SELECT * FROM orders WHERE pg_catalog.extract($1, order_date) = $2",
		},
		{
			name:     "extract rewrite preserves surrounding parameter numbering",
			query:    "SELECT CAST($1 AS varchar(50)) FROM orders WHERE EXTRACT($2 FROM order_date) BETWEEN $3 AND $4",
			expected: "SELECT CAST($1 AS varchar(50)) FROM orders WHERE pg_catalog.extract($2, order_date) BETWEEN $3 AND $4",
		},
		{
			name:     "extract rewrite is balanced when the source is a nested call",
			query:    "SELECT EXTRACT($1 FROM greatest(a, b))",
			expected: "SELECT pg_catalog.extract($1, greatest(a, b))",
		},
		{
			name:     "extract rewrite tolerates extra whitespace",
			query:    "SELECT extract(  $1   FROM   order_date )",
			expected: "SELECT pg_catalog.extract($1, order_date )",
		},
		{
			name:     "extract rewrite is case insensitive",
			query:    "SELECT Extract($1 From order_date)",
			expected: "SELECT pg_catalog.extract($1, order_date)",
		},
		{
			name:     "multiple extracts are all rewritten",
			query:    "SELECT EXTRACT($1 FROM a), EXTRACT($2 FROM b)",
			expected: "SELECT pg_catalog.extract($1, a), pg_catalog.extract($2, b)",
		},
		{
			name:     "extract with a literal field is already valid and left alone",
			query:    "SELECT EXTRACT(YEAR FROM order_date)",
			expected: "SELECT EXTRACT(YEAR FROM order_date)",
		},
		{
			name:     "extract rewrite reaches a nested extract",
			query:    "SELECT EXTRACT($1 FROM EXTRACT($2 FROM x))",
			expected: "SELECT pg_catalog.extract($1, pg_catalog.extract($2, x))",
		},
		{
			name:     "typed interval literal is rewritten to a cast",
			query:    "SELECT now() - interval $1",
			expected: "SELECT now() - CAST($1 AS interval)",
		},
		{
			name:     "typed timestamp literal is rewritten to a cast",
			query:    "SELECT * FROM events WHERE created_at > timestamp $1",
			expected: "SELECT * FROM events WHERE created_at > CAST($1 AS timestamp)",
		},
		{
			name:     "typed timestamptz literal is rewritten to a cast",
			query:    "SELECT * FROM events WHERE created_at > timestamptz $1",
			expected: "SELECT * FROM events WHERE created_at > CAST($1 AS timestamptz)",
		},
		{
			name:     "typed date literal is rewritten to a cast",
			query:    "SELECT * FROM events WHERE created_at > date $1",
			expected: "SELECT * FROM events WHERE created_at > CAST($1 AS date)",
		},
		{
			name:     "typed time literal is rewritten to a cast",
			query:    "SELECT * FROM events WHERE created_at > time $1",
			expected: "SELECT * FROM events WHERE created_at > CAST($1 AS time)",
		},
		{
			name:     "typed timetz literal is rewritten to a cast",
			query:    "SELECT * FROM events WHERE started_at > timetz $1",
			expected: "SELECT * FROM events WHERE started_at > CAST($1 AS timetz)",
		},
		{
			name:     "timestamp is not truncated to time when both appear",
			query:    "SELECT * FROM t WHERE ts > timestamp $1 AND t > time $2",
			expected: "SELECT * FROM t WHERE ts > CAST($1 AS timestamp) AND t > CAST($2 AS time)",
		},
		{
			name:     "typed literal rewrite is case insensitive",
			query:    "SELECT now() - INTERVAL $1",
			expected: "SELECT now() - CAST($1 AS INTERVAL)",
		},
		{
			name:     "multi word timestamp with time zone is rewritten whole",
			query:    "SELECT * FROM t WHERE ts > timestamp with time zone $1",
			expected: "SELECT * FROM t WHERE ts > CAST($1 AS timestamp with time zone)",
		},
		{
			name:     "multi word timestamp without time zone is rewritten whole",
			query:    "SELECT * FROM t WHERE ts > timestamp without time zone $1",
			expected: "SELECT * FROM t WHERE ts > CAST($1 AS timestamp without time zone)",
		},
		{
			name:     "multi word double precision is rewritten whole",
			query:    "SELECT * FROM t WHERE d > double precision $1",
			expected: "SELECT * FROM t WHERE d > CAST($1 AS double precision)",
		},
		{
			name:     "non temporal typed literal is rewritten to a cast",
			query:    "SELECT * FROM t WHERE total > numeric $1",
			expected: "SELECT * FROM t WHERE total > CAST($1 AS numeric)",
		},
		{
			name:     "uuid typed literal is rewritten to a cast",
			query:    "SELECT * FROM t WHERE id = uuid $1",
			expected: "SELECT * FROM t WHERE id = CAST($1 AS uuid)",
		},
		{
			name:     "single interval field qualifier is kept inside the cast",
			query:    "SELECT now() - interval $1 day",
			expected: "SELECT now() - CAST($1 AS interval day)",
		},
		{
			name:     "interval field range qualifier is kept inside the cast",
			query:    "SELECT now() - interval $1 DAY TO SECOND",
			expected: "SELECT now() - CAST($1 AS interval DAY TO SECOND)",
		},
		{
			name:     "a word after the parameter that is not a field keyword is left outside the cast",
			query:    "SELECT * FROM t WHERE ts > timestamp $1 ORDER BY ts",
			expected: "SELECT * FROM t WHERE ts > CAST($1 AS timestamp) ORDER BY ts",
		},
		{
			name:     "both repairs apply to the same expression",
			query:    "SELECT EXTRACT($1 FROM timestamp $2)",
			expected: "SELECT pg_catalog.extract($1, CAST($2 AS timestamp))",
		},
		{
			name:     "AT TIME ZONE is already valid and left alone",
			query:    "SELECT now() AT TIME ZONE $1",
			expected: "SELECT now() AT TIME ZONE $1",
		},
		{
			name:     "quoted identifier that looks like extract is left alone",
			query:    `SELECT * FROM t WHERE "EXTRACT($1 FROM x)" = $2`,
			expected: `SELECT * FROM t WHERE "EXTRACT($1 FROM x)" = $2`,
		},
		{
			name:     "string literal that looks like extract is left alone",
			query:    "SELECT * FROM t WHERE msg = 'EXTRACT($1 FROM x)'",
			expected: "SELECT * FROM t WHERE msg = 'EXTRACT($1 FROM x)'",
		},
		{
			name:     "quoted identifier that looks like a typed literal is left alone",
			query:    `SELECT * FROM t WHERE "interval $1" = $2`,
			expected: `SELECT * FROM t WHERE "interval $1" = $2`,
		},
		{
			name:     "string literal that looks like a typed literal is left alone",
			query:    "SELECT * FROM t WHERE msg = 'interval $1'",
			expected: "SELECT * FROM t WHERE msg = 'interval $1'",
		},
		{
			name:     "type name deeper inside a string literal is left alone",
			query:    "SELECT * FROM t WHERE msg = 'waited an interval $1'",
			expected: "SELECT * FROM t WHERE msg = 'waited an interval $1'",
		},
		{
			name:     "type name deeper inside a quoted identifier is left alone",
			query:    `SELECT * FROM t WHERE "my interval $1" = $2`,
			expected: `SELECT * FROM t WHERE "my interval $1" = $2`,
		},
		{
			name:     "doubled quote inside a literal does not end the protected span",
			query:    "SELECT * FROM t WHERE msg = 'it''s an interval $1'",
			expected: "SELECT * FROM t WHERE msg = 'it''s an interval $1'",
		},
		{
			name:     "escaped quote in an E string does not end the protected span",
			query:    `SELECT * FROM t WHERE msg = E'x\' interval $1'`,
			expected: `SELECT * FROM t WHERE msg = E'x\' interval $1'`,
		},
		{
			name:     "type name inside a line comment is left alone",
			query:    "SELECT * FROM t WHERE ts > -- interval\n  $1",
			expected: "SELECT * FROM t WHERE ts > -- interval\n  $1",
		},
		{
			name:     "type name inside a block comment is left alone",
			query:    "SELECT /* interval $1 */ * FROM t WHERE id = $2",
			expected: "SELECT /* interval $1 */ * FROM t WHERE id = $2",
		},
		{
			name:     "type name inside a nested block comment is left alone",
			query:    "SELECT /* a /* interval $1 */ b */ * FROM t WHERE id = $2",
			expected: "SELECT /* a /* interval $1 */ b */ * FROM t WHERE id = $2",
		},
		{
			name:     "type name inside a dollar quoted string is left alone",
			query:    "SELECT $tag$ interval $1 $tag$ FROM t",
			expected: "SELECT $tag$ interval $1 $tag$ FROM t",
		},
		{
			name:     "type name inside an untagged dollar quoted string is left alone",
			query:    "SELECT $$ interval $1 $$ FROM t",
			expected: "SELECT $$ interval $1 $$ FROM t",
		},
		{
			name:     "a protected span does not suppress a later repair",
			query:    "SELECT * FROM t WHERE msg = 'interval $1' AND ts > timestamp $2",
			expected: "SELECT * FROM t WHERE msg = 'interval $1' AND ts > CAST($2 AS timestamp)",
		},
		{
			name:     "identifier prefixed with a type name is not rewritten",
			query:    "SELECT * FROM t WHERE interval_col = $1",
			expected: "SELECT * FROM t WHERE interval_col = $1",
		},
		{
			name: "a type modifier breaks the typed literal form and is left alone",
			// timestamp(3) $1 is still unparseable; the parenthesis means there is no whitespace
			// between the type name and the parameter, so no cast is inferred. Recorded as a
			// known limit rather than a rewrite that guesses at the modifier.
			query:    "SELECT * FROM t WHERE ts > timestamp(3) $1",
			expected: "SELECT * FROM t WHERE ts > timestamp(3) $1",
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

func TestProtectedSpans(t *testing.T) {
	tests := []struct {
		name string
		// query marks the expected spans inline: every byte covered by a protected span is
		// written as one of the marker runes below in want, and every other byte as a dot. That
		// keeps offsets readable next to the query instead of listing index pairs.
		query string
		want  string
	}{
		{
			name:  "no protected regions",
			query: "SELECT a FROM t WHERE id = $1",
			want:  ".............................",
		},
		{
			name:  "string literal",
			query: "SELECT 'abc' FROM t",
			want:  ".......XXXXX.......",
		},
		{
			name:  "doubled quote continues the literal",
			query: "SELECT 'it''s' FROM t",
			want:  ".......XXXXXXX.......",
		},
		{
			name:  "backslash escape only applies to an E string",
			query: `SELECT E'a\'b' , 'c'`,
			want:  "........XXXXXX...XXX",
		},
		{
			name:  "a word ending in e does not introduce an E string",
			query: `SELECT tare'a\'`,
			want:  "...........XXXX",
		},
		{
			name:  "quoted identifier",
			query: `SELECT "col" FROM t`,
			want:  ".......XXXXX.......",
		},
		{
			name:  "line comment stops at the newline",
			query: "SELECT a -- note\nFROM t",
			want:  ".........XXXXXXX.......",
		},
		{
			name:  "block comments nest",
			query: "SELECT /* a /* b */ c */ 1",
			want:  ".......XXXXXXXXXXXXXXXXX..",
		},
		{
			name:  "dollar quoted string",
			query: "SELECT $t$ a $t$ FROM x",
			want:  ".......XXXXXXXXX.......",
		},
		{
			name:  "a parameter placeholder does not open a dollar quote",
			query: "SELECT $1 FROM t WHERE b = $2",
			want:  ".............................",
		},
		{
			name:  "unterminated literal protects to the end",
			query: "SELECT 'abc FROM t",
			want:  ".......XXXXXXXXXXX",
		},
		{
			name:  "unterminated block comment protects to the end",
			query: "SELECT /* abc FROM t",
			want:  ".......XXXXXXXXXXXXX",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Len(t, tc.want, len(tc.query), "the want mask must be as long as the query")

			got := []byte(strings.Repeat(".", len(tc.query)))
			for _, span := range protectedSpans(tc.query) {
				for i := span[0]; i < span[1]; i++ {
					got[i] = 'X'
				}
			}
			assert.Equal(t, tc.want, string(got), "query: %s", tc.query)
		})
	}
}
