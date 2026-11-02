// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlcomments

import (
	"testing"
)

func TestExtractAndFilterComments(t *testing.T) {
	tests := []struct {
		name        string
		sqlText     string
		allowedKeys []string
		expected    string
	}{
		{
			name:        "single allowed key",
			sqlText:     "/* application=abc-123 */ SELECT * FROM t",
			allowedKeys: []string{"application"},
			expected:    "application=abc-123",
		},
		{
			name:        "multiple allowed keys",
			sqlText:     "/* application=abc,app_id=xyz */ SELECT * FROM t",
			allowedKeys: []string{"application", "app_id"},
			expected:    "application=abc,app_id=xyz",
		},
		{
			name:        "no matches",
			sqlText:     "/* other=val */ SELECT * FROM t",
			allowedKeys: []string{"application"},
			expected:    "",
		},
		{
			name:        "empty allowlist",
			sqlText:     "/* application=abc */ SELECT * FROM t",
			allowedKeys: []string{},
			expected:    "",
		},
		{
			name:        "nil allowlist",
			sqlText:     "/* application=abc */ SELECT * FROM t",
			allowedKeys: nil,
			expected:    "",
		},
		{
			name:        "multiple comments",
			sqlText:     "/* a=1 */ /* b=2 */ SELECT * FROM t",
			allowedKeys: []string{"a", "b"},
			expected:    "a=1,b=2",
		},
		{
			name:        "not leading comment",
			sqlText:     "SELECT * FROM t /* a=1 */",
			allowedKeys: []string{"a"},
			expected:    "",
		},
		{
			name:        "whitespace before comment",
			sqlText:     "   /* application=abc */ SELECT * FROM t",
			allowedKeys: []string{"application"},
			expected:    "application=abc",
		},
		{
			name:        "keys with spaces trimmed",
			sqlText:     "/* key1 = value1 , key2 = value2 */ SELECT * FROM t",
			allowedKeys: []string{"key1", "key2"},
			expected:    "key1=value1,key2=value2",
		},
		{
			name:        "partial match filters correctly",
			sqlText:     "/* allowed=yes,notallowed=no,also_allowed=maybe */ SELECT * FROM t",
			allowedKeys: []string{"allowed", "also_allowed"},
			expected:    "allowed=yes,also_allowed=maybe",
		},
		{
			name:        "malformed pairs skipped",
			sqlText:     "/* valid=1,invalid,another=2 */ SELECT * FROM t",
			allowedKeys: []string{"valid", "invalid", "another"},
			expected:    "valid=1,another=2",
		},
		{
			name:        "empty comment",
			sqlText:     "/**/ SELECT * FROM t",
			allowedKeys: []string{"any"},
			expected:    "",
		},
		{
			name:        "no comments",
			sqlText:     "SELECT * FROM t",
			allowedKeys: []string{"any"},
			expected:    "",
		},
		{
			name:        "unclosed comment",
			sqlText:     "/* unclosed SELECT * FROM t",
			allowedKeys: []string{"any"},
			expected:    "",
		},
		{
			name:        "values with special characters",
			sqlText:     `/* guid=abc-123-def,path=/api/v1/users */ SELECT * FROM t`,
			allowedKeys: []string{"guid", "path"},
			expected:    "guid=abc-123-def,path=/api/v1/users",
		},
		{
			name:        "duplicate keys use first",
			sqlText:     "/* key=first,key=second */ SELECT * FROM t",
			allowedKeys: []string{"key"},
			expected:    "key=first",
		},
		{
			name:        "duplicate key across comments uses first",
			sqlText:     "/* key=first */ /* key=second */ SELECT * FROM t",
			allowedKeys: []string{"key"},
			expected:    "key=first",
		},
		{
			name:        "only leading comments parsed",
			sqlText:     "/* a=1 */ SELECT /* b=2 */ FROM t",
			allowedKeys: []string{"a", "b"},
			expected:    "a=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractAndFilterComments(tt.sqlText, tt.allowedKeys)
			if got != tt.expected {
				t.Errorf("ExtractAndFilterComments() = %q, want %q", got, tt.expected)
			}
		})
	}
}
