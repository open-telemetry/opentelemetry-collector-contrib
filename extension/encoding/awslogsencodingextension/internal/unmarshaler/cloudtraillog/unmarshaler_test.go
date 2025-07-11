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
	unmarshaler := NewCloudTrailLogUnmarshaler(component.BuildInfo{Version: "test-version"})
	reader := readLogFile(t, filesDirectory, "cloudtrail_log.json")
	logs, err := unmarshaler.UnmarshalAWSLogs(reader)
	require.NoError(t, err)

	// Read the expected logs from the file
	expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, "cloudtrail_log_expected.yaml"))
	require.NoError(t, err)

	compareOptions := []plogtest.CompareLogsOption{
		plogtest.IgnoreResourceLogsOrder(),
		plogtest.IgnoreScopeLogsOrder(),
		plogtest.IgnoreLogRecordsOrder(),
	}

	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, compareOptions...))
}

func TestCloudTrailLogUnmarshaler_UnmarshalAWSLogs_EmptyRecords(t *testing.T) {
	t.Parallel()
	unmarshaler := NewCloudTrailLogUnmarshaler(component.BuildInfo{Version: "test-version"})
	reader := bytes.NewReader([]byte(`{"Records": []}`))
	logs, err := unmarshaler.UnmarshalAWSLogs(reader)
	require.NoError(t, err)
	require.Equal(t, 0, logs.ResourceLogs().Len())
}

func TestCloudTrailLogUnmarshaler_UnmarshalAWSLogs_InvalidJSON(t *testing.T) {
	t.Parallel()
	unmarshaler := NewCloudTrailLogUnmarshaler(component.BuildInfo{Version: "test-version"})
	reader := bytes.NewReader([]byte(`{invalid-json}`))
	_, err := unmarshaler.UnmarshalAWSLogs(reader)
	require.ErrorContains(t, err, "failed to unmarshal CloudTrail logs")
}

func TestCloudTrailLogUnmarshaler_UnmarshalAWSLogs_InvalidTimestamp(t *testing.T) {
	t.Parallel()
	unmarshaler := NewCloudTrailLogUnmarshaler(component.BuildInfo{Version: "test-version"})
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
	unmarshaler := NewCloudTrailLogUnmarshaler(component.BuildInfo{Version: "test-version"})
	reader := &errorReader{err: errors.New("read failed")}
	_, err := unmarshaler.UnmarshalAWSLogs(reader)
	require.ErrorContains(t, err, "failed to read CloudTrail logs")
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

// errorReader is a reader that always returns an error
type errorReader struct {
	err error
}

func (r *errorReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

func readLogFile(t *testing.T, dir string, file string) io.Reader {
	data, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(t, err)
	return bytes.NewReader(data)
}
