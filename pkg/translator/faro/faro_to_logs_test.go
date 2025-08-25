// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/xxh3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestTranslateToLogs(t *testing.T) {
	testcases := []struct {
		name             string
		faroPayload      faroTypes.Payload
		expectedLogsFile string
		wantErr          assert.ErrorAssertionFunc
	}{
		{
			name:             "Empty payload",
			faroPayload:      faroTypes.Payload{},
			expectedLogsFile: filepath.Join("testdata", "empty-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
		{
			name:             "Standard payload",
			faroPayload:      PayloadFromFile(t, "standard-payload/payload.json"),
			expectedLogsFile: filepath.Join("testdata", "standard-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
		{
			name:             "Payload with browser brands as slice",
			faroPayload:      PayloadFromFile(t, "browser-brand-slice-payload/payload.json"),
			expectedLogsFile: filepath.Join("testdata", "browser-brand-slice-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
		{
			name:             "Payload with browser brands as string",
			faroPayload:      PayloadFromFile(t, "browser-brand-string-payload/payload.json"),
			expectedLogsFile: filepath.Join("testdata", "browser-brand-string-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
		{
			name:             "Payload with actions",
			faroPayload:      PayloadFromFile(t, "actions-payload/payload.json"),
			expectedLogsFile: filepath.Join("testdata", "actions-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			actualLogs, err := TranslateToLogs(t.Context(), tt.faroPayload)
			if !tt.wantErr(t, err) {
				return
			}
			expectedLogs, err := golden.ReadLogs(tt.expectedLogsFile)
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, actualLogs, plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestTranslateFromFaroToOTLPAndBack(t *testing.T) {
	faroPayload := PayloadFromFile(t, "general/payload.json")
	// Translate from faro payload to otlp logs
	actualLogs, err := TranslateToLogs(t.Context(), faroPayload)
	require.NoError(t, err)

	// Translate from faro payload to otlp traces
	actualTraces, err := TranslateToTraces(t.Context(), faroPayload)
	require.NoError(t, err)

	// Translate from otlp logs to faro payload
	faroLogsPayloads, err := TranslateFromLogs(t.Context(), actualLogs)
	require.NoError(t, err)

	// Translate from otlp traces to faro payload
	faroTracesPayloads, err := TranslateFromTraces(t.Context(), actualTraces)
	require.NoError(t, err)

	// Combine the traces and logs faro payloads into a single faro payload
	expectedFaroPayload := faroLogsPayloads[0]
	expectedFaroPayload.Traces = faroTracesPayloads[0].Traces

	// Compare the original faro payload with the translated faro payload
	require.Equal(t, expectedFaroPayload, faroPayload)
}

func TestTranslateFromOTLPToFaroAndBack(t *testing.T) {
	logs, err := golden.ReadLogs(filepath.Join("testdata", "general", "plogs.yaml"))
	require.NoError(t, err)

	traces, err := golden.ReadTraces(filepath.Join("testdata", "general", "ptraces.yaml"))
	require.NoError(t, err)

	// Translate from otlp logs to faro payload
	actualFaroLogsPayloads, err := TranslateFromLogs(t.Context(), logs)
	require.NoError(t, err)

	// Translate from otlp traces to faro payload
	actualFaroTracesPayloads, err := TranslateFromTraces(t.Context(), traces)
	require.NoError(t, err)

	// Combine the traces and logs faro payloads into a single faro payload
	actualFaroPayload := actualFaroLogsPayloads[0]
	actualFaroPayload.Traces = actualFaroTracesPayloads[0].Traces

	// Translate from faro payload to otlp logs
	expectedLogs, err := TranslateToLogs(t.Context(), actualFaroPayload)
	require.NoError(t, err)

	// Compare the original otlp logs with the translated otlp logs
	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreObservedTimestamp()))

	// Translate from faro payload to otlp traces
	expectedTraces, err := TranslateToTraces(t.Context(), actualFaroPayload)
	require.NoError(t, err)

	// Compare the original otlp traces with the translated otlp traces
	require.Equal(t, expectedTraces, traces)
}

func TestDrainExceptionValue(t *testing.T) {
	testcases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Property access error - find",
			input:    "Cannot read property 'find' of undefined",
			expected: "Cannot read property '<PROPERTY>' of undefined",
		},
		{
			name:     "Property access error - data",
			input:    "Cannot read property 'data' of undefined",
			expected: "Cannot read property '<PROPERTY>' of undefined",
		},
		{
			name:     "Property access error - name",
			input:    "Cannot read property 'name' of null",
			expected: "Cannot read property '<PROPERTY>' of null",
		},
		{
			name:     "Properties access error - multiple",
			input:    "Cannot read properties 'length' of undefined",
			expected: "Cannot read properties '<PROPERTY>' of undefined",
		},
		{
			name:     "Memory address in error",
			input:    "Error at Object.0x1a2b3c4d in function",
			expected: "Error at Object.<ADDRESS> in function",
		},
		{
			name:     "Multiple memory addresses",
			input:    "Stack trace: 0xabc123 -> 0xdef456",
			expected: "Stack trace: <ADDRESS> -> <ADDRESS>",
		},
		{
			name:     "UUID in error message",
			input:    "User 123e4567-e89b-12d3-a456-426614174000 not found",
			expected: "User <UUID> not found",
		},
		{
			name:     "Numeric ID patterns",
			input:    "Request failed for user id: 12345",
			expected: "Request failed for user id <ID>",
		},
		{
			name:     "Numeric ID patterns - uppercase",
			input:    "Entity ID = 98765 does not exist",
			expected: "Entity ID <ID> does not exist",
		},
		{
			name:     "Timestamp in error",
			input:    "Event occurred at 2023-11-16T10:00:55",
			expected: "Event occurred at <TIMESTAMP>",
		},
		{
			name:     "URL in error message - http",
			input:    "Failed to load script from http://example.com/static/js/app.js",
			expected: "Failed to load script from <URL>",
		},
		{
			name:     "URL in error message - https",
			input:    "Request failed to https://api.example.com/users/123",
			expected: "Request failed to <URL>",
		},
		{
			name:     "Multiple URLs in error",
			input:    "Redirect from http://old.example.com to https://new.example.com failed",
			expected: "Redirect from <URL> to <URL> failed",
		},
		{
			name:     "File path in error",
			input:    "Error in /static/js/main.chunk.js at line 42",
			expected: "Error in <PATH> at line 42",
		},
		{
			name:     "Windows file path",
			input:    "Failed to load C:\\Users\\test\\app.js",
			expected: "Failed to load <PATH>",
		},
		{
			name:     "URL takes precedence over file path",
			input:    "Error loading https://cdn.example.com/assets/main.js from server",
			expected: "Error loading <URL> from server",
		},
		{
			name:     "HTTP API error with query parameters",
			input:    "Response not ok. Status code: 500. Status text: ''. Url: https://api.example.com/products?currencyCode=USD&productIds=1YMWWN1N4O&sessionId=6d8e094c-a708-4ef4-bf22-526d563ba5b6",
			expected: "Response not ok. Status code: 500. Status text: ''. Url: <URL>",
		},
		{
			name:     "Complex error with multiple patterns",
			input:    "Cannot read property 'data' of undefined at /app/src/components/UserList.jsx:123 for user ID: 54321",
			expected: "Cannot read property '<PROPERTY>' of undefined at <PATH>:123 for user ID <ID>",
		},
		{
			name:     "No patterns to drain",
			input:    "Generic error occurred",
			expected: "Generic error occurred",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			result := drainExceptionValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDrainExceptionValueGrouping(t *testing.T) {
	// Test that similar exceptions produce the same hash
	exception1 := "Cannot read property 'find' of undefined"
	exception2 := "Cannot read property 'data' of undefined"
	exception3 := "Cannot read property 'name' of undefined"

	drained1 := drainExceptionValue(exception1)
	drained2 := drainExceptionValue(exception2)
	drained3 := drainExceptionValue(exception3)

	hash1 := xxh3.HashString(drained1)
	hash2 := xxh3.HashString(drained2)
	hash3 := xxh3.HashString(drained3)

	// All should have the same hash after draining
	assert.Equal(t, hash1, hash2, "Similar property access errors should have same hash")
	assert.Equal(t, hash1, hash3, "Similar property access errors should have same hash")

	// Test that different exception types produce different hashes
	differentException := "ReferenceError: variable is not defined"
	hashDifferent := xxh3.HashString(drainExceptionValue(differentException))

	assert.NotEqual(t, hash1, hashDifferent, "Different exception types should have different hashes")
}

func PayloadFromFile(t *testing.T, filename string) faroTypes.Payload {
	t.Helper()

	f, err := os.Open(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	var p faroTypes.Payload
	if err := json.NewDecoder(f).Decode(&p); err != nil {
		t.Fatal(err)
	}

	return p
}
