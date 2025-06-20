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
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
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

	// Verify the logs structure
	require.Equal(t, 1, actualLogs.ResourceLogs().Len())
	resourceLogs := actualLogs.ResourceLogs().At(0)
	require.Equal(t, 1, resourceLogs.ScopeLogs().Len())
	scopeLogs := resourceLogs.ScopeLogs().At(0)
	require.Equal(t, 2, scopeLogs.LogRecords().Len())

	// Verify first log record (StartInstances)
	logRecord1 := scopeLogs.LogRecords().At(0)
	attrs1 := logRecord1.Attributes()

	// Check basic attributes
	eventIDVal, exists := attrs1.Get("aws.cloudtrail.event_id")
	require.True(t, exists)
	require.Equal(t, "e755e09c-42f9-4c5c-9064-EXAMPLE228c7", eventIDVal.Str())

	rpcMethodVal, exists := attrs1.Get(string(conventions.RPCMethodKey))
	require.True(t, exists)
	require.Equal(t, "StartInstances", rpcMethodVal.Str())

	rpcSystemVal, exists := attrs1.Get(string(conventions.RPCSystemKey))
	require.True(t, exists)
	require.Equal(t, "AwsApiCall", rpcSystemVal.Str())

	rpcServiceVal, exists := attrs1.Get(string(conventions.RPCServiceKey))
	require.True(t, exists)
	require.Equal(t, "ec2.amazonaws.com", rpcServiceVal.Str())

	// Verify RequestParameters is a map
	requestParams, found := attrs1.Get("aws.request.parameters")
	require.True(t, found)
	require.Equal(t, pcommon.ValueTypeMap, requestParams.Type())

	// Verify instancesSet in RequestParameters
	instancesSet, found := requestParams.Map().Get("instancesSet")
	require.True(t, found)
	require.Equal(t, pcommon.ValueTypeMap, instancesSet.Type())

	// Verify ResponseElements is a map
	responseElements, found := attrs1.Get("aws.response.elements")
	require.True(t, found)
	require.Equal(t, pcommon.ValueTypeMap, responseElements.Type())

	// Verify second log record (CreateUser)
	logRecord2 := scopeLogs.LogRecords().At(1)
	attrs2 := logRecord2.Attributes()

	// Check basic attributes
	eventID2Val, exists := attrs2.Get("aws.cloudtrail.event_id")
	require.True(t, exists)
	require.Equal(t, "ba0801a1-87ec-4d26-be87-EXAMPLE75bbb", eventID2Val.Str())

	rpcMethod2Val, exists := attrs2.Get(string(conventions.RPCMethodKey))
	require.True(t, exists)
	require.Equal(t, "CreateUser", rpcMethod2Val.Str())

	rpcSystem2Val, exists := attrs2.Get(string(conventions.RPCSystemKey))
	require.True(t, exists)
	require.Equal(t, "AwsApiCall", rpcSystem2Val.Str())

	rpcService2Val, exists := attrs2.Get(string(conventions.RPCServiceKey))
	require.True(t, exists)
	require.Equal(t, "iam.amazonaws.com", rpcService2Val.Str())

	// Verify RequestParameters is a map
	requestParams2, found := attrs2.Get("aws.request.parameters")
	require.True(t, found)
	require.Equal(t, pcommon.ValueTypeMap, requestParams2.Type())

	// Verify userName in RequestParameters
	userName, found := requestParams2.Map().Get("userName")
	require.True(t, found)
	require.Equal(t, "Richard", userName.Str())

	// Verify ResponseElements is a map
	responseElements2, found := attrs2.Get("aws.response.elements")
	require.True(t, found)
	require.Equal(t, pcommon.ValueTypeMap, responseElements2.Type())

	// Verify user in ResponseElements
	user, found := responseElements2.Map().Get("user")
	require.True(t, found)
	require.Equal(t, pcommon.ValueTypeMap, user.Type())
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
