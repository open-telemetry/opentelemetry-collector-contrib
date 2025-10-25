// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestUniqueFlattenedAttributes(t *testing.T) {
	t.Run("empty map returns nil", func(t *testing.T) {
		m := pcommon.NewMap()
		result := UniqueFlattenedAttributes(m)
		require.Nil(t, result)
	})

	t.Run("single flat attribute", func(t *testing.T) {
		m := pcommon.NewMap()
		m.PutStr("key1", "value1")

		result := UniqueFlattenedAttributes(m)
		require.Equal(t, []string{"key1"}, result)
	})

	t.Run("multiple flat attributes sorted", func(t *testing.T) {
		m := pcommon.NewMap()
		m.PutStr("zebra", "value1")
		m.PutStr("alpha", "value2")
		m.PutStr("middle", "value3")

		result := UniqueFlattenedAttributes(m)
		require.Equal(t, []string{"alpha", "middle", "zebra"}, result)
	})

	t.Run("nested map attributes", func(t *testing.T) {
		m := pcommon.NewMap()
		nested := m.PutEmptyMap("parent")
		nested.PutStr("child", "value")

		result := UniqueFlattenedAttributes(m)
		require.Equal(t, []string{"parent.child"}, result)
	})

	t.Run("deeply nested map attributes", func(t *testing.T) {
		m := pcommon.NewMap()
		level1 := m.PutEmptyMap("level1")
		level2 := level1.PutEmptyMap("level2")
		level3 := level2.PutEmptyMap("level3")
		level3.PutStr("last", "value")

		result := UniqueFlattenedAttributes(m)
		require.Equal(t, []string{"level1.level2.level3.last"}, result)
	})

	t.Run("mixed flat and nested attributes", func(t *testing.T) {
		m := pcommon.NewMap()
		m.PutStr("flat1", "value1")
		nested := m.PutEmptyMap("nested")
		nested.PutStr("child1", "value2")
		nested.PutStr("child2", "value3")
		m.PutStr("flat2", "value4")

		result := UniqueFlattenedAttributes(m)
		require.Equal(t, []string{"flat1", "flat2", "nested.child1", "nested.child2"}, result)
	})

	t.Run("multiple nested levels with sorting", func(t *testing.T) {
		m := pcommon.NewMap()
		m.PutStr("z_flat", "value")

		parent1 := m.PutEmptyMap("b_parent")
		parent1.PutStr("child", "value")

		parent2 := m.PutEmptyMap("a_parent")
		parent2.PutStr("child", "value")

		result := UniqueFlattenedAttributes(m)
		require.Equal(t, []string{"a_parent.child", "b_parent.child", "z_flat"}, result)
	})

	t.Run("different value types", func(t *testing.T) {
		m := pcommon.NewMap()
		m.PutStr("string", "value")
		m.PutInt("int", 42)
		m.PutDouble("double", 3.14)
		m.PutBool("bool", true)

		result := UniqueFlattenedAttributes(m)
		require.Equal(t, []string{"bool", "double", "int", "string"}, result)
	})

	t.Run("empty nested map", func(t *testing.T) {
		m := pcommon.NewMap()
		m.PutEmptyMap("empty_parent")
		m.PutStr("sibling", "value")

		result := UniqueFlattenedAttributes(m)
		require.Equal(t, []string{"sibling"}, result)
	})

	t.Run("complex nested structure", func(t *testing.T) {
		m := pcommon.NewMap()

		m.PutStr("root_attr", "value")

		http := m.PutEmptyMap("http")
		http.PutStr("method", "GET")
		http.PutInt("status_code", 200)

		headers := http.PutEmptyMap("headers")
		headers.PutStr("content_type", "application/json")
		headers.PutStr("user_agent", "test")

		db := m.PutEmptyMap("db")
		db.PutStr("system", "postgresql")

		result := UniqueFlattenedAttributes(m)
		expected := []string{
			"db.system",
			"http.headers.content_type",
			"http.headers.user_agent",
			"http.method",
			"http.status_code",
			"root_attr",
		}
		require.Equal(t, expected, result)
	})

	t.Run("handles slice and bytes values", func(t *testing.T) {
		m := pcommon.NewMap()
		m.PutStr("string", "value")

		slice := m.PutEmptySlice("slice")
		slice.AppendEmpty().SetStr("item")

		m.PutEmptyBytes("bytes").FromRaw([]byte{1, 2, 3})

		result := UniqueFlattenedAttributes(m)
		require.Equal(t, []string{"bytes", "slice", "string"}, result)
	})
}

func BenchmarkUniqueFlattenedAttributes(b *testing.B) {
	m := pcommon.NewMap()

	m.PutStr("service.name", "my-service")
	m.PutStr("service.version", "1.0.0")
	m.PutStr("deployment.environment", "production")

	http := m.PutEmptyMap("http")
	http.PutStr("method", "GET")
	http.PutStr("url", "https://example.com/api/users")
	http.PutInt("status_code", 200)
	http.PutStr("user_agent", "Mozilla/5.0")

	headers := http.PutEmptyMap("request.headers")
	headers.PutStr("content-type", "application/json")
	headers.PutStr("accept", "application/json")

	db := m.PutEmptyMap("db")
	db.PutStr("system", "clickhouse")
	db.PutStr("name", "users_db")
	db.PutStr("statement", "SELECT COUNT(*) FROM users")
	db.PutStr("operation", "SELECT")

	m.PutStr("peer.service", "auth-service")
	m.PutStr("span.kind", "server")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = UniqueFlattenedAttributes(m)
	}
}
