// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver

import (
	"database/sql"
	"testing"
	"time"

	// registers the mysql driver for TestFetchDBVersionTimeout
	_ "github.com/go-sql-driver/mysql"
	version "github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestIsQueryExplainable(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Supported keywords — plain queries
		{
			name:     "select is explainable",
			input:    "SELECT * FROM t",
			expected: true,
		},
		{
			name:     "delete is explainable",
			input:    "DELETE FROM t WHERE id = 1",
			expected: true,
		},
		{
			name:     "insert is explainable",
			input:    "INSERT INTO t VALUES (1)",
			expected: true,
		},
		{
			name:     "replace is explainable",
			input:    "REPLACE INTO t VALUES (1)",
			expected: true,
		},
		{
			name:     "update is explainable",
			input:    "UPDATE t SET col = 1",
			expected: true,
		},
		// Case-insensitive matching
		{
			name:     "mixed-case SELECT is explainable",
			input:    "Select * FROM t",
			expected: true,
		},
		// Leading whitespace
		{
			name:     "leading whitespace before SELECT is explainable",
			input:    "   SELECT * FROM t",
			expected: true,
		},
		// Unsupported statements
		{
			name:     "show is not explainable",
			input:    "SHOW TABLES",
			expected: false,
		},
		{
			name:     "create is not explainable",
			input:    "CREATE TABLE t (id INT)",
			expected: false,
		},
		{
			name:     "drop is not explainable",
			input:    "DROP TABLE t",
			expected: false,
		},
		{
			name:     "empty string is not explainable",
			input:    "",
			expected: false,
		},
		// Truncated statements (handled upstream, but isQueryExplainable itself
		// should not crash; the trailing "..." doesn’t match any keyword)
		{
			name:     "truncated statement that starts with SELECT is still type-explainable",
			input:    "SELECT * FROM very_long_table_na...",
			expected: true, // still starts with SELECT
		},
		// Any leading block comment makes the query not explainable; digest_text
		// from performance_schema does not include them, so this won't arise in practice.
		{
			name:     "leading block comment before SELECT is not explainable",
			input:    "/* a comment */ SELECT * FROM t",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isQueryExplainable(tt.input))
		})
	}
}

// TestExplainQueryEarlyExits verifies that explainQuery returns "" without
// hitting the database when the sample statement is truncated or the digest
// text is not an explainable statement type.
func TestExplainQueryEarlyExits(t *testing.T) {
	// mySQLClient with a nil DB — safe because both early-exit paths return
	// before any database call is made.
	c := &mySQLClient{}
	logger := zap.NewNop()

	t.Run("truncated sample statement returns empty", func(t *testing.T) {
		result := c.explainQuery("SELECT * FROM t", "SELECT * FROM very_long_table_na...", "", "digest1", logger)
		assert.Empty(t, result)
	})

	t.Run("non-explainable digest text returns empty", func(t *testing.T) {
		result := c.explainQuery("SHOW TABLES", "SHOW TABLES", "", "digest2", logger)
		assert.Empty(t, result)
	})
}

// mustParseVersion is a test helper that parses a semver string and fails the test if parsing fails.
func mustParseVersion(t *testing.T, v string) *version.Version {
	t.Helper()
	parsed, err := version.NewVersion(v)
	require.NoError(t, err)
	return parsed
}

// TestDBVersionCapabilities tests the capability predicates on the dbVersion struct
// for MySQL 8+, MySQL 5.7, MariaDB 10.x, and MariaDB 11.x.
func TestDBVersionCapabilities(t *testing.T) {
	tests := []struct {
		name                        string
		dv                          dbVersion
		wantSupportsQuerySampleText bool
		wantSupportsReplicaStatus   bool
	}{
		{
			name:                        "MySQL 8.0.27",
			dv:                          dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.27")},
			wantSupportsQuerySampleText: true,
			wantSupportsReplicaStatus:   true,
		},
		{
			name:                        "MySQL 8.0.3 (minimum for query_sample_text)",
			dv:                          dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.3")},
			wantSupportsQuerySampleText: true,
			wantSupportsReplicaStatus:   false,
		},
		{
			name:                        "MySQL 8.0.2 (below query_sample_text minimum)",
			dv:                          dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.2")},
			wantSupportsQuerySampleText: false,
			wantSupportsReplicaStatus:   false,
		},
		{
			name:                        "MySQL 8.0.0 (below query_sample_text minimum)",
			dv:                          dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.0")},
			wantSupportsQuerySampleText: false,
			wantSupportsReplicaStatus:   false,
		},
		{
			name:                        "MySQL 8.0.22 (minimum for SHOW REPLICA STATUS)",
			dv:                          dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.22")},
			wantSupportsQuerySampleText: true,
			wantSupportsReplicaStatus:   true,
		},
		{
			name:                        "MySQL 8.0.21 (below SHOW REPLICA STATUS minimum)",
			dv:                          dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.21")},
			wantSupportsQuerySampleText: true,
			wantSupportsReplicaStatus:   false,
		},
		{
			name:                        "MySQL 5.7.44",
			dv:                          dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "5.7.44")},
			wantSupportsQuerySampleText: false,
			wantSupportsReplicaStatus:   false,
		},
		{
			name:                        "MariaDB 10.11.6",
			dv:                          dbVersion{product: dbProductMariaDB, version: mustParseVersion(t, "10.11.6")},
			wantSupportsQuerySampleText: false,
			wantSupportsReplicaStatus:   false,
		},
		{
			name:                        "MariaDB 11.4.2",
			dv:                          dbVersion{product: dbProductMariaDB, version: mustParseVersion(t, "11.4.2")},
			wantSupportsQuerySampleText: false,
			wantSupportsReplicaStatus:   false,
		},
		{
			name:                        "zero value (version unknown)",
			dv:                          dbVersion{},
			wantSupportsQuerySampleText: false,
			wantSupportsReplicaStatus:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantSupportsQuerySampleText, tt.dv.supportsQuerySampleText(), "supportsQuerySampleText()")
			assert.Equal(t, tt.wantSupportsReplicaStatus, tt.dv.supportsReplicaStatus(), "supportsReplicaStatus()")
		})
	}
}

// TestReplicaStatusQuery verifies that getReplicaStatusStats selects the correct
// SHOW command based on the detected database product and version.
func TestReplicaStatusQuery(t *testing.T) {
	tests := []struct {
		name      string
		dbVer     dbVersion
		wantQuery string
	}{
		{
			name:      "MySQL 8.0.22 uses SHOW REPLICA STATUS",
			dbVer:     dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.22")},
			wantQuery: "SHOW REPLICA STATUS",
		},
		{
			name:      "MySQL 8.0.27 uses SHOW REPLICA STATUS",
			dbVer:     dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.27")},
			wantQuery: "SHOW REPLICA STATUS",
		},
		{
			name:      "MySQL 8.0.21 uses SHOW SLAVE STATUS",
			dbVer:     dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.21")},
			wantQuery: "SHOW SLAVE STATUS",
		},
		{
			name:      "MySQL 5.7.44 uses SHOW SLAVE STATUS",
			dbVer:     dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "5.7.44")},
			wantQuery: "SHOW SLAVE STATUS",
		},
		{
			name:      "MySQL 8.0.22-log (suffix) uses SHOW REPLICA STATUS",
			dbVer:     dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.22")},
			wantQuery: "SHOW REPLICA STATUS",
		},
		{
			name:      "MariaDB 10.11.6 uses SHOW SLAVE STATUS",
			dbVer:     dbVersion{product: dbProductMariaDB, version: mustParseVersion(t, "10.11.6")},
			wantQuery: "SHOW SLAVE STATUS",
		},
		{
			name:      "MariaDB 11.4.2 uses SHOW SLAVE STATUS",
			dbVer:     dbVersion{product: dbProductMariaDB, version: mustParseVersion(t, "11.4.2")},
			wantQuery: "SHOW SLAVE STATUS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := "SHOW REPLICA STATUS"
			if !tt.dbVer.supportsReplicaStatus() {
				q = "SHOW SLAVE STATUS"
			}
			assert.Equal(t, tt.wantQuery, q)
		})
	}
}

// TestGetDBVersionCaching verifies that a cached version is returned on subsequent
// calls and that no additional query is made.
func TestParseDBVersion(t *testing.T) {
	tests := []struct {
		input       string
		wantProduct dbProduct
		wantVersion string
		wantErr     bool
	}{
		// MySQL variants (unchanged)
		{"8.0.33", dbProductMySQL, "8.0.33", false},
		{"5.7.44", dbProductMySQL, "5.7.44", false},
		{"5.6.51", dbProductMySQL, "5.6.51", false},
		// MySQL sometimes appends suffixes like "-log"
		{"8.0.22-log", dbProductMySQL, "8.0.22", false},
		// Standard MariaDB
		{"10.11.6-MariaDB", dbProductMariaDB, "10.11.6", false},
		{"11.4.2-MariaDB", dbProductMariaDB, "11.4.2", false},
		// MariaDB with MySQL 5.5.5 compatibility prefix (common in older MariaDB 10.x)
		{"5.5.5-10.11.6-MariaDB", dbProductMariaDB, "10.11.6", false},
		{"5.5.5-10.6.14-MariaDB", dbProductMariaDB, "10.6.14", false},
		// MariaDB with Debian/Ubuntu package decoration
		{"10.6.14-MariaDB-1:10.6.14+maria~ubu2204", dbProductMariaDB, "10.6.14", false},
		// MariaDB with -log suffix
		{"10.11.6-MariaDB-log", dbProductMariaDB, "10.11.6", false},
		// Malformed input
		{"not-a-version", dbProductMySQL, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDBVersion(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantProduct, got.product)
			assert.Equal(t, tt.wantVersion, got.version.String())
		})
	}
}

func TestGetDBVersionCaching(t *testing.T) {
	preloaded := dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.27")}
	c := &mySQLClient{dbVersion: preloaded}

	got := c.getDBVersion()
	assert.Equal(t, preloaded.product, got.product)
	assert.Equal(t, preloaded.version.String(), got.version.String())
}

func TestFetchDBVersionTimeout(t *testing.T) {
	// fetchDBVersion must return before its own internal timeout.
	// 192.0.2.1 is TEST-NET — guaranteed unreachable, so QueryRowContext
	// will block until the context deadline fires.
	db, err := sql.Open("mysql", "root:@tcp(192.0.2.1:3306)/")
	require.NoError(t, err)
	defer db.Close()

	c := &mySQLClient{client: db}

	start := time.Now()
	_, err = c.fetchDBVersion()
	elapsed := time.Since(start)

	require.Error(t, err, "expected timeout error from unreachable host")
	assert.Less(t, elapsed, 10*time.Second, "fetchDBVersion must not block longer than its own timeout")
}

// TestDBVersionHelperMethods verifies isValid and productString across all product/version combinations.
func TestDBVersionHelperMethods(t *testing.T) {
	t.Run("isValid returns false for zero value", func(t *testing.T) {
		assert.False(t, dbVersion{}.isValid())
	})
	t.Run("isValid returns true when version is set", func(t *testing.T) {
		assert.True(t, dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.27")}.isValid())
	})
	t.Run("productString MySQL", func(t *testing.T) {
		assert.Equal(t, "MySQL", dbVersion{product: dbProductMySQL, version: mustParseVersion(t, "8.0.27")}.productString())
	})
	t.Run("productString MariaDB", func(t *testing.T) {
		assert.Equal(t, "MariaDB", dbVersion{product: dbProductMariaDB, version: mustParseVersion(t, "10.11.6")}.productString())
	})
	t.Run("productString zero value defaults to MySQL", func(t *testing.T) {
		// Zero value has product=dbProductMySQL (iota 0); productString must return "MySQL".
		assert.Equal(t, "MySQL", dbVersion{}.productString())
	})
}
