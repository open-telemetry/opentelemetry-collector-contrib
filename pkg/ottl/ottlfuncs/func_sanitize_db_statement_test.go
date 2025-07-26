// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStringGetter struct {
	val string
}

func (g testStringGetter) Get(context.Context, any) (string, error) {
	return g.val, nil
}

func Test_sanitizeDBStatement(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple select with number",
			input:    "SELECT * FROM users WHERE id = 123",
			expected: "SELECT * FROM users WHERE id = ?",
		},
		{
			name:     "select with string literal",
			input:    "SELECT * FROM users WHERE name = 'john'",
			expected: "SELECT * FROM users WHERE name = ?",
		},
		{
			name:     "select with hex number",
			input:    "SELECT * FROM users WHERE flag = 0xFF",
			expected: "SELECT * FROM users WHERE flag = ?",
		},
		{
			name:     "select with IN clause",
			input:    "SELECT * FROM users WHERE id IN (1, 2, 3)",
			expected: "SELECT * FROM users WHERE id IN (?)",
		},
		{
			name:     "select with multiple spaces",
			input:    "SELECT   *   FROM   users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "select with dollar quoted string",
			input:    "SELECT * FROM users WHERE name = $$john$$",
			expected: "SELECT * FROM users WHERE name = ?",
		},
		{
			name:     "select with double quoted string",
			input:    "SELECT * FROM users WHERE name = \"john\"",
			expected: "SELECT * FROM users WHERE name = ?",
		},
		{
			name:     "select with double and single quoted string",
			input:    "SELECT * FROM users WHERE name = 'john' AND city = \"london\"",
			expected: "SELECT * FROM users WHERE name = ? AND city = ?",
		},
		{
			name:     "select with escaped single quotes",
			input:    "SELECT * FROM users WHERE name = 'O''Connor'",
			expected: "SELECT * FROM users WHERE name = ?",
		},
		{
			name:     "select with multiple conditions",
			input:    "SELECT * FROM users WHERE age > 21 AND balance = 100.50 AND name = 'john'",
			expected: "SELECT * FROM users WHERE age > ? AND balance = ? AND name = ?",
		},
		{
			name:     "select with subquery in IN clause",
			input:    "SELECT * FROM users WHERE department_id IN (SELECT id FROM departments WHERE budget > 1000000) AND salary > 50000",
			expected: "SELECT * FROM users WHERE department_id IN (?) AND salary > ?",
		},
		{
			name:     "select with nested subqueries",
			input:    "SELECT * FROM employees WHERE salary > (SELECT AVG(salary) FROM employees WHERE department_id = (SELECT id FROM departments WHERE name = 'Engineering'))",
			expected: "SELECT * FROM employees WHERE salary > (SELECT AVG(salary) FROM employees WHERE department_id = (SELECT id FROM departments WHERE name = ?))",
		},
		{
			name:     "redis select",
			input:    "HSET map password \"secret\"",
			expected: "HSET map password ?",
		},
		{
			name:     "redis set",
			input:    "SET mykey \"myvalue\"",
			expected: "SET mykey ?",
		},
		{
			name:     "redis get",
			input:    "GET mykey",
			expected: "GET mykey",
		},
		{
			name:     "redis hset",
			input:    "HSET myhash field1 \"value1\" field2 \"value2\"",
			expected: "HSET myhash field1 ? field2 ?",
		},
		{
			name:     "redis zadd",
			input:    "ZADD myset 1.0 \"member1\" 2.0 \"member2\"",
			expected: "ZADD myset ? ? ? ?",
		},
		{
			name:     "redis zrangebyscore",
			input:    "ZRANGEBYSCORE myset 1.0 2.0",
			expected: "ZRANGEBYSCORE myset ? ?",
		},
		{
			name:     "redis with hex number",
			input:    "SET mykey 0xFF",
			expected: "SET mykey ?",
		},
		{
			name:     "redis with dollar string",
			input:    "SET mykey $$myvalue$$",
			expected: "SET mykey ?",
		},
		{
			name:     "redis with multiple commands",
			input:    "MULTI\nSET key1 \"value1\"\nSET key2 \"value2\"\nEXEC",
			expected: "MULTI SET key1 ? SET key2 ? EXEC",
		},
		{
			name:     "redis with empty value",
			input:    "SET mykey \"\"",
			expected: "SET mykey ?",
		},
		{
			name:     "redis with special characters",
			input:    "SET my:key \"value with spaces\"",
			expected: "SET my:key ?",
		},
		{
			name:     "redis with multiple spaces",
			input:    "SET    mykey    \"myvalue\"   ",
			expected: "SET mykey ?",
		},
		{
			name:     "redis with comment",
			input:    "SET mykey \"myvalue\" -- this is a comment",
			expected: "SET mykey ? -- this is a comment",
		},
		{
			name:     "redis with multiline comment",
			input:    "SET mykey /* comment */ \"myvalue\"",
			expected: "SET mykey /* comment */ ?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			function := sanitizeDBStatement[any](testStringGetter{val: tt.input})

			result, err := function(context.Background(), nil)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_sanitizeDBStatement_error(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{
			name:        "empty string",
			input:       "",
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			function := sanitizeDBStatement[any](testStringGetter{val: tt.input})

			result, err := function(context.Background(), nil)
			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Empty(t, result)
			}
		})
	}
}
