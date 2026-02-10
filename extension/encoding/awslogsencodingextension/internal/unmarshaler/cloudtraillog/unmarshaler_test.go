// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillog

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

const filesDirectory = "testdata"

func TestCloudTrailLogUnmarshaler_UnmarshalAWSLogs_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		inputLogsFile  string
		outputLogsFile string
		userIDFeature  bool
	}{
		{
			name:           "Valid with CloudTrailUserIdentityPrefixFeatureGate disabled",
			inputLogsFile:  "cloudtrail_log.json",
			outputLogsFile: "cloudtrail_log_expected.yaml",
			userIDFeature:  false,
		},
		{
			name:           "Valid with CloudTrailUserIdentityPrefixFeatureGate enabled",
			inputLogsFile:  "cloudtrail_log.json",
			outputLogsFile: "cloudtrail_log_expected_with_uid_feature.yaml",
			userIDFeature:  true,
		},
		{
			name:           "Valid CloudWatch subscription filter format",
			inputLogsFile:  "cloudtrail_log_cw.json",
			outputLogsFile: "cloudtrail_log_cw_expected.yaml",
			userIDFeature:  true,
		},
		{
			name:           "Valid CloudWatch subscription filter format with reordered keys",
			inputLogsFile:  "cloudtrail_log_cw_reordered.json",
			outputLogsFile: "cloudtrail_log_cw_reordered_expected.yaml",
			userIDFeature:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unmarshaler := NewCloudTrailLogUnmarshaler(component.BuildInfo{Version: "test-version"}, test.userIDFeature)
			reader := readLogFile(t, filesDirectory, test.inputLogsFile)
			logs, err := unmarshaler.UnmarshalAWSLogs(reader)
			require.NoError(t, err)

			// Read the expected logs from the file
			expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, test.outputLogsFile))
			require.NoError(t, err)

			compareOptions := []plogtest.CompareLogsOption{
				plogtest.IgnoreResourceLogsOrder(),
				plogtest.IgnoreScopeLogsOrder(),
				plogtest.IgnoreLogRecordsOrder(),
			}

			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, compareOptions...))
		})
	}
}

func TestCloudTrailLogUnmarshaler_UnmarshalAWSLogs_EmptyRecords(t *testing.T) {
	t.Parallel()
	unmarshaler := NewCloudTrailLogUnmarshaler(component.BuildInfo{Version: "test-version"}, false)
	reader := readLogFile(t, filesDirectory, "cloudtrail_log_empty.json")
	logs, err := unmarshaler.UnmarshalAWSLogs(reader)
	require.NoError(t, err)

	// Read the expected logs from the file
	expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, "cloudtrail_log_empty_expected.yaml"))
	require.NoError(t, err)

	compareOptions := []plogtest.CompareLogsOption{
		plogtest.IgnoreResourceLogsOrder(),
		plogtest.IgnoreScopeLogsOrder(),
		plogtest.IgnoreLogRecordsOrder(),
	}

	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, compareOptions...))
}

func TestCloudtrailLogUnmarshaler_UnmarshalAWSDigest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		logFile      string
		expectedFile string
	}{
		{
			name:         "Empty Digest",
			logFile:      "cloudtrail_digest_empty.json",
			expectedFile: "cloudtrail_digest_empty_expected.yaml",
		},
		{
			name:         "Complete digest",
			logFile:      "cloudtrail_digest.json",
			expectedFile: "cloudtrail_digest_expected.yaml",
		},
	}

	unmarshaler := NewCloudTrailLogUnmarshaler(component.BuildInfo{Version: "test-version"}, false)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := readLogFile(t, filesDirectory, tt.logFile)
			logs, err := unmarshaler.UnmarshalAWSLogs(content)
			require.NoError(t, err)

			expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, tt.expectedFile))
			require.NoError(t, err)

			compareLogsOptions := []plogtest.CompareLogsOption{
				plogtest.IgnoreResourceLogsOrder(),
				plogtest.IgnoreScopeLogsOrder(),
				plogtest.IgnoreLogRecordsOrder(),
			}

			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, compareLogsOptions...))
		})
	}
}

func TestCloudTrailLogUnmarshaler_UnmarshalAWSLogs_InvalidJSON(t *testing.T) {
	t.Parallel()
	unmarshaler := NewCloudTrailLogUnmarshaler(component.BuildInfo{Version: "test-version"}, false)
	reader := bytes.NewReader([]byte(`{invalid-json}`))
	_, err := unmarshaler.UnmarshalAWSLogs(reader)
	require.ErrorContains(t, err, "failed to extract the first JSON key")
}

func TestCloudTrailLogUnmarshaler_UnmarshalAWSLogs_InvalidTimestamp(t *testing.T) {
	t.Parallel()
	unmarshaler := NewCloudTrailLogUnmarshaler(component.BuildInfo{Version: "test-version"}, false)
	reader := bytes.NewReader([]byte(`{
		"Records": [{
			"eventTime": "invalid-timestamp",
			"eventSource": "test.amazonaws.com",
			"eventName": "TestEvent"
		}]
	}`))
	_, err := unmarshaler.UnmarshalAWSLogs(reader)
	require.ErrorContains(t, err, "failed to parse timestamp of log")
}

func TestCloudTrailLogUnmarshaler_UnmarshalAWSLogs_ReadError(t *testing.T) {
	t.Parallel()
	unmarshaler := NewCloudTrailLogUnmarshaler(component.BuildInfo{Version: "test-version"}, false)
	reader := &errorReader{err: errors.New("read failed")}
	_, err := unmarshaler.UnmarshalAWSLogs(reader)
	require.ErrorContains(t, err, "failed to peek into CloudTrail log")
}

func TestExtractTLSVersion(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input    string
		expected string
	}{
		"TLSv1.2_format":    {"TLSv1.2", "1.2"},
		"TLSv1.3_format":    {"TLSv1.3", "1.3"},
		"TLSv1.0_format":    {"TLSv1.0", "1.0"},
		"already_extracted": {"1.2", "1.2"},
		"empty_string":      {"", ""},
		"short_string":      {"TLS", "TLS"},
		"no_TLSv_prefix":    {"SSL3.0", "SSL3.0"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := extractTLSVersion(test.input)
			require.Equal(t, test.expected, result)
		})
	}
}

func TestExtractFirstKey(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedKey string
		expectError string
	}{
		{
			name:        "Minified JSON",
			input:       `{"key", "value"}`,
			expectedKey: "key",
		},
		{
			name: "Formatted JSON",
			input: `{
						"key", "value"
					}`,
			expectedKey: "key",
		},
		{
			name: "Formatted multi-key returns first key",
			input: `{
						"keyA", "value",
						"keyB", "value"
					}`,
			expectedKey: "keyA",
		},
		{
			name:        "Key with array",
			input:       `{"key" : [] }`,
			expectedKey: "key",
		},
		{
			name:        "Invalid JSON - non JSON input",
			input:       `Key value`,
			expectError: "invalid JSON payload, failed to find the JSON opening",
		},
		{
			name:        "Invalid format - malformed with no proper elements",
			input:       `{ key }`,
			expectError: "invalid JSON payload, expected a JSON key but found none",
		},
		{
			name:        "Invalid format - incomplete JSON object",
			input:       `{ "key }`,
			expectError: "invalid JSON payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := extractFirstKey([]byte(tt.input))
			if tt.expectError != "" {
				require.ErrorContains(t, err, tt.expectError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedKey, key)
			}
		})
	}
}

func TestCloudWatchKeyOrdering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		expectKey string
	}{
		{
			name:      "messageType first",
			input:     `{"messageType":"DATA_MESSAGE","owner":"123456789010","logGroup":"/aws/cloudtrail/logs"}`,
			expectKey: "messageType",
		},
		{
			name:      "owner first",
			input:     `{"owner":"123456789010","messageType":"DATA_MESSAGE","logGroup":"/aws/cloudtrail/logs"}`,
			expectKey: "owner",
		},
		{
			name:      "logGroup first",
			input:     `{"logGroup":"/aws/cloudtrail/logs","messageType":"DATA_MESSAGE","owner":"123456789010"}`,
			expectKey: "logGroup",
		},
		{
			name:      "logStream first",
			input:     `{"logStream":"stream","messageType":"DATA_MESSAGE","owner":"123456789010"}`,
			expectKey: "logStream",
		},
		{
			name:      "subscriptionFilters first",
			input:     `{"subscriptionFilters":[],"messageType":"DATA_MESSAGE","owner":"123456789010"}`,
			expectKey: "subscriptionFilters",
		},
		{
			name:      "logEvents first",
			input:     `{"logEvents":[],"messageType":"DATA_MESSAGE","owner":"123456789010"}`,
			expectKey: "logEvents",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the first key is what we expect
			firstKey, err := extractFirstKey([]byte(tt.input))
			require.NoError(t, err)
			require.Equal(t, tt.expectKey, firstKey)

			// Verify isCloudWatchKey recognizes it
			require.True(t, isCloudWatchKey(firstKey), "isCloudWatchKey should return true for %s", firstKey)
		})
	}
}

// errorReader is a reader that always returns an error
type errorReader struct {
	err error
}

func (r *errorReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

func readLogFile(t *testing.T, dir, file string) io.Reader {
	data, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(t, err)
	return bytes.NewReader(data)
}
