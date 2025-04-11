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
)

const (
	filesDirectory = "testdata"
)

func TestCloudTrailLogsUnmarshaler_Unmarshal_MultipleRecords(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})
	data, err := os.ReadFile(filepath.Join(filesDirectory, "cloudtrail_logs.json"))
	require.NoError(t, err)

	actualLogs, err := unmarshaler.UnmarshalLogs(data)
	require.NoError(t, err)

	expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, "cloudtrail_logs_expected.yaml"))
	require.NoError(t, err)

	// Compare logs ignoring the order of resource logs
	compareLogsIgnoringResourceOrder(t, expectedLogs, actualLogs)
}

// compressData compresses the input data using gzip compression
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

	// Compress the data
	compressedData, err := compressData(data)
	require.NoError(t, err)

	// Unmarshal the compressed data
	actualLogs, err := unmarshaler.UnmarshalLogs(compressedData)
	require.NoError(t, err)

	expectedLogs, err := golden.ReadLogs(filepath.Join(filesDirectory, "cloudtrail_logs_expected.yaml"))
	require.NoError(t, err)

	// Compare logs ignoring the order of resource logs
	compareLogsIgnoringResourceOrder(t, expectedLogs, actualLogs)
}

// compareLogsIgnoringResourceOrder compares logs ignoring the order of resource logs
func compareLogsIgnoringResourceOrder(t *testing.T, expected, actual plog.Logs) {
	// Check that the number of resource logs is the same
	require.Equal(t, expected.ResourceLogs().Len(), actual.ResourceLogs().Len(), "Number of resource logs doesn't match")

	// Create maps of resource logs by account ID for both expected and actual
	expectedResourcesByAccount := mapResourceLogsByAccount(expected.ResourceLogs())
	actualResourcesByAccount := mapResourceLogsByAccount(actual.ResourceLogs())

	// Check that the account IDs are the same
	require.Len(t, actualResourcesByAccount, len(expectedResourcesByAccount), "Number of unique account IDs doesn't match")

	// For each account ID in expected, check that there's a matching resource log in actual
	for accountID, expectedRL := range expectedResourcesByAccount {
		actualRL, ok := actualResourcesByAccount[accountID]
		require.True(t, ok, "Missing resource log for account ID %s", accountID)

		// Compare the resource logs for this account ID
		compareResourceLogs(t, expectedRL, actualRL, accountID)
	}
}

// mapResourceLogsByAccount creates a map of resource logs by account ID
func mapResourceLogsByAccount(rl plog.ResourceLogsSlice) map[string]plog.ResourceLogs {
	result := make(map[string]plog.ResourceLogs)
	for i := range rl.Len() {
		resourceLog := rl.At(i)
		accountID, found := resourceLog.Resource().Attributes().Get("cloud.account.id")
		if found {
			result[accountID.AsString()] = resourceLog
		}
	}
	return result
}

// compareResourceLogs compares two resource logs
func compareResourceLogs(t *testing.T, expected, actual plog.ResourceLogs, accountID string) {
	// Compare resource attributes
	require.Equal(t, expected.Resource().Attributes().Len(), actual.Resource().Attributes().Len(),
		"Number of resource attributes doesn't match for account ID %s", accountID)

	// Compare scope logs
	require.Equal(t, expected.ScopeLogs().Len(), actual.ScopeLogs().Len(),
		"Number of scope logs doesn't match for account ID %s", accountID)

	for i := 0; i < expected.ScopeLogs().Len(); i++ {
		expectedSL := expected.ScopeLogs().At(i)
		actualSL := actual.ScopeLogs().At(i)

		// Compare scope
		require.Equal(t, expectedSL.Scope().Name(), actualSL.Scope().Name(),
			"Scope name doesn't match for account ID %s", accountID)
		require.Equal(t, expectedSL.Scope().Version(), actualSL.Scope().Version(),
			"Scope version doesn't match for account ID %s", accountID)

		// Compare log records
		require.Equal(t, expectedSL.LogRecords().Len(), actualSL.LogRecords().Len(),
			"Number of log records doesn't match for account ID %s", accountID)

		// For each log record in expected, find a matching log record in actual
		for j := 0; j < expectedSL.LogRecords().Len(); j++ {
			expectedLR := expectedSL.LogRecords().At(j)

			// Find a matching log record in actual
			matchFound := false
			for k := 0; k < actualSL.LogRecords().Len(); k++ {
				actualLR := actualSL.LogRecords().At(k)

				// Check if this is a match by comparing event ID
				expectedEventID, expectedOk := expectedLR.Attributes().Get("request.event_id")
				actualEventID, actualOk := actualLR.Attributes().Get("request.event_id")

				if expectedOk && actualOk && expectedEventID.AsString() == actualEventID.AsString() {
					// Found a match, compare the log records
					require.Equal(t, expectedLR.Timestamp(), actualLR.Timestamp(),
						"Timestamp doesn't match for log record with event ID %s", expectedEventID.AsString())
					require.Equal(t, expectedLR.Attributes().Len(), actualLR.Attributes().Len(),
						"Number of attributes doesn't match for log record with event ID %s", expectedEventID.AsString())

					matchFound = true
					break
				}
			}

			eventID, ok := expectedLR.Attributes().Get("request.event_id")
			eventIDStr := "unknown"
			if ok {
				eventIDStr = eventID.AsString()
			}
			require.True(t, matchFound, "No matching log record found for event ID %s", eventIDStr)
		}
	}
}
