// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillogs

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

const (
	filesDirectory = "testdata"
)

func compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	_, err := gw.Write(data)
	if err != nil {
		return nil, err
	}

	if err := gw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestCloudTrailLogsUnmarshaler_Unmarshal_MultipleRecords_Compressed(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})
	data, err := os.ReadFile(filepath.Join(filesDirectory, "cloudtrail_logs.json"))
	require.NoError(t, err)

	compressedData, err := compressData(data)
	require.NoError(t, err)

	actualLogs, err := unmarshaler.UnmarshalLogs(compressedData)
	require.NoError(t, err)

	expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, "cloudtrail_logs_expected.yaml"))
	require.NoError(t, err)

	compareLogsIgnoringResourceOrder(t, expectedLogs, actualLogs)
}

func compareLogsIgnoringResourceOrder(t *testing.T, expected, actual plog.Logs) {
	require.Positive(t, actual.ResourceLogs().Len(), "No resource logs found in actual logs")

	err := plogtest.CompareLogs(expected, actual,
		plogtest.IgnoreResourceLogsOrder(),
		plogtest.IgnoreScopeLogsOrder(),
		plogtest.IgnoreLogRecordsOrder())
	require.NoError(t, err, "Logs don't match")
}

func TestCloudTrailLogsUnmarshaler_Unmarshal_EmptyRecords(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})

	emptyRecordsJSON := `{"Records": []}`

	compressedData, err := compressData([]byte(emptyRecordsJSON))
	require.NoError(t, err)

	logs, err := unmarshaler.UnmarshalLogs(compressedData)
	require.NoError(t, err)

	require.Equal(t, 0, logs.ResourceLogs().Len(), "Expected empty logs")
}

func TestCloudTrailLogsUnmarshaler_Unmarshal_InvalidTimestamp(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})

	// Create a CloudTrailLogs with an invalid timestamp format
	invalidTimestampJSON := `{"Records": [{
		"eventVersion": "1.08",
		"eventTime": "2023-07-19T21:17:28",
		"eventSource": "ec2.amazonaws.com",
		"eventName": "StartInstances",
		"awsRegion": "us-east-1",
		"sourceIPAddress": "192.0.2.1",
		"userAgent": "aws-cli/1.16.312",
		"requestID": "12345678-1234-1234-1234-123456789012",
		"eventID": "12345678-1234-1234-1234-123456789012",
		"eventType": "AwsApiCall",
		"recipientAccountId": "123456789012"
	}]}`

	compressedData, err := compressData([]byte(invalidTimestampJSON))
	require.NoError(t, err)

	// This should return an error since the timestamp is invalid
	_, err = unmarshaler.UnmarshalLogs(compressedData)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse timestamp of log")
}

func TestExtractTLSVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "TLSv1.2 format",
			input:    "TLSv1.2",
			expected: "1.2",
		},
		{
			name:     "TLSv1.3 format",
			input:    "TLSv1.3",
			expected: "1.3",
		},
		{
			name:     "Already in version-only format",
			input:    "1.2",
			expected: "1.2",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTLSVersion(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
