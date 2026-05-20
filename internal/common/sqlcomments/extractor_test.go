// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlcomments

import (
	"testing"
)

func TestExtractAndFilterComments(t *testing.T) {
	t.Run("single allowed key", func(t *testing.T) {
		sqlText := "/* nr_service_guid=abc-123 */ SELECT * FROM t"
		allowedKeys := []string{"nr_service_guid"}
		want := "nr_service_guid=abc-123"

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("multiple allowed keys", func(t *testing.T) {
		sqlText := "/* nr_service_guid=abc,app_id=xyz */ SELECT * FROM t"
		allowedKeys := []string{"nr_service_guid", "app_id"}
		want := "nr_service_guid=abc,app_id=xyz"

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		sqlText := "/* other=val */ SELECT * FROM t"
		allowedKeys := []string{"nr_service_guid"}
		want := ""

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("empty allowlist", func(t *testing.T) {
		sqlText := "/* nr_service_guid=abc */ SELECT * FROM t"
		allowedKeys := []string{}
		want := ""

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("nil allowlist", func(t *testing.T) {
		sqlText := "/* nr_service_guid=abc */ SELECT * FROM t"
		var allowedKeys []string
		want := ""

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("multiple comments", func(t *testing.T) {
		sqlText := "/* a=1 */ /* b=2 */ SELECT * FROM t"
		allowedKeys := []string{"a", "b"}
		want := "a=1,b=2"

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("not leading comment", func(t *testing.T) {
		sqlText := "SELECT * FROM t /* a=1 */"
		allowedKeys := []string{"a"}
		want := ""

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("whitespace before comment", func(t *testing.T) {
		sqlText := "   /* nr_service_guid=abc */ SELECT * FROM t"
		allowedKeys := []string{"nr_service_guid"}
		want := "nr_service_guid=abc"

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("keys with spaces trimmed", func(t *testing.T) {
		sqlText := "/* key1 = value1 , key2 = value2 */ SELECT * FROM t"
		allowedKeys := []string{"key1", "key2"}
		want := "key1=value1,key2=value2"

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("partial match filters correctly", func(t *testing.T) {
		sqlText := "/* allowed=yes,notallowed=no,also_allowed=maybe */ SELECT * FROM t"
		allowedKeys := []string{"allowed", "also_allowed"}
		want := "allowed=yes,also_allowed=maybe"

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("malformed pairs skipped", func(t *testing.T) {
		sqlText := "/* valid=1,invalid,another=2 */ SELECT * FROM t"
		allowedKeys := []string{"valid", "invalid", "another"}
		want := "valid=1,another=2"

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("empty comment", func(t *testing.T) {
		sqlText := "/**/ SELECT * FROM t"
		allowedKeys := []string{"any"}
		want := ""

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("no comments", func(t *testing.T) {
		sqlText := "SELECT * FROM t"
		allowedKeys := []string{"any"}
		want := ""

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("unclosed comment", func(t *testing.T) {
		sqlText := "/* unclosed SELECT * FROM t"
		allowedKeys := []string{"any"}
		want := ""

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("values with special characters", func(t *testing.T) {
		sqlText := `/* guid=abc-123-def,path=/api/v1/users */ SELECT * FROM t`
		allowedKeys := []string{"guid", "path"}
		want := "guid=abc-123-def,path=/api/v1/users"

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})

	t.Run("duplicate keys use first", func(t *testing.T) {
		sqlText := "/* key=first,key=second */ SELECT * FROM t"
		allowedKeys := []string{"key"}
		want := "key=first"

		got := ExtractAndFilterComments(sqlText, allowedKeys)

		if got != want {
			t.Errorf("ExtractAndFilterComments() = %q, want %q", got, want)
		}
	})
}
