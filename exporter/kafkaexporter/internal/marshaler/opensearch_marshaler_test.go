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

// TestOpenSearchLogsMarshaler_BatchHandling tests the marshaler's ability to handle
// batches of plog.Logs with various configurations of ResourceLogs, ScopeLogs, and LogRecords.
func TestOpenSearchLogsMarshaler_BatchHandling(t *testing.T) {
	marshaler := &OpenSearchLogsMarshaler{
		unixTimestamps: false,
	}

	t.Run("empty logs", func(t *testing.T) {
		logs := plog.NewLogs()
		messages, err := marshaler.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Empty(t, messages)
	})

	t.Run("single resource with empty scope logs", func(t *testing.T) {
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "test-service")
		// No ScopeLogs added

		messages, err := marshaler.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Empty(t, messages)
	})

	t.Run("single resource with scope but no log records", func(t *testing.T) {
		logs := plog.NewLogs()
		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "test-service")
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("test-logger")
		// No LogRecords added

		messages, err := marshaler.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Empty(t, messages)
	})

	t.Run("multiple resource logs with different services", func(t *testing.T) {
		logs := plog.NewLogs()

		// First ResourceLogs - service-a
		rl1 := logs.ResourceLogs().AppendEmpty()
		rl1.Resource().Attributes().PutStr("service.name", "service-a")
		rl1.Resource().Attributes().PutStr("host.name", "host-1")
		sl1 := rl1.ScopeLogs().AppendEmpty()
		sl1.Scope().SetName("logger-a")
		record1 := sl1.LogRecords().AppendEmpty()
		record1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		record1.Body().SetStr("Log from service-a")
		record1.SetSeverityText("INFO")

		// Second ResourceLogs - service-b
		rl2 := logs.ResourceLogs().AppendEmpty()
		rl2.Resource().Attributes().PutStr("service.name", "service-b")
		rl2.Resource().Attributes().PutStr("host.name", "host-2")
		sl2 := rl2.ScopeLogs().AppendEmpty()
		sl2.Scope().SetName("logger-b")
		record2 := sl2.LogRecords().AppendEmpty()
		record2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		record2.Body().SetStr("Log from service-b")
		record2.SetSeverityText("WARN")

		// Third ResourceLogs - service-c
		rl3 := logs.ResourceLogs().AppendEmpty()
		rl3.Resource().Attributes().PutStr("service.name", "service-c")
		sl3 := rl3.ScopeLogs().AppendEmpty()
		sl3.Scope().SetName("logger-c")
		record3 := sl3.LogRecords().AppendEmpty()
		record3.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		record3.Body().SetStr("Log from service-c")
		record3.SetSeverityText("ERROR")

		messages, err := marshaler.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Len(t, messages, 3)

		// Verify each message has correct resource attribution
		expectedServices := []string{"service-a", "service-b", "service-c"}
		expectedBodies := []string{"Log from service-a", "Log from service-b", "Log from service-c"}
		expectedLoggers := []string{"logger-a", "logger-b", "logger-c"}

		for i, msg := range messages {
			var doc map[string]any
			err := json.Unmarshal(msg.Value, &doc)
			require.NoError(t, err)

			resource := doc["resource"].(map[string]any)
			assert.Equal(t, expectedServices[i], resource["service.name"])

			assert.Equal(t, expectedBodies[i], doc["body"])

			instrScope := doc["instrumentationScope"].(map[string]any)
			assert.Equal(t, expectedLoggers[i], instrScope["name"])
		}
	})

	t.Run("single resource with multiple scope logs", func(t *testing.T) {
		logs := plog.NewLogs()

		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "multi-scope-service")

		// First ScopeLogs - HTTP module
		sl1 := rl.ScopeLogs().AppendEmpty()
		sl1.Scope().SetName("http-module")
		sl1.Scope().SetVersion("1.0.0")
		sl1.SetSchemaUrl("https://example.com/schema/http")
		record1 := sl1.LogRecords().AppendEmpty()
		record1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		record1.Body().SetStr("HTTP request received")
		record1.SetSeverityText("INFO")

		// Second ScopeLogs - Database module
		sl2 := rl.ScopeLogs().AppendEmpty()
		sl2.Scope().SetName("database-module")
		sl2.Scope().SetVersion("2.0.0")
		sl2.SetSchemaUrl("https://example.com/schema/db")
		record2 := sl2.LogRecords().AppendEmpty()
		record2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		record2.Body().SetStr("Database query executed")
		record2.SetSeverityText("DEBUG")

		// Third ScopeLogs - Cache module
		sl3 := rl.ScopeLogs().AppendEmpty()
		sl3.Scope().SetName("cache-module")
		sl3.Scope().SetVersion("3.0.0")
		record3 := sl3.LogRecords().AppendEmpty()
		record3.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		record3.Body().SetStr("Cache miss")
		record3.SetSeverityText("WARN")

		messages, err := marshaler.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Len(t, messages, 3)

		// Verify each message has the same resource but different scopes
		expectedScopes := []string{"http-module", "database-module", "cache-module"}
		expectedVersions := []string{"1.0.0", "2.0.0", "3.0.0"}
		expectedSchemas := []string{"https://example.com/schema/http", "https://example.com/schema/db", ""}

		for i, msg := range messages {
			var doc map[string]any
			err := json.Unmarshal(msg.Value, &doc)
			require.NoError(t, err)

			// All should have the same resource
			resource := doc["resource"].(map[string]any)
			assert.Equal(t, "multi-scope-service", resource["service.name"])

			// But different instrumentation scopes
			instrScope := doc["instrumentationScope"].(map[string]any)
			assert.Equal(t, expectedScopes[i], instrScope["name"])
			assert.Equal(t, expectedVersions[i], instrScope["version"])

			// Verify schema URL handling
			if expectedSchemas[i] != "" {
				assert.Equal(t, expectedSchemas[i], doc["schemaUrl"])
				assert.Equal(t, expectedSchemas[i], instrScope["schemaUrl"])
			} else {
				_, hasSchemaUrl := doc["schemaUrl"]
				assert.False(t, hasSchemaUrl, "schemaUrl should not be present when empty")
			}
		}
	})

	t.Run("single scope with multiple log records", func(t *testing.T) {
		logs := plog.NewLogs()

		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "batch-service")

		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("batch-logger")

		// Add 10 log records to simulate a batch
		baseTime := time.Now()
		for i := 0; i < 10; i++ {
			record := sl.LogRecords().AppendEmpty()
			record.SetTimestamp(pcommon.NewTimestampFromTime(baseTime.Add(time.Duration(i) * time.Second)))
			record.Body().SetStr("Batch log message " + string(rune('0'+i)))
			record.SetSeverityText("INFO")
			record.SetSeverityNumber(plog.SeverityNumberInfo)
			record.Attributes().PutInt("sequence", int64(i))
		}

		messages, err := marshaler.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Len(t, messages, 10)

		// Verify all messages maintain correct order and attributes
		for i, msg := range messages {
			var doc map[string]any
			err := json.Unmarshal(msg.Value, &doc)
			require.NoError(t, err)

			attrs := doc["attributes"].(map[string]any)
			sequence := attrs["sequence"].(float64) // JSON numbers are float64
			assert.Equal(t, float64(i), sequence)

			resource := doc["resource"].(map[string]any)
			assert.Equal(t, "batch-service", resource["service.name"])

			instrScope := doc["instrumentationScope"].(map[string]any)
			assert.Equal(t, "batch-logger", instrScope["name"])
		}
	})

	t.Run("complex batch with multiple resources, scopes, and records", func(t *testing.T) {
		logs := plog.NewLogs()

		// Service A: 2 scopes, each with 3 records = 6 messages
		rlA := logs.ResourceLogs().AppendEmpty()
		rlA.Resource().Attributes().PutStr("service.name", "service-a")
		rlA.Resource().Attributes().PutStr("deployment.environment", "production")

		slA1 := rlA.ScopeLogs().AppendEmpty()
		slA1.Scope().SetName("scope-a1")
		for i := 0; i < 3; i++ {
			rec := slA1.LogRecords().AppendEmpty()
			rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			rec.Body().SetStr("service-a scope-a1 record-" + string(rune('0'+i)))
			rec.Attributes().PutStr("origin", "a1")
			rec.Attributes().PutInt("index", int64(i))
		}

		slA2 := rlA.ScopeLogs().AppendEmpty()
		slA2.Scope().SetName("scope-a2")
		for i := 0; i < 3; i++ {
			rec := slA2.LogRecords().AppendEmpty()
			rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			rec.Body().SetStr("service-a scope-a2 record-" + string(rune('0'+i)))
			rec.Attributes().PutStr("origin", "a2")
			rec.Attributes().PutInt("index", int64(i))
		}

		// Service B: 1 scope with 4 records = 4 messages
		rlB := logs.ResourceLogs().AppendEmpty()
		rlB.Resource().Attributes().PutStr("service.name", "service-b")
		rlB.Resource().Attributes().PutStr("deployment.environment", "staging")

		slB := rlB.ScopeLogs().AppendEmpty()
		slB.Scope().SetName("scope-b")
		for i := 0; i < 4; i++ {
			rec := slB.LogRecords().AppendEmpty()
			rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			rec.Body().SetStr("service-b scope-b record-" + string(rune('0'+i)))
			rec.Attributes().PutStr("origin", "b")
			rec.Attributes().PutInt("index", int64(i))
		}

		// Service C: 3 scopes with 1 record each = 3 messages
		rlC := logs.ResourceLogs().AppendEmpty()
		rlC.Resource().Attributes().PutStr("service.name", "service-c")

		for scopeIdx := 0; scopeIdx < 3; scopeIdx++ {
			sl := rlC.ScopeLogs().AppendEmpty()
			sl.Scope().SetName("scope-c" + string(rune('0'+scopeIdx)))
			rec := sl.LogRecords().AppendEmpty()
			rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			rec.Body().SetStr("service-c scope-c" + string(rune('0'+scopeIdx)) + " record")
			rec.Attributes().PutStr("origin", "c"+string(rune('0'+scopeIdx)))
		}

		// Total expected: 6 + 4 + 3 = 13 messages
		messages, err := marshaler.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Len(t, messages, 13)

		// Verify message order and attribution
		// First 6 should be from service-a (3 from scope-a1, 3 from scope-a2)
		for i := 0; i < 6; i++ {
			var doc map[string]any
			err := json.Unmarshal(messages[i].Value, &doc)
			require.NoError(t, err)

			resource := doc["resource"].(map[string]any)
			assert.Equal(t, "service-a", resource["service.name"])
			assert.Equal(t, "production", resource["deployment.environment"])

			attrs := doc["attributes"].(map[string]any)
			if i < 3 {
				assert.Equal(t, "a1", attrs["origin"])
			} else {
				assert.Equal(t, "a2", attrs["origin"])
			}
		}

		// Next 4 should be from service-b
		for i := 6; i < 10; i++ {
			var doc map[string]any
			err := json.Unmarshal(messages[i].Value, &doc)
			require.NoError(t, err)

			resource := doc["resource"].(map[string]any)
			assert.Equal(t, "service-b", resource["service.name"])
			assert.Equal(t, "staging", resource["deployment.environment"])

			attrs := doc["attributes"].(map[string]any)
			assert.Equal(t, "b", attrs["origin"])
		}

		// Last 3 should be from service-c
		for i := 10; i < 13; i++ {
			var doc map[string]any
			err := json.Unmarshal(messages[i].Value, &doc)
			require.NoError(t, err)

			resource := doc["resource"].(map[string]any)
			assert.Equal(t, "service-c", resource["service.name"])

			attrs := doc["attributes"].(map[string]any)
			expectedOrigin := "c" + string(rune('0'+(i-10)))
			assert.Equal(t, expectedOrigin, attrs["origin"])
		}
	})

	t.Run("large batch processing", func(t *testing.T) {
		logs := plog.NewLogs()

		numResources := 5
		numScopesPerResource := 4
		numRecordsPerScope := 25

		expectedTotal := numResources * numScopesPerResource * numRecordsPerScope

		for r := 0; r < numResources; r++ {
			rl := logs.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("service.name", "service-"+string(rune('A'+r)))
			rl.Resource().Attributes().PutInt("resource.index", int64(r))

			for s := 0; s < numScopesPerResource; s++ {
				sl := rl.ScopeLogs().AppendEmpty()
				sl.Scope().SetName("scope-" + string(rune('0'+s)))
				sl.Scope().Attributes().PutInt("scope.index", int64(s))

				for rec := 0; rec < numRecordsPerScope; rec++ {
					record := sl.LogRecords().AppendEmpty()
					record.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
					record.Body().SetStr("Log message")
					record.SetSeverityText("INFO")
					record.Attributes().PutInt("resource.index", int64(r))
					record.Attributes().PutInt("scope.index", int64(s))
					record.Attributes().PutInt("record.index", int64(rec))
				}
			}
		}

		messages, err := marshaler.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Len(t, messages, expectedTotal)

		// Verify message integrity across the batch
		for _, msg := range messages {
			var doc map[string]any
			err := json.Unmarshal(msg.Value, &doc)
			require.NoError(t, err)

			// Each message should have required fields
			assert.Contains(t, doc, "@timestamp")
			assert.Contains(t, doc, "body")
			assert.Contains(t, doc, "severity")
			assert.Contains(t, doc, "attributes")
			assert.Contains(t, doc, "resource")
			assert.Contains(t, doc, "instrumentationScope")
		}

		// Spot check some specific messages to verify ordering
		// First message should be from resource 0, scope 0, record 0
		var firstDoc map[string]any
		json.Unmarshal(messages[0].Value, &firstDoc)
		firstAttrs := firstDoc["attributes"].(map[string]any)
		assert.Equal(t, float64(0), firstAttrs["resource.index"])
		assert.Equal(t, float64(0), firstAttrs["scope.index"])
		assert.Equal(t, float64(0), firstAttrs["record.index"])

		// Last message should be from resource 4, scope 3, record 24
		var lastDoc map[string]any
		json.Unmarshal(messages[expectedTotal-1].Value, &lastDoc)
		lastAttrs := lastDoc["attributes"].(map[string]any)
		assert.Equal(t, float64(numResources-1), lastAttrs["resource.index"])
		assert.Equal(t, float64(numScopesPerResource-1), lastAttrs["scope.index"])
		assert.Equal(t, float64(numRecordsPerScope-1), lastAttrs["record.index"])
	})

	t.Run("batch with mixed trace context", func(t *testing.T) {
		logs := plog.NewLogs()

		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "trace-service")

		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("trace-logger")

		// Record without trace context
		rec1 := sl.LogRecords().AppendEmpty()
		rec1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		rec1.Body().SetStr("No trace context")
		rec1.Attributes().PutBool("has.trace", false)

		// Record with trace context
		rec2 := sl.LogRecords().AppendEmpty()
		rec2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		rec2.Body().SetStr("With trace context")
		rec2.Attributes().PutBool("has.trace", true)
		traceID1 := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		spanID1 := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
		rec2.SetTraceID(traceID1)
		rec2.SetSpanID(spanID1)

		// Another record without trace context
		rec3 := sl.LogRecords().AppendEmpty()
		rec3.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		rec3.Body().SetStr("No trace context again")
		rec3.Attributes().PutBool("has.trace", false)

		// Another record with different trace context
		rec4 := sl.LogRecords().AppendEmpty()
		rec4.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		rec4.Body().SetStr("Different trace context")
		rec4.Attributes().PutBool("has.trace", true)
		traceID2 := [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
		spanID2 := [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
		rec4.SetTraceID(traceID2)
		rec4.SetSpanID(spanID2)

		messages, err := marshaler.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Len(t, messages, 4)

		// Verify message keys (nil for no trace, trace ID bytes for traced)
		assert.Nil(t, messages[0].Key)
		assert.Equal(t, traceID1[:], messages[1].Key)
		assert.Nil(t, messages[2].Key)
		assert.Equal(t, traceID2[:], messages[3].Key)

		// Verify trace IDs in documents
		var doc1, doc2, doc3, doc4 map[string]any
		json.Unmarshal(messages[0].Value, &doc1)
		json.Unmarshal(messages[1].Value, &doc2)
		json.Unmarshal(messages[2].Value, &doc3)
		json.Unmarshal(messages[3].Value, &doc4)

		// Records without trace context should not have traceId/spanId fields
		_, hasTraceId1 := doc1["traceId"]
		assert.False(t, hasTraceId1)
		_, hasSpanId1 := doc1["spanId"]
		assert.False(t, hasSpanId1)

		// Records with trace context should have properly formatted IDs
		assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", doc2["traceId"])
		assert.Equal(t, "0102030405060708", doc2["spanId"])

		_, hasTraceId3 := doc3["traceId"]
		assert.False(t, hasTraceId3)

		assert.Equal(t, "100f0e0d0c0b0a090807060504030201", doc4["traceId"])
		assert.Equal(t, "0807060504030201", doc4["spanId"])
	})

	t.Run("batch preserves attribute types", func(t *testing.T) {
		logs := plog.NewLogs()

		rl := logs.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "type-test-service")

		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("type-logger")

		// Record with various attribute types
		rec := sl.LogRecords().AppendEmpty()
		rec.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		rec.Body().SetStr("Type test log")
		rec.Attributes().PutStr("string.attr", "string-value")
		rec.Attributes().PutInt("int.attr", 42)
		rec.Attributes().PutDouble("double.attr", 3.14159)
		rec.Attributes().PutBool("bool.attr", true)

		// Add slice attribute
		sliceVal := rec.Attributes().PutEmptySlice("slice.attr")
		sliceVal.AppendEmpty().SetStr("item1")
		sliceVal.AppendEmpty().SetStr("item2")

		// Add map attribute
		mapVal := rec.Attributes().PutEmptyMap("map.attr")
		mapVal.PutStr("nested.key", "nested-value")
		mapVal.PutInt("nested.num", 123)

		messages, err := marshaler.MarshalLogs(logs)
		require.NoError(t, err)
		assert.Len(t, messages, 1)

		var doc map[string]any
		json.Unmarshal(messages[0].Value, &doc)

		attrs := doc["attributes"].(map[string]any)

		// Verify types are preserved through JSON marshaling
		assert.Equal(t, "string-value", attrs["string.attr"])
		assert.Equal(t, float64(42), attrs["int.attr"]) // JSON numbers are float64
		assert.InDelta(t, 3.14159, attrs["double.attr"], 0.00001)
		assert.Equal(t, true, attrs["bool.attr"])

		// Verify slice
		sliceAttr := attrs["slice.attr"].([]any)
		assert.Len(t, sliceAttr, 2)
		assert.Equal(t, "item1", sliceAttr[0])
		assert.Equal(t, "item2", sliceAttr[1])

		// Verify map
		mapAttr := attrs["map.attr"].(map[string]any)
		assert.Equal(t, "nested-value", mapAttr["nested.key"])
		assert.Equal(t, float64(123), mapAttr["nested.num"])
	})
}

// TestOpenSearchLogsMarshaler_BatchOrderPreservation verifies that the marshaler
// preserves the order of log records across resources and scopes.
func TestOpenSearchLogsMarshaler_BatchOrderPreservation(t *testing.T) {
	marshaler := &OpenSearchLogsMarshaler{
		unixTimestamps: false,
	}

	logs := plog.NewLogs()
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// Create a deterministic sequence of logs
	sequence := []struct {
		service   string
		scope     string
		message   string
		timestamp time.Time
	}{
		{"svc-1", "scope-a", "msg-1", baseTime},
		{"svc-1", "scope-a", "msg-2", baseTime.Add(1 * time.Second)},
		{"svc-1", "scope-b", "msg-3", baseTime.Add(2 * time.Second)},
		{"svc-2", "scope-c", "msg-4", baseTime.Add(3 * time.Second)},
		{"svc-2", "scope-c", "msg-5", baseTime.Add(4 * time.Second)},
		{"svc-2", "scope-d", "msg-6", baseTime.Add(5 * time.Second)},
	}

	// Build logs structure
	currentService := ""
	currentScope := ""
	var currentRL plog.ResourceLogs
	var currentSL plog.ScopeLogs

	for i, s := range sequence {
		if s.service != currentService {
			currentRL = logs.ResourceLogs().AppendEmpty()
			currentRL.Resource().Attributes().PutStr("service.name", s.service)
			currentService = s.service
			currentScope = "" // Reset scope when service changes
		}

		if s.scope != currentScope {
			currentSL = currentRL.ScopeLogs().AppendEmpty()
			currentSL.Scope().SetName(s.scope)
			currentScope = s.scope
		}

		rec := currentSL.LogRecords().AppendEmpty()
		rec.SetTimestamp(pcommon.NewTimestampFromTime(s.timestamp))
		rec.Body().SetStr(s.message)
		rec.Attributes().PutInt("sequence", int64(i))
	}

	messages, err := marshaler.MarshalLogs(logs)
	require.NoError(t, err)
	assert.Len(t, messages, len(sequence))

	// Verify messages come out in the expected order
	for i, msg := range messages {
		var doc map[string]any
		err := json.Unmarshal(msg.Value, &doc)
		require.NoError(t, err)

		expected := sequence[i]

		resource := doc["resource"].(map[string]any)
		assert.Equal(t, expected.service, resource["service.name"], "Message %d service mismatch", i)

		instrScope := doc["instrumentationScope"].(map[string]any)
		assert.Equal(t, expected.scope, instrScope["name"], "Message %d scope mismatch", i)

		assert.Equal(t, expected.message, doc["body"], "Message %d body mismatch", i)

		attrs := doc["attributes"].(map[string]any)
		assert.Equal(t, float64(i), attrs["sequence"], "Message %d sequence mismatch", i)
	}
}

// TestOpenSearchLogsMarshaler_Encoding verifies the Encoding method returns the correct value.
func TestOpenSearchLogsMarshaler_Encoding(t *testing.T) {
	marshaler := &OpenSearchLogsMarshaler{}
	assert.Equal(t, "opensearch_json", marshaler.Encoding())
}
