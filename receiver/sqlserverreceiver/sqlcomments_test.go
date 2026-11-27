// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sqlcomments"
)

// TestCommentTagCoverage documents which comment placements the shared extractor in
// internal/common/sqlcomments recognizes, since that determines what
// db.query.comment_tags can carry. It only reads leading /* */ block comments, so
// the cases below that resolve to "" are silently untagged rather than failing.
//
// Two of them are placements real clients produce: sqlcommenter appends its comment
// after the statement, and sp_executesql prefixes the batch with parameter
// declarations before any injected comment. Widening the shared extractor would
// benefit every receiver using it, so it belongs there rather than here.
func TestCommentTagCoverage(t *testing.T) {
	allowed := []string{"traceparent", "framework"}

	tests := []struct {
		name     string
		sql      string
		expected string
	}{
		{
			name:     "leading block comment",
			sql:      `/*traceparent='00-abc-01',framework='hibernate'*/SELECT name FROM users`,
			expected: "traceparent='00-abc-01',framework='hibernate'",
		},
		{
			name:     "output follows allowedKeys order, not comment order",
			sql:      `/*framework='hibernate',traceparent='00-abc-01'*/SELECT 1`,
			expected: "traceparent='00-abc-01',framework='hibernate'",
		},
		{
			name:     "keys outside the allow-list are dropped",
			sql:      `/*traceparent='00-abc-01',secret='do-not-export'*/SELECT 1`,
			expected: "traceparent='00-abc-01'",
		},
		{
			name:     "quotes are preserved as written",
			sql:      `/*traceparent=00-abc-01*/SELECT 1`,
			expected: "traceparent=00-abc-01",
		},
		{
			name:     "not recognized: comment after parameter declarations (sp_executesql)",
			sql:      `(@P0 int)/*traceparent='00-abc-01'*/UPDATE orders SET total = @P0`,
			expected: "",
		},
		{
			name:     "not recognized: trailing comment (sqlcommenter)",
			sql:      `SELECT name FROM users /*traceparent='00-abc-01'*/`,
			expected: "",
		},
		{
			name:     "not recognized: line comment",
			sql:      "--traceparent='00-abc-01'\nSELECT name FROM users",
			expected: "",
		},
		{
			name:     "no comment",
			sql:      `SELECT name FROM users`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sqlcomments.ExtractAndFilterComments(tt.sql, allowed))
		})
	}
}

// TestCommentTagsDisabledWithoutAllowedKeys asserts the allow-list is the gate: with
// no keys configured, nothing is extracted regardless of what the comment holds.
func TestCommentTagsDisabledWithoutAllowedKeys(t *testing.T) {
	sql := `/*traceparent='00-abc-01'*/SELECT 1`
	assert.Empty(t, sqlcomments.ExtractAndFilterComments(sql, nil))
	assert.Empty(t, sqlcomments.ExtractAndFilterComments(sql, []string{}))
}
