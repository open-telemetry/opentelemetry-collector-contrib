// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/pool"
)

// Helper function to create a bodyMapMappingModel for testing
func createBodyMapMappingModel(t *testing.T) *bodyMapMappingModel {
	t.Helper()
	bufferPool := pool.NewBufferPool()
	return &bodyMapMappingModel{
		bufferPool: bufferPool,
	}
}

// Helper function to create test log record with map body
func createLogRecordWithMapBody(t *testing.T) plog.LogRecord {
	t.Helper()
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return logRecord
}

// Helper function to create test resource and scope
func createTestResourceAndScope(t *testing.T) (pcommon.Resource, pcommon.InstrumentationScope) {
	t.Helper()
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resource := resourceLogs.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scope := scopeLogs.Scope()
	scope.SetName("test-scope")
	scope.SetVersion("1.0.0")

	return resource, scope
}

func TestBodyMapMappingModel_EncodeLog_Success(t *testing.T) {
	model := createBodyMapMappingModel(t)
	resource, scope := createTestResourceAndScope(t)

	testCases := []struct {
		name         string
		setupBody    func(plog.LogRecord)
		expectedJSON string
	}{
		{
			name: "basic map with various data types",
			setupBody: func(logRecord plog.LogRecord) {
				bodyMap := logRecord.Body().SetEmptyMap()
				bodyMap.PutStr("@timestamp", "2024-03-12T20:00:41.123456789Z")
				bodyMap.PutInt("id", 1)
				bodyMap.PutStr("key", "value")
				bodyMap.PutStr("key.a", "a")
				bodyMap.PutStr("key.a.b", "b")
				bodyMap.PutDouble("pi", 3.14)
				bodyMap.PutBool("active", true)
			},
			expectedJSON: `{
				"@timestamp": "2024-03-12T20:00:41.123456789Z",
				"id": 1,
				"key": "value",
				"key.a": "a",
				"key.a.b": "b",
				"pi": 3.14,
				"active": true
			}`,
		},
		{
			name: "empty map body",
			setupBody: func(logRecord plog.LogRecord) {
				logRecord.Body().SetEmptyMap() // Empty map
			},
			expectedJSON: `{}`,
		},
		{
			name: "map with special characters in keys",
			setupBody: func(logRecord plog.LogRecord) {
				bodyMap := logRecord.Body().SetEmptyMap()
				bodyMap.PutStr("@timestamp", "2024-03-12T20:00:00Z")
				bodyMap.PutStr("field.with.dots", "value1")
				bodyMap.PutStr("field-with-dashes", "value2")
				bodyMap.PutStr("field_with_underscores", "value3")
				bodyMap.PutStr("field with spaces", "value4")
				bodyMap.PutStr("field/with/slashes", "value5")
			},
			expectedJSON: `{
				"@timestamp": "2024-03-12T20:00:00Z",
				"field.with.dots": "value1",
				"field-with-dashes": "value2",
				"field_with_underscores": "value3",
				"field with spaces": "value4",
				"field/with/slashes": "value5"
			}`,
		},
		{
			name: "complex nested structures with arrays of objects",
			setupBody: func(logRecord plog.LogRecord) {
				bodyMap := logRecord.Body().SetEmptyMap()
				bodyMap.PutStr("operation", "user_login")
				bodyMap.PutStr("status", "failed")

				// Nested map
				errorDetails := bodyMap.PutEmptyMap("error")
				errorDetails.PutStr("type", "ConnectionError")
				errorDetails.PutInt("code", 500)

				// Nested array
				tags := bodyMap.PutEmptySlice("tags")
				tags.AppendEmpty().SetStr("database")
				tags.AppendEmpty().SetStr("critical")

				// Array of objects
				attempts := bodyMap.PutEmptySlice("login_attempts")
				for i := 1; i <= 3; i++ {
					attempt := attempts.AppendEmpty()
					attempt.SetEmptyMap()
					attemptMap := attempt.Map()
					attemptMap.PutInt("attempt_number", int64(i))
					attemptMap.PutStr("timestamp", "2024-03-12T20:0"+string(rune('0'+i-1))+":00Z")
					attemptMap.PutStr("ip_address", "192.168.1."+string(rune('0'+i)))
				}
			},
			expectedJSON: `{
				"operation": "user_login",
				"status": "failed",
				"error": {
					"type": "ConnectionError",
					"code": 500
				},
				"tags": ["database", "critical"],
				"login_attempts": [
					{
						"attempt_number": 1,
						"timestamp": "2024-03-12T20:00:00Z",
						"ip_address": "192.168.1.1"
					},
					{
						"attempt_number": 2,
						"timestamp": "2024-03-12T20:01:00Z",
						"ip_address": "192.168.1.2"
					},
					{
						"attempt_number": 3,
						"timestamp": "2024-03-12T20:02:00Z",
						"ip_address": "192.168.1.3"
					}
				]
			}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logRecord := createLogRecordWithMapBody(t)
			tc.setupBody(logRecord)

			result, err := model.encodeLog(resource, scope, "", logRecord)
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.JSONEq(t, tc.expectedJSON, string(result))
		})
	}
}

func TestBodyMapMappingModel_EncodeLog_Errors(t *testing.T) {
	model := createBodyMapMappingModel(t)
	resource, scope := createTestResourceAndScope(t)

	testCases := []struct {
		name          string
		bodyType      pcommon.ValueType
		setupBody     func(pcommon.Value)
		expectedError string
	}{
		{
			name:     "string body",
			bodyType: pcommon.ValueTypeStr,
			setupBody: func(v pcommon.Value) {
				v.SetStr("This is a string message")
			},
			expectedError: `invalid log record body type for 'bodymap' mapping mode: "Str"`,
		},
		{
			name:     "int body",
			bodyType: pcommon.ValueTypeInt,
			setupBody: func(v pcommon.Value) {
				v.SetInt(12345)
			},
			expectedError: `invalid log record body type for 'bodymap' mapping mode: "Int"`,
		},
		{
			name:     "bool body",
			bodyType: pcommon.ValueTypeBool,
			setupBody: func(v pcommon.Value) {
				v.SetBool(true)
			},
			expectedError: `invalid log record body type for 'bodymap' mapping mode: "Bool"`,
		},
		{
			name:     "double body",
			bodyType: pcommon.ValueTypeDouble,
			setupBody: func(v pcommon.Value) {
				v.SetDouble(3.14159)
			},
			expectedError: `invalid log record body type for 'bodymap' mapping mode: "Double"`,
		},
		{
			name:     "slice body",
			bodyType: pcommon.ValueTypeSlice,
			setupBody: func(v pcommon.Value) {
				slice := v.SetEmptySlice()
				slice.AppendEmpty().SetStr("item1")
				slice.AppendEmpty().SetStr("item2")
			},
			expectedError: `invalid log record body type for 'bodymap' mapping mode: "Slice"`,
		},
		{
			name:     "bytes body",
			bodyType: pcommon.ValueTypeBytes,
			setupBody: func(v pcommon.Value) {
				bytesValue := v.SetEmptyBytes()
				bytesValue.FromRaw([]byte("test bytes"))
			},
			expectedError: `invalid log record body type for 'bodymap' mapping mode: "Bytes"`,
		},
		{
			name:     "empty body",
			bodyType: pcommon.ValueTypeEmpty,
			setupBody: func(_ pcommon.Value) {
				// Empty body - no setup needed
			},
			expectedError: `invalid log record body type for 'bodymap' mapping mode: "Empty"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logRecord := createLogRecordWithMapBody(t)
			tc.setupBody(logRecord.Body())

			result, err := model.encodeLog(resource, scope, "", logRecord)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.ErrorIs(t, err, errInvalidTypeForBodyMapMode)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestBodyMapMappingModel_EncodeLog_Concurrent_Calls(t *testing.T) {
	t.Run("concurrent buffer pool usage", func(t *testing.T) {
		model := createBodyMapMappingModel(t)
		resource, scope := createTestResourceAndScope(t)

		const numGoroutines = 10
		const operationsPerGoroutine = 20

		var wg sync.WaitGroup
		errorsChan := make(chan error, numGoroutines*operationsPerGoroutine)

		// Test concurrent access to buffer pool
		for g := range numGoroutines {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				for i := range operationsPerGoroutine {
					logRecord := createLogRecordWithMapBody(t)
					bodyMap := logRecord.Body().SetEmptyMap()

					bodyMap.PutStr("goroutine", string(rune('A'+goroutineID)))
					bodyMap.PutInt("iteration", int64(i))
					bodyMap.PutStr("message", "concurrent test")

					result, err := model.encodeLog(resource, scope, "", logRecord)
					if err != nil {
						errorsChan <- fmt.Errorf("goroutine %d, iteration %d: %w", goroutineID, i, err)
						return
					}

					// Verify result
					var decoded map[string]any
					if err := json.Unmarshal(result, &decoded); err != nil {
						errorsChan <- fmt.Errorf("goroutine %d, iteration %d unmarshal: %w", goroutineID, i, err)
						return
					}

					if decoded["goroutine"] != string(rune('A'+goroutineID)) {
						errorsChan <- fmt.Errorf("goroutine %d, iteration %d: wrong goroutine ID in result", goroutineID, i)
						return
					}
				}
			}(g)
		}

		wg.Wait()
		close(errorsChan)

		// Check for any errors
		var errors []error
		for err := range errorsChan {
			errors = append(errors, err)
		}

		require.Empty(t, errors, "No errors should occur during concurrent buffer pool usage: %v", errors)
		t.Logf("Successfully completed %d concurrent operations across %d goroutines",
			numGoroutines*operationsPerGoroutine, numGoroutines)
	})
}

func TestBodyMapMappingModel_EncodeTrace(t *testing.T) {
	model := createBodyMapMappingModel(t)

	// Create minimal test data
	resource := pcommon.NewResource()
	scope := pcommon.NewInstrumentationScope()
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	// Call encodeTrace - should return error
	result, err := model.encodeTrace(resource, scope, "", span)

	// Verify error is returned and result is nil
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestOTelV1_EncodeLog(t *testing.T) {
	model := &encodeModel{otelV1: true}

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	resource := rl.Resource()
	resource.Attributes().PutStr("service.name", "test-service")
	resource.Attributes().PutInt("resource.int", 42)
	resource.Attributes().PutDouble("resource.double", 3.14)

	sl := rl.ScopeLogs().AppendEmpty()
	scope := sl.Scope()
	scope.SetName("my-scope")
	scope.SetVersion("1.2.3")
	scope.Attributes().PutStr("scope.attr", "val")

	record := sl.LogRecords().AppendEmpty()
	record.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 3, 12, 20, 0, 41, 123456789, time.UTC)))
	record.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 3, 12, 20, 0, 42, 0, time.UTC)))
	record.SetSeverityNumber(plog.SeverityNumberError)
	record.SetSeverityText("ERROR")
	record.Body().SetStr("something went wrong")
	record.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	record.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	record.SetFlags(plog.DefaultLogRecordFlags)
	record.Attributes().PutStr("log.attr", "hello")
	record.Attributes().PutInt("log.count", 5)

	result, err := model.encodeLog(resource, scope, "https://schema.example.com", record)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(result, &doc))

	// Verify top-level fields
	assert.Equal(t, "something went wrong", doc["body"])
	assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", doc["traceId"])
	assert.Equal(t, "0102030405060708", doc["spanId"])
	assert.Equal(t, float64(0), doc["flags"])
	assert.Contains(t, doc, "@timestamp")
	assert.Contains(t, doc, "time")
	assert.Contains(t, doc, "observedTime")

	// Verify severity is a nested object with numeric code
	sev := doc["severity"].(map[string]any)
	assert.Equal(t, float64(17), sev["number"]) // SeverityNumberError = 17
	assert.Equal(t, "ERROR", sev["text"])

	// Verify resource preserves types
	res := doc["resource"].(map[string]any)
	attrs := res["attributes"].(map[string]any)
	assert.Equal(t, "test-service", attrs["service.name"])
	assert.Equal(t, float64(42), attrs["resource.int"])
	assert.Equal(t, 3.14, attrs["resource.double"])
	assert.Equal(t, "https://schema.example.com", res["schemaUrl"])

	// Verify instrumentationScope
	is := doc["instrumentationScope"].(map[string]any)
	assert.Equal(t, "my-scope", is["name"])
	assert.Equal(t, "1.2.3", is["version"])
	assert.Equal(t, "https://schema.example.com", is["schemaUrl"])
	scopeAttrs := is["attributes"].(map[string]any)
	assert.Equal(t, "val", scopeAttrs["scope.attr"])

	// Verify log attributes preserve types
	logAttrs := doc["attributes"].(map[string]any)
	assert.Equal(t, "hello", logAttrs["log.attr"])
	assert.Equal(t, float64(5), logAttrs["log.count"])
}

func TestOTelV1_EncodeLog_EventName(t *testing.T) {
	model := &encodeModel{otelV1: true}

	resource := pcommon.NewResource()
	resource.Attributes().PutStr("service.name", "test-service")

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("test-scope")

	record := plog.NewLogRecord()
	record.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 3, 12, 20, 0, 41, 0, time.UTC)))
	record.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 3, 12, 20, 0, 42, 0, time.UTC)))
	record.SetSeverityNumber(plog.SeverityNumberInfo)
	record.SetSeverityText("INFO")
	record.Body().SetStr("hello world")
	record.SetEventName("ServerStartedSuccessfully")

	result, err := model.encodeLog(resource, scope, "", record)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(result, &doc))

	assert.Equal(t, "ServerStartedSuccessfully", doc["eventName"])

	// Verify eventName is omitted when empty
	record2 := plog.NewLogRecord()
	record2.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 3, 12, 20, 0, 41, 0, time.UTC)))
	record2.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 3, 12, 20, 0, 42, 0, time.UTC)))
	record2.Body().SetStr("no event name")

	result2, err := model.encodeLog(resource, scope, "", record2)
	require.NoError(t, err)

	var doc2 map[string]any
	require.NoError(t, json.Unmarshal(result2, &doc2))
	_, hasEventName := doc2["eventName"]
	assert.False(t, hasEventName, "eventName should be omitted when empty")
}

func TestOTelV1_EncodeTrace_RootSpan(t *testing.T) {
	model := &encodeModel{otelV1: true}

	resource := pcommon.NewResource()
	resource.Attributes().PutStr("service.name", "my-service")
	resource.Attributes().PutInt("host.port", 8080)

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("tracer")
	scope.SetVersion("0.1.0")

	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID([16]byte{0x8c, 0x8b, 0x17, 0x65, 0xa7, 0xb0, 0xac, 0xf0, 0xb6, 0x6a, 0xa4, 0x62, 0x3f, 0xcb, 0x7b, 0xd5})
	span.SetSpanID([8]byte{0xfd, 0x0d, 0xa8, 0x83, 0xbb, 0x27, 0xcd, 0x6b})
	// No parent span ID = root span
	span.SetName("HTTP GET /api/users")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 0, 0, 500000000, time.UTC)))
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.Status().SetMessage("success")
	span.Attributes().PutStr("http.method", "GET")
	span.Attributes().PutInt("http.status_code", 200)

	// Add an event
	event := span.Events().AppendEmpty()
	event.SetName("exception")
	event.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 0, 0, 100000000, time.UTC)))
	event.Attributes().PutStr("exception.message", "timeout")

	// Add a link
	link := span.Links().AppendEmpty()
	link.SetTraceID([16]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99})
	link.SetSpanID([8]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88})
	link.Attributes().PutStr("link.reason", "retry")

	result, err := model.encodeTrace(resource, scope, "https://schema.example.com", span)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(result, &doc))

	// Verify basic span fields
	assert.Equal(t, "8c8b1765a7b0acf0b66aa4623fcb7bd5", doc["traceId"])
	assert.Equal(t, "fd0da883bb27cd6b", doc["spanId"])
	assert.Empty(t, doc["parentSpanId"])
	assert.Equal(t, "HTTP GET /api/users", doc["name"])
	assert.Equal(t, "Server", doc["kind"])

	// Verify durationInNanos = 500ms = 500_000_000 ns
	assert.Equal(t, float64(500000000), doc["durationInNanos"])

	// Verify serviceName from resource
	assert.Equal(t, "my-service", doc["serviceName"])

	// Verify status code is integer
	status := doc["status"].(map[string]any)
	assert.Equal(t, float64(1), status["code"]) // StatusCodeOk = 1
	assert.Equal(t, "success", status["message"])

	// Verify root span has traceGroup
	assert.Equal(t, "HTTP GET /api/users", doc["traceGroup"])
	tgf := doc["traceGroupFields"].(map[string]any)
	assert.Equal(t, float64(500000000), tgf["durationInNanos"])
	assert.Equal(t, float64(1), tgf["statusCode"])
	assert.Contains(t, tgf, "endTime")

	// Verify events
	events := doc["events"].([]any)
	require.Len(t, events, 1)
	ev := events[0].(map[string]any)
	assert.Equal(t, "exception", ev["name"])
	assert.Contains(t, ev, "time")
	evAttrs := ev["attributes"].(map[string]any)
	assert.Equal(t, "timeout", evAttrs["exception.message"])

	// Verify links
	links := doc["links"].([]any)
	require.Len(t, links, 1)
	lk := links[0].(map[string]any)
	assert.Equal(t, "aabbccddeeff00112233445566778899", lk["traceId"])
	assert.Equal(t, "1122334455667788", lk["spanId"])
	lkAttrs := lk["attributes"].(map[string]any)
	assert.Equal(t, "retry", lkAttrs["link.reason"])

	// Verify resource attributes preserve types
	res := doc["resource"].(map[string]any)
	resAttrs := res["attributes"].(map[string]any)
	assert.Equal(t, "my-service", resAttrs["service.name"])
	assert.Equal(t, float64(8080), resAttrs["host.port"])

	// Verify instrumentationScope
	is := doc["instrumentationScope"].(map[string]any)
	assert.Equal(t, "tracer", is["name"])
	assert.Equal(t, "0.1.0", is["version"])
}

func TestOTelV1_EncodeTrace_NonRootSpan(t *testing.T) {
	model := &encodeModel{otelV1: true}

	resource := pcommon.NewResource()
	scope := pcommon.NewInstrumentationScope()

	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.SetParentSpanID([8]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11}) // non-empty = not root
	span.SetName("child-span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Date(2024, 1, 15, 10, 0, 1, 0, time.UTC)))
	span.Status().SetCode(ptrace.StatusCodeError)

	result, err := model.encodeTrace(resource, scope, "", span)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(result, &doc))

	// Non-root span should NOT have traceGroup
	assert.Empty(t, doc["traceGroup"])
	assert.Nil(t, doc["traceGroupFields"])

	// Verify status code Error = 2
	status := doc["status"].(map[string]any)
	assert.Equal(t, float64(2), status["code"])

	// Verify parentSpanId is set
	assert.Equal(t, "aabbccddeeff0011", doc["parentSpanId"])

	// Verify durationInNanos = 1 second
	assert.Equal(t, float64(1000000000), doc["durationInNanos"])
}
