// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObfuscateSQL(t *testing.T) {
	// With the two-step approach:
	expected := `SELECT e.employee_id, e.first_name, e.last_name, e.department_id, s.salary, d.department_name
                 FROM employees e
                 INNER JOIN
                    ( SELECT department_id FROM employees GROUP BY department_id HAVING COUNT(employee_id) > ?) AS subquery
                      ON e.department_id = subquery.department_id
                 INNER JOIN
                     salaries s ON e.employee_id = s.employee_id
                 INNER JOIN
                    departments d ON e.department_id = d.department_id
                 WHERE s.salary > ?
                    AND d.department_name LIKE ?
                 ORDER BY e.salary DESC;`

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
			expected: `?
SELECT * FROM profiles
WHERE role = ? AND structural_id = ?`,
		},
		{
			name: "inline comment at end",
			input: `SELECT * FROM profiles
WHERE role = 'admin' AND structural_id = 9954 -- Verification filter`,
			expected: `SELECT * FROM profiles
WHERE role = ? AND structural_id = ? ?`,
		},
		{
			name: "multiple comments mixed",
			input: `/* Query for active users */
SELECT * FROM users
WHERE status = 'active' -- Only active users
AND age > 18 /* Adults only */`,
			expected: `?
SELECT * FROM users
WHERE status = ? ?
AND age > ? ?`,
		},
		{
			name: "comment in middle of query",
			input: `SELECT * FROM employees
/* Get high earners */
WHERE salary > 100000`,
			expected: `SELECT * FROM employees
?
WHERE salary > ?`,
		},
		{
			name: "hash comment style",
			input: `SELECT * FROM orders
WHERE order_date > '2024-01-01' # Recent orders only`,
			expected: `SELECT * FROM orders
WHERE order_date > ? # Recent orders only`, // Hash comments are not collected by obfuscator
		},
		{
			name: "nested comments",
			input: `/* Outer comment
   /* Inner comment */
   More outer comment */
SELECT col FROM table WHERE id = 123`,
			expected: `?
   More outer comment */
SELECT col FROM table WHERE id = ?`, // Nested comments: only first /* is collected
		},
		{
			name: "comment with special characters",
			input: `SELECT * FROM logs -- Filter: status='active' AND user_id=123
WHERE message LIKE '%error%'`,
			expected: `SELECT * FROM logs ?
WHERE message LIKE ?`,
		},
		{
			name: "multiple inline comments",
			input: `SELECT
    col1, -- first column
    col2, -- second column
    col3  -- third column
FROM users WHERE age > 21`,
			expected: `SELECT
    col1, ?
    col2, ?
    col3  ?
FROM users WHERE age > ?`,
		},
		{
			name:     "comment with query hints",
			input:    `SELECT /*+ INDEX(emp emp_idx) */ * FROM employees WHERE dept_id = 10`,
			expected: `SELECT ? * FROM employees WHERE dept_id = ?`,
		},
		{
			name: "multiline comment with SQL inside",
			input: `/* This is a test query
   Original: SELECT * FROM users WHERE id = 100
   Modified below */
SELECT * FROM users WHERE status = 'active'`,
			expected: `?
SELECT * FROM users WHERE status = ?`,
		},
	}

	obf := newObfuscator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := obf.obfuscateSQLString(tt.input)
			assert.NoError(t, err)
			t.Logf("\n=== Input ===\n%s\n=== Output ===\n%s\n=== Expected ===\n%s", tt.input, result, tt.expected)
			assert.Equal(t, tt.expected, result, "Comments should be replaced with ? during obfuscation")
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
			name:     "simple AS alias",
			input:    `SELECT COUNT(*) AS total FROM users WHERE age > 25`,
			expected: `SELECT COUNT(*) AS total FROM users WHERE age > ?`,
		},
		{
			name:     "multiple aliases",
			input:    `SELECT u.name AS user_name, u.email AS user_email FROM users u WHERE u.id = 100`,
			expected: `SELECT u.name AS user_name, u.email AS user_email FROM users u WHERE u.id = ?`,
		},
		{
			name:     "table alias",
			input:    `SELECT e.* FROM employees AS e WHERE e.salary > 50000`,
			expected: `SELECT e.* FROM employees AS e WHERE e.salary > ?`, // obfuscate_only preserves original formatting
		},
		{
			name:     "subquery with alias",
			input:    `SELECT * FROM (SELECT id, name FROM users WHERE active = 'Y') AS active_users`,
			expected: `SELECT * FROM (SELECT id, name FROM users WHERE active = ?) AS active_users`,
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
			name:     "double quoted table name",
			input:    `SELECT * FROM "Employee" WHERE "EmployeeID" = 123`,
			expected: `SELECT * FROM "Employee" WHERE "EmployeeID" = ?`, // obfuscate_only preserves quoted identifiers
		},
		{
			name:     "mixed quoted and unquoted",
			input:    `SELECT name, "Address", age FROM users WHERE id = 456`,
			expected: `SELECT name, "Address", age FROM users WHERE id = ?`, // obfuscate_only preserves quoted identifiers
		},
		{
			name:     "schema qualified quoted identifiers",
			input:    `SELECT * FROM ADMIN."Employee" WHERE ADMIN."Department"."DeptID" = 10`,
			expected: `SELECT * FROM ADMIN."Employee" WHERE ADMIN."Department"."DeptID" = ?`, // Quoted identifiers are preserved
		},
	}

	obf := newObfuscator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := obf.obfuscateSQLString(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result, "obfuscate_only mode preserves quoted identifiers")
		})
	}
}

func TestObfuscateSQLWithSpecialValues(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NULL values preserved",
			input:    `SELECT * FROM users WHERE deleted_at IS NULL AND status = 'active'`,
			expected: `SELECT * FROM users WHERE deleted_at IS NULL AND status = ?`,
		},
		{
			name:     "boolean TRUE preserved",
			input:    `SELECT * FROM settings WHERE enabled = TRUE AND value = 'test'`,
			expected: `SELECT * FROM settings WHERE enabled = TRUE AND value = ?`,
		},
		{
			name:     "boolean FALSE preserved",
			input:    `SELECT * FROM flags WHERE active = FALSE AND priority = 1`,
			expected: `SELECT * FROM flags WHERE active = FALSE AND priority = ?`,
		},
		{
			name:     "mixed special values",
			input:    `SELECT * FROM records WHERE flag = TRUE AND notes IS NULL AND count = 100`,
			expected: `SELECT * FROM records WHERE flag = TRUE AND notes IS NULL AND count = ?`,
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
			expected: `?
SELECT
    e.id AS employee_id,
    e.name AS employee_name,
    d.name AS dept_name
FROM employees e ?
INNER JOIN departments d ON e.dept_id = d.id
WHERE e.salary > ? ?
AND d.location = ?`,
		},
		{
			name: "nested subqueries with formatting",
			input: `SELECT COUNT(*) AS total
FROM (
    SELECT id FROM users
    WHERE status = 'active'
    AND created_at > '2024-01-01'
) AS active_users`,
			expected: `SELECT COUNT(*) AS total
FROM (
    SELECT id FROM users
    WHERE status = ?
    AND created_at > ?
) AS active_users`,
		},
	}

	obf := newObfuscator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := obf.obfuscateSQLString(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result, "Complex queries should have comments replaced with ? and literals obfuscated")
		})
	}
}
