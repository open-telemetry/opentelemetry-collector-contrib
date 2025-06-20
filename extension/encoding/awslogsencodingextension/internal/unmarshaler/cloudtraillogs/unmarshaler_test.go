// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillogs

import (
	"bytes"
	"compress/gzip"
	"errors"
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

func TestCloudTrailLogsUnmarshaler_UnmarshalAWSLogs(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})
	data, err := os.ReadFile(filepath.Join(filesDirectory, "cloudtrail_logs.json"))
	require.NoError(t, err)

	// Test with io.Reader
	reader := bytes.NewReader(data)
	actualLogs, err := unmarshaler.UnmarshalAWSLogs(reader)
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

	eventIDVal, exists := attrs1.Get("aws.cloudtrail.event_id")
	require.True(t, exists)
	require.Equal(t, "e755e09c-42f9-4c5c-9064-EXAMPLE228c7", eventIDVal.Str())
}

func TestCloudTrailLogsUnmarshaler_UnmarshalAWSLogs_ReadError(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})

	// Create a reader that will return an error
	errReader := &errorReader{err: errors.New("read error")}

	// This should return an error
	_, err := unmarshaler.UnmarshalAWSLogs(errReader)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read CloudTrail logs")
}

// errorReader is a mock reader that always returns an error
type errorReader struct {
	err error
}

func (r *errorReader) Read(_ []byte) (n int, err error) {
	return 0, r.err
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

func TestCloudTrailLogsUnmarshaler_UnmarshalLogs_InvalidGzip(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})

	// Create invalid gzip data
	invalidGzip := []byte("not a gzip file")

	// This should return an error
	_, err := unmarshaler.UnmarshalLogs(invalidGzip)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create gzip reader")
}

func TestCloudTrailLogsUnmarshaler_UnmarshalLogs_InvalidJSON(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})

	// Create valid gzip but invalid JSON
	invalidJSON := "not valid json"
	compressedData, err := compressData([]byte(invalidJSON))
	require.NoError(t, err)

	// This should return an error
	_, err = unmarshaler.UnmarshalLogs(compressedData)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal CloudTrail logs")
}

func TestCloudTrailLogsUnmarshaler_SetResourceAttributes(t *testing.T) {
	unmarshaler := NewCloudTrailLogsUnmarshaler(component.BuildInfo{
		Version: "1.0.0",
	})

	// Create a test record
	record := CloudTrailRecord{
		EventSource:        "ec2.amazonaws.com",
		AwsRegion:          "us-east-1",
		RecipientAccountID: "123456789012",
	}

	// Create a map to store attributes
	attrs := pcommon.NewMap()

	// Call setResourceAttributes
	unmarshaler.setResourceAttributes(attrs, record)

	// Verify attributes were set correctly
	val, ok := attrs.Get(string(conventions.CloudProviderKey))
	require.True(t, ok)
	require.Equal(t, conventions.CloudProviderAWS.Value.AsString(), val.Str())

	val, ok = attrs.Get(string(conventions.CloudRegionKey))
	require.True(t, ok)
	require.Equal(t, "us-east-1", val.Str())

	val, ok = attrs.Get(string(conventions.CloudAccountIDKey))
	require.True(t, ok)
	require.Equal(t, "123456789012", val.Str())
}

func TestAddAttributeValue(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    any
		expected func(pcommon.Map) bool
	}{
		{
			name:  "string value",
			key:   "stringKey",
			value: "stringValue",
			expected: func(m pcommon.Map) bool {
				v, ok := m.Get("stringKey")
				return ok && v.Type() == pcommon.ValueTypeStr && v.Str() == "stringValue"
			},
		},
		{
			name:  "bool value",
			key:   "boolKey",
			value: true,
			expected: func(m pcommon.Map) bool {
				v, ok := m.Get("boolKey")
				return ok && v.Type() == pcommon.ValueTypeBool && v.Bool()
			},
		},
		{
			name:  "float64 value",
			key:   "floatKey",
			value: 123.45,
			expected: func(m pcommon.Map) bool {
				v, ok := m.Get("floatKey")
				return ok && v.Type() == pcommon.ValueTypeDouble && v.Double() == 123.45
			},
		},
		{
			name:  "int value",
			key:   "intKey",
			value: 42,
			expected: func(m pcommon.Map) bool {
				v, ok := m.Get("intKey")
				return ok && v.Type() == pcommon.ValueTypeInt && v.Int() == 42
			},
		},
		{
			name:  "int64 value",
			key:   "int64Key",
			value: int64(9223372036854775807),
			expected: func(m pcommon.Map) bool {
				v, ok := m.Get("int64Key")
				return ok && v.Type() == pcommon.ValueTypeInt && v.Int() == 9223372036854775807
			},
		},
		{
			name:  "map value",
			key:   "mapKey",
			value: map[string]any{"nestedKey": "nestedValue"},
			expected: func(m pcommon.Map) bool {
				v, ok := m.Get("mapKey")
				if !ok || v.Type() != pcommon.ValueTypeMap {
					return false
				}
				nv, ok := v.Map().Get("nestedKey")
				return ok && nv.Type() == pcommon.ValueTypeStr && nv.Str() == "nestedValue"
			},
		},
		{
			name:  "slice value",
			key:   "sliceKey",
			value: []any{"item1", 42, true, map[string]any{"k": "v"}},
			expected: func(m pcommon.Map) bool {
				v, ok := m.Get("sliceKey")
				if !ok || v.Type() != pcommon.ValueTypeSlice {
					return false
				}
				slice := v.Slice()
				if slice.Len() != 4 {
					return false
				}
				return slice.At(0).Str() == "item1" &&
					slice.At(1).Int() == 42 &&
					slice.At(2).Bool() &&
					slice.At(3).Map().Len() == 1
			},
		},
		{
			name:  "unsupported value",
			key:   "unsupportedKey",
			value: complex(1, 2), // complex type is not supported
			expected: func(m pcommon.Map) bool {
				_, ok := m.Get("unsupportedKey")
				return !ok // Key should not be added
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := pcommon.NewMap()
			addAttributeValue(m, tt.key, tt.value)
			require.True(t, tt.expected(m))
		})
	}
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
