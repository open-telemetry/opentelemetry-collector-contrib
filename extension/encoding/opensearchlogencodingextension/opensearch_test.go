// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchlogencodingextension

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestMarshalLogs_BasicSingleRecord(t *testing.T) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-svc")

	sl := rl.ScopeLogs().AppendEmpty()
	sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
	scope := sl.Scope()
	scope.SetName("test-logger")
	scope.SetVersion("1.0.0")

	rec := sl.LogRecords().AppendEmpty()
	rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC)))
	rec.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 30, 46, 0, time.UTC)))
	rec.SetSeverityText("INFO")
	rec.SetSeverityNumber(plog.SeverityNumberInfo)
	rec.Body().SetStr("test log message")
	rec.Attributes().PutStr("user.id", "12345")
	rec.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rec.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})

	e := &opensearchLogExtension{}
	result, err := e.MarshalLogs(logs)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(result, &doc))

	assert.Equal(t, "2024-01-15T10:30:45.123456789Z", doc["@timestamp"])
	assert.Equal(t, "test log message", doc["body"])
	assert.Equal(t, "2024-01-15T10:30:46Z", doc["observedTimestamp"])
	assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", doc["traceId"])
	assert.Equal(t, "0102030405060708", doc["spanId"])

	severity := doc["severity"].(map[string]any)
	assert.Equal(t, "INFO", severity["text"])
	assert.Equal(t, float64(9), severity["number"])

	attrs := doc["attributes"].(map[string]any)
	assert.Equal(t, "12345", attrs["user.id"])

	resource := doc["resource"].(map[string]any)
	assert.Equal(t, "test-svc", resource["service.name"])

	assert.Equal(t, "https://opentelemetry.io/schemas/1.21.0", doc["schemaUrl"])

	is := doc["instrumentationScope"].(map[string]any)
	assert.Equal(t, "test-logger", is["name"])
	assert.Equal(t, "1.0.0", is["version"])
	assert.Equal(t, "https://opentelemetry.io/schemas/1.21.0", is["schemaUrl"])
}

func TestMarshalLogs_EmptyInput(t *testing.T) {
	e := &opensearchLogExtension{}

	t.Run("zero resource logs", func(t *testing.T) {
		result, err := e.MarshalLogs(plog.NewLogs())
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty scope logs", func(t *testing.T) {
		logs := plog.NewLogs()
		logs.ResourceLogs().AppendEmpty()
		result, err := e.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("empty log records", func(t *testing.T) {
		logs := plog.NewLogs()
		logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
		result, err := e.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestMarshalLogs_OptionalFieldsOmitted(t *testing.T) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("logger")
	sl.Scope().SetVersion("1.0")
	rec := sl.LogRecords().AppendEmpty()
	rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)))
	rec.Body().SetStr("minimal log")
	// No observedTimestamp, no traceId, no spanId, no schemaUrl set.

	e := &opensearchLogExtension{}
	result, err := e.MarshalLogs(logs)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(result, &doc))

	_, hasObserved := doc["observedTimestamp"]
	assert.False(t, hasObserved, "observedTimestamp should be absent")

	_, hasTrace := doc["traceId"]
	assert.False(t, hasTrace, "traceId should be absent")

	_, hasSpan := doc["spanId"]
	assert.False(t, hasSpan, "spanId should be absent")

	_, hasSchema := doc["schemaUrl"]
	assert.False(t, hasSchema, "schemaUrl should be absent")
}

func TestMarshalLogs_NDJSONBatch(t *testing.T) {
	logs := plog.NewLogs()

	// Resource 1 with 2 log records
	rl1 := logs.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("service.name", "svc-a")
	sl1 := rl1.ScopeLogs().AppendEmpty()
	sl1.Scope().SetName("logger-a")
	sl1.Scope().SetVersion("1.0")
	for i := range 2 {
		rec := sl1.LogRecords().AppendEmpty()
		rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 0, i, 0, time.UTC)))
		rec.Body().SetStr(fmt.Sprintf("log %d from svc-a", i))
	}

	// Resource 2 with 1 log record
	rl2 := logs.ResourceLogs().AppendEmpty()
	rl2.Resource().Attributes().PutStr("service.name", "svc-b")
	sl2 := rl2.ScopeLogs().AppendEmpty()
	sl2.Scope().SetName("logger-b")
	sl2.Scope().SetVersion("2.0")
	rec := sl2.LogRecords().AppendEmpty()
	rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)))
	rec.Body().SetStr("log from svc-b")

	e := &opensearchLogExtension{}
	result, err := e.MarshalLogs(logs)
	require.NoError(t, err)

	lines := bytes.Split(result, []byte("\n"))
	require.Len(t, lines, 3, "expected 3 NDJSON lines for 3 log records")

	// Verify each line is valid JSON and has correct resource
	for i, line := range lines {
		var doc map[string]any
		require.NoError(t, json.Unmarshal(line, &doc), "line %d should be valid JSON", i)

		resource := doc["resource"].(map[string]any)
		if i < 2 {
			assert.Equal(t, "svc-a", resource["service.name"])
		} else {
			assert.Equal(t, "svc-b", resource["service.name"])
		}
	}
}

func TestMarshalLogs_AttributeTypes(t *testing.T) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("logger")
	sl.Scope().SetVersion("1.0")
	rec := sl.LogRecords().AppendEmpty()
	rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)))
	rec.Body().SetStr("typed attrs")

	rec.Attributes().PutStr("str", "hello")
	rec.Attributes().PutInt("int", 42)
	rec.Attributes().PutDouble("double", 3.14)
	rec.Attributes().PutBool("bool", true)

	sliceVal := rec.Attributes().PutEmptySlice("slice")
	sliceVal.AppendEmpty().SetStr("a")
	sliceVal.AppendEmpty().SetInt(1)

	mapVal := rec.Attributes().PutEmptyMap("map")
	mapVal.PutStr("nested", "value")

	bytesVal := rec.Attributes().PutEmptyBytes("bytes")
	bytesVal.Append(0xde, 0xad, 0xbe, 0xef)

	e := &opensearchLogExtension{}
	result, err := e.MarshalLogs(logs)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(result, &doc))

	attrs := doc["attributes"].(map[string]any)
	assert.Equal(t, "hello", attrs["str"])
	assert.Equal(t, float64(42), attrs["int"])
	assert.Equal(t, 3.14, attrs["double"])
	assert.Equal(t, true, attrs["bool"])
	assert.Equal(t, []any{"a", float64(1)}, attrs["slice"])
	assert.Equal(t, map[string]any{"nested": "value"}, attrs["map"])
	assert.Equal(t, "deadbeef", attrs["bytes"])
}

func TestMarshalLogs_BodyTypes(t *testing.T) {
	tests := []struct {
		name     string
		setBody  func(pcommon.Value)
		expected string
	}{
		{
			name:     "string body",
			setBody:  func(v pcommon.Value) { v.SetStr("hello world") },
			expected: "hello world",
		},
		{
			name: "map body as string",
			setBody: func(v pcommon.Value) {
				m := v.SetEmptyMap()
				m.PutStr("key", "val")
			},
			// AsString() on a map produces its string representation
		},
		{
			name:     "int body as string",
			setBody:  func(v pcommon.Value) { v.SetInt(42) },
			expected: "42",
		},
		{
			name:     "bool body as string",
			setBody:  func(v pcommon.Value) { v.SetBool(true) },
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := plog.NewLogs()
			rl := logs.ResourceLogs().AppendEmpty()
			sl := rl.ScopeLogs().AppendEmpty()
			sl.Scope().SetName("l")
			sl.Scope().SetVersion("1")
			rec := sl.LogRecords().AppendEmpty()
			rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
			tt.setBody(rec.Body())

			e := &opensearchLogExtension{}
			result, err := e.MarshalLogs(logs)
			require.NoError(t, err)

			var doc map[string]any
			require.NoError(t, json.Unmarshal(result, &doc), "output must be valid JSON")

			if tt.expected != "" {
				assert.Equal(t, tt.expected, doc["body"])
			} else {
				// For non-string types, just verify body is a non-empty string
				assert.IsType(t, "", doc["body"])
				assert.NotEmpty(t, doc["body"])
			}
		})
	}
}

func TestMarshalLogs_JSONEscaping(t *testing.T) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("host", "server\"one")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("l")
	sl.Scope().SetVersion("1")
	rec := sl.LogRecords().AppendEmpty()
	rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
	rec.Body().SetStr("line1\nline2\ttab\"quote\\backslash")
	rec.Attributes().PutStr("path", "C:\\Users\\test")

	e := &opensearchLogExtension{}
	result, err := e.MarshalLogs(logs)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(result, &doc), "output with special chars must be valid JSON")
	assert.Equal(t, "line1\nline2\ttab\"quote\\backslash", doc["body"])

	attrs := doc["attributes"].(map[string]any)
	assert.Equal(t, "C:\\Users\\test", attrs["path"])

	resource := doc["resource"].(map[string]any)
	assert.Equal(t, "server\"one", resource["host"])
}

func buildBenchLogs(n int) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "bench-svc")
	rl.Resource().Attributes().PutStr("host.name", "bench-host")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("bench-logger")
	sl.Scope().SetVersion("1.0.0")
	sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
	for i := range n {
		rec := sl.LogRecords().AppendEmpty()
		rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 0, 0, i*1000, time.UTC)))
		rec.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 0, 1, 0, time.UTC)))
		rec.SetSeverityText("INFO")
		rec.SetSeverityNumber(plog.SeverityNumberInfo)
		rec.Body().SetStr("benchmark log message with some realistic content")
		rec.Attributes().PutStr("user.id", "user-123")
		rec.Attributes().PutInt("http.status_code", 200)
		rec.Attributes().PutStr("http.method", "GET")
	}
	return logs
}

func BenchmarkMarshalLogs(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	e := &opensearchLogExtension{}
	for _, n := range sizes {
		logs := buildBenchLogs(n)
		b.Run(fmt.Sprintf("records=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				result, err := e.MarshalLogs(logs)
				if err != nil {
					b.Fatal(err)
				}
				_ = result
			}
		})
	}
}

func BenchmarkMarshalLogs_OTLPJSONComparison(b *testing.B) {
	logs := buildBenchLogs(1000)
	otlp := &plog.JSONMarshaler{}
	ss4o := &opensearchLogExtension{}

	b.Run("otlp_json", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			result, err := otlp.MarshalLogs(logs)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})
	b.Run("ss4o_ndjson", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			result, err := ss4o.MarshalLogs(logs)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})
}
