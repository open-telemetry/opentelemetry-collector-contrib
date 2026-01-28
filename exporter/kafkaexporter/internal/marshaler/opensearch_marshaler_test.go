package marshaler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestOpenSearchLogsMarshaler(t *testing.T) {
	tests := []struct {
		name           string
		unixTimestamps bool
	}{
		{"ISO 8601 timestamps", false},
		{"Unix milliseconds", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshaler := &OpenSearchLogsMarshaler{
				unixTimestamps: tt.unixTimestamps,
			}

			logs := createTestLogs()
			messages, err := marshaler.MarshalLogs(logs)
			require.NoError(t, err)
			require.NotEmpty(t, messages)

			var doc map[string]any
			err = json.Unmarshal(messages[0].Value, &doc)
			require.NoError(t, err)

			// Verify required fields
			assert.Contains(t, doc, "@timestamp")
			assert.Contains(t, doc, "body")
			assert.Contains(t, doc, "severity")
			assert.Contains(t, doc, "attributes")
			assert.Contains(t, doc, "resource")
			assert.Contains(t, doc, "instrumentationScope")
			assert.Contains(t, doc, "schemaUrl")

			// Verify timestamp format
			timestamp := doc["@timestamp"]
			if tt.unixTimestamps {
				_, ok := timestamp.(float64)
				assert.True(t, ok, "Expected Unix timestamp as number")
			} else {
				_, ok := timestamp.(string)
				assert.True(t, ok, "Expected ISO 8601 timestamp as string")
			}

			// Verify content
			assert.Equal(t, "Test log message", doc["body"])
			assert.Equal(t, "https://opentelemetry.io/schemas/1.21.0", doc["schemaUrl"])

			severity := doc["severity"].(map[string]any)
			assert.Equal(t, "INFO", severity["text"])

			resource := doc["resource"].(map[string]any)
			assert.Equal(t, "test-service", resource["service.name"])

			instrScope := doc["instrumentationScope"].(map[string]any)
			assert.Equal(t, "test-logger", instrScope["name"])
			assert.Equal(t, "1.0.0", instrScope["version"])
			assert.Equal(t, "https://opentelemetry.io/schemas/1.21.0", instrScope["schemaUrl"])

			scopeAttrs := instrScope["attributes"].(map[string]any)
			assert.Equal(t, "test-value", scopeAttrs["scope.attr"])

			attrs := doc["attributes"].(map[string]any)
			assert.Equal(t, "12345", attrs["user.id"])
		})
	}
}

func TestOpenSearchLogsMarshaler_TraceCorrelation(t *testing.T) {
	marshaler := &OpenSearchLogsMarshaler{
		unixTimestamps: false,
	}

	logs := createTestLogsWithTraceContext()
	messages, err := marshaler.MarshalLogs(logs)
	require.NoError(t, err)
	require.NotEmpty(t, messages)

	var doc map[string]any
	err = json.Unmarshal(messages[0].Value, &doc)
	require.NoError(t, err)

	// Verify trace correlation IDs
	assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", doc["traceId"])
	assert.Equal(t, "0102030405060708", doc["spanId"])

	// Verify message key is set to trace ID for partitioning
	expectedKey := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	assert.Equal(t, expectedKey, messages[0].Key)
}

func TestOpenSearchLogsMarshaler_OpenSearchCompatibility(t *testing.T) {
	// This test verifies that the output is compatible with OpenSearch SS4O schema
	marshaler := &OpenSearchLogsMarshaler{
		unixTimestamps: false,
	}

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")
	rl.Resource().Attributes().PutStr("deployment.environment", "production")

	sl := rl.ScopeLogs().AppendEmpty()
	sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
	sl.Scope().SetName("test.logger")
	sl.Scope().SetVersion("2.0.0")

	// Create multiple log records with different characteristics
	records := []struct {
		body       string
		severity   string
		hasTrace   bool
		attributes map[string]any
	}{
		{"Simple log message", "INFO", false, map[string]any{"key": "value"}},
		{"Error occurred", "ERROR", true, map[string]any{"error.code": int64(500)}},
		{"Debug trace", "DEBUG", false, map[string]any{"debug": true}},
	}

	for i, r := range records {
		record := sl.LogRecords().AppendEmpty()
		record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		record.Body().SetStr(r.body)
		record.SetSeverityText(r.severity)

		for k, v := range r.attributes {
			switch val := v.(type) {
			case string:
				record.Attributes().PutStr(k, val)
			case int64:
				record.Attributes().PutInt(k, val)
			case bool:
				record.Attributes().PutBool(k, val)
			}
		}

		if r.hasTrace {
			traceID := [16]byte{byte(i), 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
			spanID := [8]byte{byte(i), 1, 2, 3, 4, 5, 6, 7}
			record.SetTraceID(traceID)
			record.SetSpanID(spanID)
		}
	}

	messages, err := marshaler.MarshalLogs(logs)
	require.NoError(t, err)
	assert.Len(t, messages, 3)

	// Verify each message is valid JSON and has required SS4O fields
	for i, msg := range messages {
		var doc map[string]any
		err := json.Unmarshal(msg.Value, &doc)
		require.NoError(t, err, "Message %d should be valid JSON", i)

		// Required fields for OpenSearch SS4O
		requiredFields := []string{"@timestamp", "body", "severity", "attributes", "resource", "instrumentationScope"}
		for _, field := range requiredFields {
			assert.Contains(t, doc, field, "Message %d missing required field: %s", i, field)
		}

		// Verify structure types
		assert.IsType(t, map[string]any{}, doc["severity"], "severity should be object")
		assert.IsType(t, map[string]any{}, doc["attributes"], "attributes should be object")
		assert.IsType(t, map[string]any{}, doc["resource"], "resource should be object")
		assert.IsType(t, map[string]any{}, doc["instrumentationScope"], "instrumentationScope should be object")

		// Verify severity has text and number
		severity := doc["severity"].(map[string]any)
		assert.Contains(t, severity, "text")
		assert.Contains(t, severity, "number")

		// Verify resource has service name
		resource := doc["resource"].(map[string]any)
		assert.Equal(t, "test-service", resource["service.name"])

		// Verify instrumentation scope
		instrScope := doc["instrumentationScope"].(map[string]any)
		assert.Equal(t, "test.logger", instrScope["name"])
		assert.Equal(t, "2.0.0", instrScope["version"])

		// If trace context exists, verify format
		if traceID, ok := doc["traceId"].(string); ok {
			assert.Len(t, traceID, 32, "traceId should be 32 hex characters")
			assert.Contains(t, doc, "spanId")
			spanID := doc["spanId"].(string)
			assert.Len(t, spanID, 16, "spanId should be 16 hex characters")
		}
	}
}

func createTestLogs() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")

	sl := rl.ScopeLogs().AppendEmpty()
	sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
	sl.Scope().SetName("test-logger")
	sl.Scope().SetVersion("1.0.0")
	sl.Scope().Attributes().PutStr("scope.attr", "test-value")

	record := sl.LogRecords().AppendEmpty()
	record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	record.Body().SetStr("Test log message")
	record.SetSeverityText("INFO")
	record.SetSeverityNumber(plog.SeverityNumberInfo)
	record.Attributes().PutStr("user.id", "12345")

	return logs
}

func createTestLogsWithTraceContext() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-service")

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("test-logger")

	record := sl.LogRecords().AppendEmpty()
	record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	record.Body().SetStr("Test log with trace context")
	record.SetSeverityText("INFO")
	record.SetSeverityNumber(plog.SeverityNumberInfo)

	traceID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	record.SetTraceID(traceID)
	record.SetSpanID(spanID)

	return logs
}
