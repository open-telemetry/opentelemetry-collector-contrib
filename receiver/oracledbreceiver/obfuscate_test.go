// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObfuscateSQL(t *testing.T) {
	// ObfuscateAndNormalize collapses whitespace, strips comments and the
	// trailing semicolon, and puts spaces around punctuation so the output is a
	// canonical, format-independent representation of the query.
	expected := `SELECT e.employee_id, e.first_name, e.last_name, e.department_id, s.salary, d.department_name FROM employees e INNER JOIN ( SELECT department_id FROM employees GROUP BY department_id HAVING COUNT ( employee_id ) > ? ) AS subquery ON e.department_id = subquery.department_id INNER JOIN salaries s ON e.employee_id = s.employee_id INNER JOIN departments d ON e.department_id = d.department_id WHERE s.salary > ? AND d.department_name LIKE ? ORDER BY e.salary DESC`

	origin := `SELECT e.employee_id, e.first_name, e.last_name, e.department_id, s.salary, d.department_name
                 FROM employees e
                 INNER JOIN
                    ( SELECT department_id FROM employees GROUP BY department_id HAVING COUNT(employee_id) > 10) AS subquery
                      ON e.department_id = subquery.department_id
                 INNER JOIN
                     salaries s ON e.employee_id = s.employee_id
                 INNER JOIN
                    departments d ON e.department_id = d.department_id
                 WHERE s.salary > 50000
                    AND d.department_name LIKE 'IT%'
                 ORDER BY e.salary DESC;`
	obf := newObfuscator()
	result, err := obf.obfuscateSQLString(origin)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

// TestObfuscateSQLIsFormatInvariant is the core regression test for the query
// signature divergence bug: the same logical statement submitted with different
// whitespace (and comments) must obfuscate to a single canonical string so that
// downstream signatures are stable. The previous obfuscate_only mode preserved
// the original formatting and therefore produced different output per formatting.
func TestObfuscateSQLIsFormatInvariant(t *testing.T) {
	tests := []struct {
		name     string
		variants []string
	}{
		{
			name: "CTE with different whitespace",
			variants: []string{
				`WITH total_bytes AS (SELECT SUM(bytes) AS total FROM dba_data_files) SELECT (total - (SELECT SUM(bytes) FROM dba_free_space)) AS USED_DB_SIZE, total AS ALLOCATED_DB_SIZE FROM total_bytes`,
				"WITH total_bytes AS (\n    SELECT SUM(bytes) AS total FROM dba_data_files\n)\nSELECT\n    (total - (SELECT SUM(bytes) FROM dba_free_space)) AS USED_DB_SIZE,\n    total AS ALLOCATED_DB_SIZE\nFROM total_bytes",
			},
		},
		{
			name: "same query with and without comments",
			variants: []string{
				`SELECT id FROM users WHERE age > 18`,
				"/* app:web */ SELECT id FROM users WHERE age > 18 -- trailing comment",
				"SELECT id\nFROM users\n/* mid */ WHERE age > 18",
			},
		},
	}

	obf := newObfuscator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotEmpty(t, tt.variants)
			first, err := obf.obfuscateSQLString(tt.variants[0])
			require.NoError(t, err)
			for i, v := range tt.variants {
				got, err := obf.obfuscateSQLString(v)
				require.NoError(t, err)
				assert.Equal(t, first, got, "variant %d must produce the same canonical text", i)
			}
		})
	}
}

func TestObfuscateSQLWithComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "multiline comment at start",
			input: `/* Fetching active admin profiles */
SELECT * FROM profiles
WHERE role = 'admin' AND structural_id = 9954`,
			expected: `SELECT * FROM profiles WHERE role = ? AND structural_id = ?`,
		},
		{
			name: "inline comment at end",
			input: `SELECT * FROM profiles
WHERE role = 'admin' AND structural_id = 9954 -- Verification filter`,
			expected: `SELECT * FROM profiles WHERE role = ? AND structural_id = ?`,
		},
		{
			name: "multiple comments mixed",
			input: `/* Query for active users */
SELECT * FROM users
WHERE status = 'active' -- Only active users
AND age > 18 /* Adults only */`,
			expected: `SELECT * FROM users WHERE status = ? AND age > ?`,
		},
		{
			name: "comment in middle of query",
			input: `SELECT * FROM employees
/* Get high earners */
WHERE salary > 100000`,
			expected: `SELECT * FROM employees WHERE salary > ?`,
		},
		{
			name: "hash comment style",
			input: `SELECT * FROM orders
WHERE order_date > '2024-01-01' # Recent orders only`,
			expected: `SELECT * FROM orders WHERE order_date > ? # Recent orders only`, // Hash comments are not recognized as comments
		},
		{
			name:     "comment with query hints",
			input:    `SELECT /*+ INDEX(emp emp_idx) */ * FROM employees WHERE dept_id = 10`,
			expected: `SELECT * FROM employees WHERE dept_id = ?`,
		},
	}

	obf := newObfuscator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := obf.obfuscateSQLString(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result, "Comments should be stripped during normalization")
		})
	}
}

func TestObfuscateSQLWithAliases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "column and table aliases",
			input:    `SELECT u.name AS user_name, u.email AS user_email FROM users AS u WHERE u.id = 100`,
			expected: `SELECT u.name AS user_name, u.email AS user_email FROM users AS u WHERE u.id = ?`,
		},
		{
			name:     "subquery with alias",
			input:    `SELECT * FROM (SELECT id, name FROM users WHERE active = 'Y') AS active_users`,
			expected: `SELECT * FROM ( SELECT id, name FROM users WHERE active = ? ) AS active_users`,
		},
	}

	obf := newObfuscator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := obf.obfuscateSQLString(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result, "Aliases should be preserved")
		})
	}
}

func TestObfuscateSQLWithQuotedIdentifiers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "mixed quoted and unquoted",
			input:    `SELECT name, "Address", age FROM users WHERE id = 456`,
			expected: `SELECT name, "Address", age FROM users WHERE id = ?`,
		},
		{
			name:     "schema qualified quoted identifiers",
			input:    `SELECT * FROM ADMIN."Employee" WHERE ADMIN."Department"."DeptID" = 10`,
			expected: `SELECT * FROM ADMIN."Employee" WHERE ADMIN."Department"."DeptID" = ?`,
		},
	}

	obf := newObfuscator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := obf.obfuscateSQLString(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result, "KeepIdentifierQuotation retains identifier quotes")
		})
	}
}

// TestObfuscateSQLQuotedIdentifierCollision verifies that KeepIdentifierQuotation
// keeps a quoted identifier (a single column named "a b") distinct from an
// unquoted "a b" (column a aliased as b). Without KeepIdentifierQuotation both
// would normalize to `SELECT a b FROM t` and collide into one signature.
func TestObfuscateSQLQuotedIdentifierCollision(t *testing.T) {
	obf := newObfuscator()

	quoted, err := obf.obfuscateSQLString(`SELECT "a b" FROM t`)
	require.NoError(t, err)
	assert.Equal(t, `SELECT "a b" FROM t`, quoted)

	unquoted, err := obf.obfuscateSQLString(`SELECT a b FROM t`)
	require.NoError(t, err)
	assert.Equal(t, `SELECT a b FROM t`, unquoted)

	assert.NotEqual(t, quoted, unquoted, "quoted and unquoted identifiers must not collide")
}

func TestObfuscateSQLWithSpecialValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NULL, TRUE, and FALSE preserved while literals are obfuscated",
			input:    `SELECT * FROM records WHERE enabled = TRUE AND active = FALSE AND notes IS NULL AND count = 100`,
			expected: `SELECT * FROM records WHERE enabled = TRUE AND active = FALSE AND notes IS NULL AND count = ?`,
		},
	}

	obf := newObfuscator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := obf.obfuscateSQLString(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result, "Special values (NULL, TRUE, FALSE) should be preserved")
		})
	}
}

func TestObfuscateSQLComplexQueries(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "complex join with comments and aliases",
			input: `/* Get employee details */
SELECT
    e.id AS employee_id,
    e.name AS employee_name,
    d.name AS dept_name
FROM employees e -- employee table
INNER JOIN departments d ON e.dept_id = d.id
WHERE e.salary > 50000 -- high earners
AND d.location = 'NYC'`,
			expected: `SELECT e.id AS employee_id, e.name AS employee_name, d.name AS dept_name FROM employees e INNER JOIN departments d ON e.dept_id = d.id WHERE e.salary > ? AND d.location = ?`,
		},
		{
			name: "nested subqueries with formatting",
			input: `SELECT COUNT(*) AS total
FROM (
    SELECT id FROM users
    WHERE status = 'active'
    AND created_at > '2024-01-01'
) AS active_users`,
			expected: `SELECT COUNT ( * ) AS total FROM ( SELECT id FROM users WHERE status = ? AND created_at > ? ) AS active_users`,
		},
	}

	obf := newObfuscator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := obf.obfuscateSQLString(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result, "Complex queries should be normalized with literals obfuscated")
		})
	}
}
