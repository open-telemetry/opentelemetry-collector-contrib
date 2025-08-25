// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestMetricsJSONLMarshaler(t *testing.T) {
	marshaler := &metricsJSONLMarshaler{}
	md := testdata.GenerateMetricsTwoMetrics()

	result, err := marshaler.MarshalMetrics(md)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Check that result contains multiple JSON lines
	lines := strings.Split(strings.TrimSpace(string(result)), "\n")
	assert.Greater(t, len(lines), 1, "Expected multiple JSON lines")

	// Verify each line is valid JSON
	for i, line := range lines {
		var parsed map[string]any
		err := json.Unmarshal([]byte(line), &parsed)
		assert.NoErrorf(t, err, "Line %d is not valid JSON: %s", i, line)

		// Verify structure contains expected fields
		assert.Contains(t, parsed, "resourceMetrics", "Line %d missing resourceMetrics", i)
	}
}

func TestTracesJSONLMarshaler(t *testing.T) {
	marshaler := &tracesJSONLMarshaler{}
	td := testdata.GenerateTracesTwoSpansSameResource()

	result, err := marshaler.MarshalTraces(td)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Check that result contains multiple JSON lines
	lines := strings.Split(strings.TrimSpace(string(result)), "\n")
	assert.Greater(t, len(lines), 1, "Expected multiple JSON lines")

	// Verify each line is valid JSON
	for i, line := range lines {
		var parsed map[string]any
		err := json.Unmarshal([]byte(line), &parsed)
		assert.NoErrorf(t, err, "Line %d is not valid JSON: %s", i, line)

		// Verify structure contains expected fields
		assert.Contains(t, parsed, "resourceSpans", "Line %d missing resourceSpans", i)
	}
}

func TestLogsJSONLMarshalerCompatibility(t *testing.T) {
	// Test that existing logs JSONL marshaler still works
	marshaler := &logsJSONLMarshaler{}
	ld := testdata.GenerateLogsTwoLogRecordsSameResource()

	result, err := marshaler.MarshalLogs(ld)
	require.NoError(t, err)
	require.NotEmpty(t, result)

	// Check that result contains multiple JSON lines
	lines := strings.Split(strings.TrimSpace(string(result)), "\n")
	assert.Greater(t, len(lines), 1, "Expected multiple JSON lines")

	// Verify each line is valid JSON
	for i, line := range lines {
		var parsed map[string]any
		err := json.Unmarshal([]byte(line), &parsed)
		assert.NoErrorf(t, err, "Line %d is not valid JSON: %s", i, line)

		// Verify structure contains expected fields
		assert.Contains(t, parsed, "resourceLogs", "Line %d missing resourceLogs", i)
	}
}

func TestJSONLFormatComparison(t *testing.T) {
	// Generate test data
	md := testdata.GenerateMetricsTwoMetrics()

	// Marshal with regular JSON
	jsonMarshaler := &pmetric.JSONMarshaler{}
	jsonResult, err := jsonMarshaler.MarshalMetrics(md)
	require.NoError(t, err)

	// Marshal with JSONL
	jsonlMarshaler := &metricsJSONLMarshaler{}
	jsonlResult, err := jsonlMarshaler.MarshalMetrics(md)
	require.NoError(t, err)

	// JSONL should be different from regular JSON (contains newlines)
	assert.NotEqual(t, jsonResult, jsonlResult)
	assert.Contains(t, string(jsonlResult), "\n", "JSONL should contain newlines")

	// Count lines in JSONL output
	scanner := bufio.NewScanner(bytes.NewReader(jsonlResult))
	lineCount := 0
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			lineCount++
		}
	}
	assert.Positive(t, lineCount, "Should have at least one line")
}
