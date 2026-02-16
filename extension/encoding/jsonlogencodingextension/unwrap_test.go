// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestExtractRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		jsonPath    string
		wantCount   int
		wantErr     string
		wantRecords []string
	}{
		{
			name:      "object records with $.records[*]",
			input:     `{"records": [{"id": 1}, {"id": 2}]}`,
			jsonPath:  "$.records[*]",
			wantCount: 2,
		},
		{
			name:      "JSON array with $[*]",
			input:     `[{"id": 1}, {"id": 2}, {"id": 3}]`,
			jsonPath:  "$[*]",
			wantCount: 3,
		},
		{
			name:      "NDJSON fallback",
			input:     "{\"a\":1}\n{\"b\":2}\n{\"c\":3}",
			jsonPath:  "$.records[*]",
			wantCount: 3,
		},
		{
			name:      "NDJSON with empty lines",
			input:     "{\"a\":1}\n\n{\"b\":2}\n",
			jsonPath:  "$.records[*]",
			wantCount: 2,
		},
		{
			name:      "single object matching $[*] on array",
			input:     `[{"id": 1}]`,
			jsonPath:  "$[*]",
			wantCount: 1,
		},
		{
			name:      "nested path",
			input:     `{"data": {"items": [{"id": 1}, {"id": 2}]}}`,
			jsonPath:  "$.data.items[*]",
			wantCount: 2,
		},
		{
			name:      "custom field name",
			input:     `{"events": [{"type": "click"}, {"type": "view"}]}`,
			jsonPath:  "$.events[*]",
			wantCount: 2,
		},
		{
			name:      "empty input",
			input:     "",
			jsonPath:  "$.records[*]",
			wantCount: 0,
		},
		{
			name:      "whitespace only input",
			input:     "   \n  ",
			jsonPath:  "$.records[*]",
			wantCount: 0,
		},
		{
			name:    "invalid input",
			input:   "not json at all",
			jsonPath: "$.records[*]",
			wantErr: "unsupported input format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			records, err := extractRecords([]byte(tt.input), tt.jsonPath)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Len(t, records, tt.wantCount)
		})
	}
}

// set to true to regenerate golden files, then set back to false
var updateGoldenFiles = false

func TestUnmarshalLogsWithUnwrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		unwrap   string
		input    string
		wantLogs int
		logsPath string
	}{
		{
			name:     "unwrap object records",
			unwrap:   "$.records[*]",
			input:    `{"records": [{"level": "info", "message": "hello"}, {"level": "error", "message": "oops"}]}`,
			wantLogs: 2,
			logsPath: filepath.Join(testDataDir, "unwrap_object_records.yml"),
		},
		{
			name:     "unwrap JSON array",
			unwrap:   "$[*]",
			input:    `[{"level": "info", "message": "hello"}, {"level": "error", "message": "oops"}]`,
			wantLogs: 2,
			logsPath: filepath.Join(testDataDir, "unwrap_json_array.yml"),
		},
		{
			name:     "unwrap NDJSON fallback",
			unwrap:   "$.records[*]",
			input:    "{\"level\":\"info\",\"message\":\"hello\"}\n{\"level\":\"error\",\"message\":\"oops\"}",
			wantLogs: 2,
			logsPath: filepath.Join(testDataDir, "unwrap_ndjson.yml"),
		},
		{
			name:     "unwrap nested path",
			unwrap:   "$.data.items[*]",
			input:    `{"data": {"items": [{"level": "info", "message": "hello"}, {"level": "error", "message": "oops"}]}}`,
			wantLogs: 2,
			logsPath: filepath.Join(testDataDir, "unwrap_nested_path.yml"),
		},
		{
			name:     "unwrap custom field",
			unwrap:   "$.events[*]",
			input:    `{"events": [{"type": "click", "page": "/home"}]}`,
			wantLogs: 1,
			logsPath: filepath.Join(testDataDir, "unwrap_custom_field.yml"),
		},
		{
			name:     "unwrap single element array",
			unwrap:   "$.records[*]",
			input:    `{"records": [{"level": "info", "message": "only one"}]}`,
			wantLogs: 1,
			logsPath: filepath.Join(testDataDir, "unwrap_single_element.yml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := &jsonLogExtension{
				config: &Config{
					Mode:   JSONEncodingModeBody,
					Unwrap: tt.unwrap,
				},
			}

			logs, err := e.UnmarshalLogs([]byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.wantLogs, logs.LogRecordCount())

			if updateGoldenFiles {
				golden.WriteLogs(t, tt.logsPath, logs)
			}

			expected, err := golden.ReadLogs(tt.logsPath)
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expected, logs, plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestUnmarshalLogsWithUnwrapTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		unwrap       string
		unwrapTarget string
		input        string
		wantLogs     int
		logsPath     string
	}{
		{
			name:         "unwrap_target message with object records",
			unwrap:       "$.records[*]",
			unwrapTarget: "message",
			input:        `{"records": [{"level": "info", "msg": "hello"}, {"level": "error", "msg": "oops"}]}`,
			wantLogs:     2,
			logsPath:     filepath.Join(testDataDir, "unwrap_content_field_message.yml"),
		},
		{
			name:         "unwrap_target custom name",
			unwrap:       "$.records[*]",
			unwrapTarget: "event",
			input:        `{"records": [{"level": "info", "msg": "hello"}]}`,
			wantLogs:     1,
			logsPath:     filepath.Join(testDataDir, "unwrap_content_field_custom.yml"),
		},
		{
			name:         "unwrap_target with NDJSON fallback",
			unwrap:       "$.records[*]",
			unwrapTarget: "message",
			input:        "{\"a\":1}\n{\"b\":2}",
			wantLogs:     2,
			logsPath:     filepath.Join(testDataDir, "unwrap_content_field_ndjson.yml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := &jsonLogExtension{
				config: &Config{
					Mode:         JSONEncodingModeBody,
					Unwrap:       tt.unwrap,
					UnwrapTarget: tt.unwrapTarget,
				},
			}

			logs, err := e.UnmarshalLogs([]byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.wantLogs, logs.LogRecordCount())

			if updateGoldenFiles {
				golden.WriteLogs(t, tt.logsPath, logs)
			}

			expected, err := golden.ReadLogs(tt.logsPath)
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expected, logs, plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestUnmarshalLogsWithUnwrap_ObservedTimestamp(t *testing.T) {
	t.Parallel()
	e := &jsonLogExtension{
		config: &Config{
			Mode:   JSONEncodingModeBody,
			Unwrap: "$.records[*]",
		},
	}

	logs, err := e.UnmarshalLogs([]byte(`{"records": [{"id": 1}]}`))
	require.NoError(t, err)
	require.Equal(t, 1, logs.LogRecordCount())

	lr := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.NotZero(t, lr.ObservedTimestamp(), "ObservedTimestamp should be set")
}

func TestUnmarshalLogsWithUnwrap_EmptyInput(t *testing.T) {
	t.Parallel()
	e := &jsonLogExtension{
		config: &Config{
			Mode:   JSONEncodingModeBody,
			Unwrap: "$.records[*]",
		},
	}

	logs, err := e.UnmarshalLogs([]byte(""))
	require.NoError(t, err)
	assert.Equal(t, 0, logs.LogRecordCount())
}

func TestUnmarshalLogsWithUnwrap_InvalidJSON(t *testing.T) {
	t.Parallel()
	e := &jsonLogExtension{
		config: &Config{
			Mode:   JSONEncodingModeBody,
			Unwrap: "$.records[*]",
		},
	}

	_, err := e.UnmarshalLogs([]byte("not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported input format")
}

func TestConfigValidation_Unwrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		unwrap  string
		wantErr string
	}{
		{
			name:   "valid JSONPath records",
			unwrap: "$.records[*]",
		},
		{
			name:   "valid JSONPath array",
			unwrap: "$[*]",
		},
		{
			name:   "valid JSONPath nested",
			unwrap: "$.data.items[*]",
		},
		{
			name:   "empty unwrap is valid",
			unwrap: "",
		},
		{
			name:    "missing $ prefix",
			unwrap:  "records[*]",
			wantErr: "must be a JSONPath starting with $",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Config{
				Mode:   JSONEncodingModeBody,
				Unwrap: tt.unwrap,
			}
			err := c.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestConfigValidation_UnwrapTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		unwrap       string
		unwrapTarget string
		wantErr      string
	}{
		{
			name:         "unwrap_target with unwrap is valid",
			unwrap:       "$.records[*]",
			unwrapTarget: "message",
		},
		{
			name:         "unwrap_target without unwrap is invalid",
			unwrap:       "",
			unwrapTarget: "message",
			wantErr:      "unwrap_target requires unwrap to be set",
		},
		{
			name:         "empty unwrap_target is always valid",
			unwrap:       "",
			unwrapTarget: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Config{
				Mode:         JSONEncodingModeBody,
				Unwrap:       tt.unwrap,
				UnwrapTarget: tt.unwrapTarget,
			}
			err := c.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
