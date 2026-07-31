// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryContents(t *testing.T) {
	queryTests := []struct {
		name                     string
		instanceName             string
		getQuery                 func(string) string
		expectedQueryValFilename string
	}{
		{
			name:                     "Test database IO query without instance name",
			instanceName:             "",
			getQuery:                 getSQLServerDatabaseIOQuery,
			expectedQueryValFilename: "databaseIOQueryWithoutInstanceName.txt",
		},
		{
			name:                     "Test database IO query with instance name",
			instanceName:             "instanceName",
			getQuery:                 getSQLServerDatabaseIOQuery,
			expectedQueryValFilename: "databaseIOQueryWithInstanceName.txt",
		},
		{
			name:                     "Test perf counter query without instance name",
			instanceName:             "",
			getQuery:                 getSQLServerPerformanceCounterQuery,
			expectedQueryValFilename: "perfCounterQueryWithoutInstanceName.txt",
		},
		{
			name:                     "Test perf counter query with instance name",
			instanceName:             "instanceName",
			getQuery:                 getSQLServerPerformanceCounterQuery,
			expectedQueryValFilename: "perfCounterQueryWithInstanceName.txt",
		},
		{
			name:                     "Test properties query without instance name",
			instanceName:             "",
			getQuery:                 getSQLServerPropertiesQuery,
			expectedQueryValFilename: "propertyQueryWithoutInstanceName.txt",
		},
		{
			name:                     "Test properties query with instance name",
			instanceName:             "instanceName",
			getQuery:                 getSQLServerPropertiesQuery,
			expectedQueryValFilename: "propertyQueryWithInstanceName.txt",
		},
		{
			name:                     "Test wait stats query without instance name",
			instanceName:             "",
			getQuery:                 getSQLServerWaitStatsQuery,
			expectedQueryValFilename: "waitStatsQueryWithoutInstanceName.txt",
		},
		{
			name:                     "Test wait stats query with instance name",
			instanceName:             "instanceName",
			getQuery:                 getSQLServerWaitStatsQuery,
			expectedQueryValFilename: "waitStatsQueryWithInstanceName.txt",
		},
		{
			name:                     "Test worker threads query without instance name",
			instanceName:             "",
			getQuery:                 getSQLServerWorkerThreadsQuery,
			expectedQueryValFilename: "workerThreadsQueryWithoutInstanceName.txt",
		},
		{
			name:                     "Test worker threads query with instance name",
			instanceName:             "instanceName",
			getQuery:                 getSQLServerWorkerThreadsQuery,
			expectedQueryValFilename: "workerThreadsQueryWithInstanceName.txt",
		},
		{
			name:                     "Test index physical stats query without instance name",
			instanceName:             "",
			getQuery:                 getSQLServerIndexPhysicalStatsQuery,
			expectedQueryValFilename: "indexPhysicalQueryWithoutInstanceName.txt",
		},
		{
			name:                     "Test index physical stats query with instance name",
			instanceName:             "instanceName",
			getQuery:                 getSQLServerIndexPhysicalStatsQuery,
			expectedQueryValFilename: "indexPhysicalQueryWithInstanceName.txt",
		},
		{
			name:                     "Test availability group query without instance name",
			instanceName:             "",
			getQuery:                 getSQLServerAvailabilityGroupQuery,
			expectedQueryValFilename: "availabilityGroupQueryWithoutInstanceName.txt",
		},
		{
			name:                     "Test availability group query with instance name",
			instanceName:             "instanceName",
			getQuery:                 getSQLServerAvailabilityGroupQuery,
			expectedQueryValFilename: "availabilityGroupQueryWithInstanceName.txt",
		},
	}

	for _, tt := range queryTests {
		t.Run(tt.name, func(t *testing.T) {
			expectedBytes, err := os.ReadFile(path.Join("./testdata", tt.expectedQueryValFilename))
			require.NoError(t, err)
			// Replace all will fix newlines when testing on Windows
			expected := strings.ReplaceAll(string(expectedBytes), "\r\n", "\n")

			actual := tt.getQuery(tt.instanceName)
			require.Equal(t, expected, actual)
		})
	}
}

func TestQueryTextAndPlanQueryContents(t *testing.T) {
	queryTests := []struct {
		name                     string
		instanceName             string
		maxQuerySampleCount      uint
		lookbackTime             uint
		getQuery                 func() string
		expectedQueryValFilename string
	}{
		{
			name:                     "Test query text and query plan",
			instanceName:             "",
			maxQuerySampleCount:      1000,
			lookbackTime:             60,
			getQuery:                 getSQLServerQueryTextAndPlanQuery,
			expectedQueryValFilename: "databaseTopQueryWithoutInstanceName.txt",
		},
	}

	for _, tt := range queryTests {
		t.Run(tt.name, func(t *testing.T) {
			expected, err := os.ReadFile(path.Join("./testdata", tt.expectedQueryValFilename))
			require.NoError(t, err)
			actual := tt.getQuery()
			require.NoError(t, err)
			require.Equal(t, strings.TrimSpace(string(expected)), strings.TrimSpace(actual))
		})
	}
}

func TestGetSQLServerQuerySamplesQuery(t *testing.T) {
	queryTests := []struct {
		name                     string
		instanceName             string
		getQuery                 func() string
		expectedQueryValFilename string
		maxRowsPerQuery          uint64
	}{
		{
			name:                     "Test query sample query",
			instanceName:             "",
			maxRowsPerQuery:          1000,
			getQuery:                 getSQLServerQuerySamplesQuery,
			expectedQueryValFilename: "testQuerySampleQuery.txt",
		},
	}

	for _, tt := range queryTests {
		t.Run(tt.name, func(t *testing.T) {
			expectedBytes, err := os.ReadFile(path.Join("./testdata", tt.expectedQueryValFilename))
			require.NoError(t, err)
			// Replace all will fix newlines when testing on Windows
			expected := strings.ReplaceAll(string(expectedBytes), "\r\n", "\n")
			actual := strings.ReplaceAll(tt.getQuery(), "\r\n", "\n")
			require.Equal(t, expected, actual)
		})
	}
}

// TestQuerySampleQueryDetectsSchemaLockBlocking is a regression test for the class of
// bugs where sessions blocked on LCK_M_SCH_S / LCK_M_SCH_M (schema stability / modification
// locks) were silently dropped from the query sample results. The root cause was
// `CROSS APPLY sys.dm_exec_sql_text(r.plan_handle)`: for a session waiting on a schema
// lock, `sys.dm_exec_sql_text` returns zero rows, and `CROSS APPLY` therefore eliminates
// the outer row entirely, hiding the blocking session from downstream reporting.
//
// The fix uses `OUTER APPLY` (keeps the row when the TVF returns nothing) and adds
// `OUTER APPLY sys.dm_exec_input_buffer(...)` as a fallback source for statement_text.
//
// This test asserts the invariants that prevent the bug from being re-introduced.
// If a future change violates any of these, the failing assertion pinpoints the
// specific problem instead of just a diff error.
func TestQuerySampleQueryDetectsSchemaLockBlocking(t *testing.T) {
	query := getSQLServerQuerySamplesQuery()

	// Invariant 1: must not use CROSS APPLY on dm_exec_sql_text.
	// CROSS APPLY drops rows when the TVF returns zero rows, which is exactly what
	// happens for sessions blocked on LCK_M_SCH_S / LCK_M_SCH_M.
	require.NotContains(t, query, "CROSS APPLY sys.dm_exec_sql_text",
		"query sample must not use CROSS APPLY on sys.dm_exec_sql_text — it silently drops "+
			"sessions blocked on LCK_M_SCH_S / LCK_M_SCH_M. Use OUTER APPLY instead.")

	// Invariant 2: must use OUTER APPLY on dm_exec_sql_text.
	require.Contains(t, query, "OUTER APPLY sys.dm_exec_sql_text(r.plan_handle)",
		"query sample must use OUTER APPLY on sys.dm_exec_sql_text(r.plan_handle) so that "+
			"sessions with unresolvable plan_handle (e.g. blocked on schema locks) are still emitted.")

	// Invariant 3: must include sys.dm_exec_input_buffer as a fallback source of
	// statement_text. When plan_handle is unresolvable, dm_exec_input_buffer reads
	// directly from the client connection buffer and still returns the submitted SQL.
	require.Contains(t, query, "sys.dm_exec_input_buffer",
		"query sample must join sys.dm_exec_input_buffer to provide fallback statement_text "+
			"when plan_handle cannot be resolved.")

	// Invariant 4: WHERE clause must be NULL-safe for o.TEXT.
	// A bare `WHERE o.TEXT NOT LIKE ...` treats NULL as UNKNOWN and drops the row —
	// silently re-introducing the same class of bug for encrypted procs and other
	// cases where dm_exec_sql_text returns a row with NULL text.
	require.Contains(t, query, "o.TEXT IS NULL OR o.TEXT NOT LIKE",
		"query sample WHERE clause must be NULL-safe for o.TEXT so that rows with NULL "+
			"plan text (e.g. encrypted stored procedures) are not silently dropped.")

	// Invariant 5: WHERE clause must be NULL-safe for input_buffer text as well.
	require.Contains(t, query, "ib.event_info IS NULL OR ib.event_info NOT LIKE",
		"query sample WHERE clause must be NULL-safe for ib.event_info.")

	// Invariant 6: statement_text must prefer plan_handle-based extraction (which uses
	// statement_start_offset / statement_end_offset to isolate the *specific* statement
	// inside a multi-statement batch), and only fall back to input_buffer (which
	// returns the whole batch) when the plan_handle lookup failed. Using COALESCE
	// with SUBSTRING first, then ib.event_info, preserves statement-level granularity
	// for healthy sessions.
	require.Contains(t, query, "COALESCE(",
		"query sample must COALESCE plan_handle SUBSTRING with input_buffer fallback so "+
			"statement-level granularity is preserved for healthy sessions.")
	require.Contains(t, query, "ib.event_info,",
		"query sample must reference ib.event_info in the COALESCE fallback chain.")
}

func TestGetSQLServerIdleBlockingSessionsQuery(t *testing.T) {
	expectedBytes, err := os.ReadFile(path.Join(".", "testdata", "testIdleBlockerQuerySampleQuery.txt"))
	require.NoError(t, err)

	expected := strings.ReplaceAll(string(expectedBytes), "\r\n", "\n")
	actual := strings.ReplaceAll(getSQLServerIdleBlockingSessionsQuery(), "\r\n", "\n")
	require.Equal(t, expected, actual)
}

func TestFormatSQLServerSessionIDsParam(t *testing.T) {
	actual := formatSQLServerSessionIDsParam(map[int64]struct{}{
		91: {},
		60: {},
		77: {},
	})
	require.Equal(t, "60,77,91", actual)
}
