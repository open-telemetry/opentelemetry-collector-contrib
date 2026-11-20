// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func newTestLogParser(t *testing.T) ottl.Parser[*ottllog.TransformContext] {
	settings := componenttest.NewNopTelemetrySettings()
	p, err := ottllog.NewParser(ottlfuncs.StandardFuncs[*ottllog.TransformContext](), settings, ottllog.EnablePathContextNames())
	require.NoError(t, err)
	return p
}

// 1. Ordinary nested conditions still parse successfully.
func TestNestedConditions_Ordinary(t *testing.T) {
	parser := newTestLogParser(t)

	conds := []string{
		`(((log.body == "fido")))`,
		`(log.body == "fido")`,
		`((log.body == "fido"))`,
		`((((((log.body == "fido"))))))`,
	}

	for _, c := range conds {
		t.Run(c, func(t *testing.T) {
			parsed, err := parser.ParseConditions([]string{c})
			require.NoError(t, err)
			require.Len(t, parsed, 1)
		})
	}
}

// 2. Multiple nested boolean expressions preserve their meaning.
func TestNestedConditions_PreserveMeaning(t *testing.T) {
	parser := newTestLogParser(t)

	cond := `((log.body == "fido") and (log.severity_number > 2)) or ((not (log.dropped_attributes_count == 0)) and (log.flags == 1))`
	parsed, err := parser.ParseConditions([]string{cond})
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	// Verify evaluation works as expected
	tCtx := newBenchmarkLogContext(1)
	tCtx.GetLogRecord().Body().SetStr("fido")
	tCtx.GetLogRecord().SetSeverityNumber(5)

	matched, err := parsed[0].Eval(t.Context(), tCtx)
	require.NoError(t, err)
	assert.True(t, matched)

	tCtx.GetLogRecord().Body().SetStr("cat")
	matched, err = parsed[0].Eval(t.Context(), tCtx)
	require.NoError(t, err)
	assert.False(t, matched)
}

// 3. Conditions at allowed nesting depths (including boundary below and at limit) parse successfully.
func TestNestedConditions_AllowedDepthsBoundary(t *testing.T) {
	parser := newTestLogParser(t)

	// Boundary test: immediately below limit (1999) and at limit (2000)
	for _, depth := range []int{10, 50, 100, 300, 600, 1000, 1999, 2000} {
		t.Run(fmt.Sprintf("depth=%d", depth), func(t *testing.T) {
			cond := buildNestedCondition(depth)
			parsed, err := parser.ParseConditions([]string{cond})
			require.NoError(t, err, "depth %d should be accepted", depth)
			require.Len(t, parsed, 1)
		})
	}
}

// 4. Excessively nested conditions return an error without crashing.
func TestNestedConditions_ExcessiveNestingRejected(t *testing.T) {
	parser := newTestLogParser(t)

	// Boundary test: immediately above limit (2001) and deeply nested inputs
	for _, depth := range []int{2001, 2500, 5000, 10000} {
		t.Run(fmt.Sprintf("depth=%d", depth), func(t *testing.T) {
			cond := buildNestedCondition(depth)
			_, err := parser.ParseConditions([]string{cond})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "expression exceeds maximum allowed nesting depth of 2000")
		})
	}
}

// 5. Deep nesting in a statement's where clause is handled safely.
func TestNestedConditions_StatementWhereClause(t *testing.T) {
	parser := newTestLogParser(t)

	t.Run("within_limit", func(t *testing.T) {
		stmt := buildNestedStatement(100)
		parsed, err := parser.ParseStatements([]string{stmt})
		require.NoError(t, err)
		require.Len(t, parsed, 1)
	})

	t.Run("at_limit", func(t *testing.T) {
		stmt := buildNestedStatement(2000)
		parsed, err := parser.ParseStatements([]string{stmt})
		require.NoError(t, err)
		require.Len(t, parsed, 1)
	})

	t.Run("above_limit", func(t *testing.T) {
		stmt := buildNestedStatement(2001)
		_, err := parser.ParseStatements([]string{stmt})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression exceeds maximum allowed nesting depth of 2000")
	})
}

// 6. Long but shallow expressions are not rejected solely because of their length.
func TestNestedConditions_LongShallowExpression(t *testing.T) {
	parser := newTestLogParser(t)

	// Build a condition with 2500 flat OR clauses (total size > 60 KB, depth 0)
	var clauses []string
	for i := range 2500 {
		clauses = append(clauses, fmt.Sprintf(`log.body == "%d"`, i))
	}
	cond := strings.Join(clauses, " or ")

	parsed, err := parser.ParseConditions([]string{cond})
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	// Also test 2500 flat parenthesized OR clauses: (log.body == "0") or (log.body == "1") (depth 1)
	var parenClauses []string
	for i := range 2500 {
		parenClauses = append(parenClauses, fmt.Sprintf(`(log.body == "%d")`, i))
	}
	parenCond := strings.Join(parenClauses, " or ")

	parsedParen, err := parser.ParseConditions([]string{parenCond})
	require.NoError(t, err)
	require.Len(t, parsedParen, 1)
}

// 7. Parentheses inside quoted strings are not counted as syntactic nesting.
func TestNestedConditions_ParenthesesInStringsIgnored(t *testing.T) {
	parser := newTestLogParser(t)

	// 2500 parentheses inside a string literal
	deepString := strings.Repeat("(", 2500) + strings.Repeat(")", 2500)
	cond := fmt.Sprintf("log.body == %q", deepString)

	parsed, err := parser.ParseConditions([]string{cond})
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	// String with escaped quotes and brackets
	escapedString := strings.Repeat(`\"(`, 1500) + strings.Repeat(`)\"`, 1500)
	stmt := `set(log.body, "` + escapedString + `")`
	parsedStmt, err := parser.ParseStatements([]string{stmt})
	require.NoError(t, err)
	require.Len(t, parsedStmt, 1)
}

// 8. Malformed and unbalanced parentheses produce appropriate syntax errors.
func TestNestedConditions_MalformedUnbalanced(t *testing.T) {
	parser := newTestLogParser(t)

	tests := []struct {
		name      string
		condition string
		errSubstr string
	}{
		{
			name:      "unclosed_paren",
			condition: `(((log.body == "x"))`,
			errSubstr: "invalid syntax",
		},
		{
			name:      "extra_closing_paren",
			condition: `((log.body == "x")))`,
			errSubstr: "invalid syntax",
		},
		{
			name:      "lone_open_paren",
			condition: `(`,
			errSubstr: "invalid syntax",
		},
		{
			name:      "lone_close_paren",
			condition: `)`,
			errSubstr: "invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.ParseConditions([]string{tt.condition})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)
			// Ensure it did not trigger the nesting depth error
			assert.NotContains(t, err.Error(), "exceeds maximum allowed nesting depth")
		})
	}
}

// 9. Existing OTTL grammar behavior remains unchanged.
func TestNestedConditions_GrammarCompatibility(t *testing.T) {
	parser := newTestLogParser(t)

	tests := []struct {
		name      string
		condition string
	}{
		{name: "parenthesized_math_comparison", condition: `(1 + 2) == 3`},
		{name: "nested_math_comparison", condition: `((1 + 2) * 3) == 9`},
		{name: "not_with_subexpr", condition: `not (log.body == "x")`},
		{name: "complex_boolean_logic", condition: `not ((log.body == "1" and log.severity_number == 2) or log.body == "3")`},
		{name: "converter_comparison", condition: `IsString(log.body) == true`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parser.ParseConditions([]string{tt.condition})
			require.NoError(t, err)
			require.Len(t, parsed, 1)
		})
	}
}

// 10. Other nested syntactic constructs do not bypass safety limits.
func TestNestedConditions_OtherConstructsSafety(t *testing.T) {
	parser := newTestLogParser(t)

	t.Run("nested_lists_rejected_above_limit", func(t *testing.T) {
		deepList := strings.Repeat("[", 2001) + "1" + strings.Repeat("]", 2001)
		stmt := fmt.Sprintf(`set(log.body, %s)`, deepList)
		_, err := parser.ParseStatements([]string{stmt})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression exceeds maximum allowed nesting depth of 2000")
	})

	t.Run("nested_maps_rejected_above_limit", func(t *testing.T) {
		deepMap := strings.Repeat(`{"k": `, 2001) + `"v"` + strings.Repeat("}", 2001)
		stmt := fmt.Sprintf(`set(log.body, %s)`, deepMap)
		_, err := parser.ParseStatements([]string{stmt})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression exceeds maximum allowed nesting depth of 2000")
	})

	t.Run("nested_functions_rejected_above_limit", func(t *testing.T) {
		deepFunc := strings.Repeat("IsString(", 2001) + `log.body` + strings.Repeat(")", 2001)
		_, err := parser.ParseConditions([]string{deepFunc})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expression exceeds maximum allowed nesting depth of 2000")
	})
}
