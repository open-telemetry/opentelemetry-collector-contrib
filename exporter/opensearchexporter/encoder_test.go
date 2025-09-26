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
		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				for i := 0; i < operationsPerGoroutine; i++ {
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
