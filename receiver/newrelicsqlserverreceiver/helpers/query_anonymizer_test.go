package helpers
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0


import (
	"strings"
	"testing"
)

func TestAnonymizeQueryText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple SELECT with string literal",
			input:    "SELECT * FROM users WHERE name = 'John Doe'",
			expected: "SELECT * FROM users WHERE name = ?",
		},
		{
			name:     "SELECT with multiple literals",
			input:    "SELECT * FROM products WHERE price > 100 AND category = 'electronics'",
			expected: "SELECT * FROM products WHERE price > ? AND category = ?",
		},
		{
			name:     "Double quoted string",
			input:    `UPDATE products SET name = "New Product" WHERE id = 123`,
			expected: "UPDATE products SET name = ? WHERE id = ?",
		},
		{
			name:     "Mixed quote types",
			input:    `SELECT * FROM orders WHERE status = 'pending' AND total > 99.99 AND notes = "urgent"`,
			expected: "SELECT * FROM orders WHERE status = ? AND total > ?.? AND notes = ?",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "No literals to anonymize",
			input:    "SELECT column1, column2 FROM table1 WHERE column3 IS NOT NULL",
			expected: "SELECT column?, column? FROM table? WHERE column? IS NOT NULL",
		},
		{
			name:     "Multiple numbers",
			input:    "SELECT * FROM table WHERE id BETWEEN 10 AND 20 AND status = 1",
			expected: "SELECT * FROM table WHERE id BETWEEN ? AND ? AND status = ?",
		},
		{
			name:     "Complex query with subquery",
			input:    "SELECT t1.id FROM table1 t1 WHERE t1.value > (SELECT AVG(value) FROM table2 WHERE category = 'test')",
			expected: "SELECT t?.id FROM table? t? WHERE t?.value > (SELECT AVG(value) FROM table? WHERE category = ?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnonymizeQueryText(tt.input)
			if result != tt.expected {
				t.Errorf("AnonymizeQueryText() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestAnonymizeExecutionPlanXML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Simple StatementText with single quotes",
			input: `<StmtSimple StatementText='SELECT * FROM users WHERE id = 123' StatementId="1">`,
			expected: `<StmtSimple StatementText='SELECT * FROM users WHERE id = ?' StatementId="1">`,
		},
		{
			name: "StatementText with double quotes",
			input: `<StmtSimple StatementText="SELECT * FROM products WHERE price > 100" StatementId="1">`,
			expected: `<StmtSimple StatementText="SELECT * FROM products WHERE price > ?" StatementId="1">`,
		},
		{
			name: "Multiple StatementText attributes",
			input: `<Root><StmtSimple StatementText="SELECT name FROM users WHERE id = 1"/><StmtSimple StatementText='UPDATE products SET price = 99.99'/></Root>`,
			expected: `<Root><StmtSimple StatementText="SELECT name FROM users WHERE id = ?"/><StmtSimple StatementText='UPDATE products SET price = ?.?'/></Root>`,
		},
		{
			name: "No StatementText attributes",
			input: `<StmtSimple StatementId="1" StatementType="SELECT">`,
			expected: `<StmtSimple StatementId="1" StatementType="SELECT">`,
		},
		{
			name: "Empty StatementText",
			input: `<StmtSimple StatementText="" StatementId="1">`,
			expected: `<StmtSimple StatementText="" StatementId="1">`,
		},
		{
			name: "Complex nested XML with StatementText",
			input: `<ShowPlanXML><Batch><Statements><StmtSimple StatementText="SELECT * FROM orders WHERE total > 1000 AND status = 'pending'" StatementId="1"/></Statements></Batch></ShowPlanXML>`,
			expected: `<ShowPlanXML><Batch><Statements><StmtSimple StatementText="SELECT * FROM orders WHERE total > ? AND status = ?" StatementId="1"/></Statements></Batch></ShowPlanXML>`,
		},
		{
			name: "StatementText with whitespace around equals",
			input: `<StmtSimple StatementText  =  "SELECT * FROM users WHERE age = 25" StatementId="1">`,
			expected: `<StmtSimple StatementText  =  "SELECT * FROM users WHERE age = ?" StatementId="1">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnonymizeExecutionPlanXML(tt.input)
			if result != tt.expected {
				t.Errorf("AnonymizeExecutionPlanXML() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestSafeAnonymizeQueryText(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected string
	}{
		{
			name:     "Nil pointer",
			input:    nil,
			expected: "[nil query]",
		},
		{
			name:     "Empty string pointer",
			input:    stringPtr(""),
			expected: "[empty query]",
		},
		{
			name:     "Valid query",
			input:    stringPtr("SELECT * FROM users WHERE id = 123"),
			expected: "SELECT * FROM users WHERE id = ?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeAnonymizeQueryText(tt.input)
			if result != tt.expected {
				t.Errorf("SafeAnonymizeQueryText() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestIsQueryTextSafe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: true,
		},
		{
			name:     "Normal size query",
			input:    "SELECT * FROM users WHERE id = 123",
			expected: true,
		},
		{
			name:     "Large but safe query",
			input:    strings.Repeat("SELECT * FROM table WHERE column = 'value' AND ", 100),
			expected: true,
		},
		{
			name:     "Too large query",
			input:    strings.Repeat("x", 20000), // Over 16KB limit
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsQueryTextSafe(tt.input)
			if result != tt.expected {
				t.Errorf("IsQueryTextSafe() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestIsXMLContentSafe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Empty XML",
			input:    "",
			expected: true,
		},
		{
			name:     "Normal size XML",
			input:    `<ShowPlanXML><Batch><Statements><StmtSimple StatementText="SELECT * FROM users"/></Statements></Batch></ShowPlanXML>`,
			expected: true,
		},
		{
			name:     "Large but safe XML",
			input:    strings.Repeat(`<node attr="value">content</node>`, 1000),
			expected: true,
		},
		{
			name:     "Too large XML",
			input:    strings.Repeat("x", 70000), // Over 64KB limit
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsXMLContentSafe(tt.input)
			if result != tt.expected {
				t.Errorf("IsXMLContentSafe() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// Helper function to create string pointers for testing
func stringPtr(s string) *string {
	return &s
}

// Benchmark tests to ensure performance
func BenchmarkAnonymizeQueryText(b *testing.B) {
	query := "SELECT * FROM users WHERE name = 'John Doe' AND age = 25 AND city = 'New York'"
	
	for i := 0; i < b.N; i++ {
		AnonymizeQueryText(query)
	}
}

func BenchmarkAnonymizeExecutionPlanXML(b *testing.B) {
	xml := `<ShowPlanXML><Batch><Statements><StmtSimple StatementText="SELECT * FROM orders WHERE total > 1000 AND status = 'pending'" StatementId="1"/></Statements></Batch></ShowPlanXML>`
	
	for i := 0; i < b.N; i++ {
		AnonymizeExecutionPlanXML(xml)
	}
}