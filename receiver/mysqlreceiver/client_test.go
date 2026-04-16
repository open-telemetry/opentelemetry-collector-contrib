// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
