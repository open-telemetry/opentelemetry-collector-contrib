// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/azure/resourcelogs"

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.13.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

var testBuildInfo = component.BuildInfo{
	Version: "1.2.3",
}

var minimumLogRecord = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := asTimestamp("2022-11-11T04:48:27.6767145Z")
	lr.SetTimestamp(ts)
	lr.Attributes().PutStr(azureOperationName, "SecretGet")
	lr.Attributes().PutStr(azureCategory, "AuditEvent")
	lr.Attributes().PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	return lr
}()

var maximumLogRecord = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := asTimestamp("2022-11-11T04:48:27.6767145Z")
	lr.SetTimestamp(ts)
	lr.SetSeverityNumber(plog.SeverityNumberWarn)
	lr.SetSeverityText("Warning")
	guid := "607964b6-41a5-4e24-a5db-db7aab3b9b34"

	lr.Attributes().PutStr(azureTenantID, "/TENANT_ID")
	lr.Attributes().PutStr(azureOperationName, "SecretGet")
	lr.Attributes().PutStr(azureOperationVersion, "7.0")
	lr.Attributes().PutStr(azureCategory, "AuditEvent")
	lr.Attributes().PutStr(azureCorrelationID, guid)
	lr.Attributes().PutStr(azureResultType, "Success")
	lr.Attributes().PutStr(azureResultSignature, "Signature")
	lr.Attributes().PutStr(azureResultDescription, "Description")
	lr.Attributes().PutInt(azureDuration, 1234)
	lr.Attributes().PutStr(conventions.AttributeNetSockPeerAddr, "127.0.0.1")
	lr.Attributes().PutStr(conventions.AttributeCloudRegion, "ukso")
	lr.Attributes().PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)

	lr.Attributes().PutEmptyMap(azureIdentity).PutEmptyMap("claim").PutStr("oid", "607964b6-41a5-4e24-a5db-db7aab3b9b34")
	m := lr.Attributes().PutEmptyMap(azureProperties)
	m.PutStr("string", "string")
	m.PutDouble("int", 429)
	m.PutDouble("float", 3.14)
	m.PutBool("bool", false)

	return lr
}()

func TestAsTimestamp(t *testing.T) {
	timestamp := "2022-11-11T04:48:27.6767145Z"
	nanos, err := asTimestamp(timestamp)
	assert.NoError(t, err)
	assert.Less(t, pcommon.Timestamp(0), nanos)

	timestamp = "invalid-time"
	nanos, err = asTimestamp(timestamp)
	assert.Error(t, err)
	assert.Equal(t, pcommon.Timestamp(0), nanos)
}

func TestAsSeverity(t *testing.T) {
	tests := map[string]plog.SeverityNumber{
		"Informational": plog.SeverityNumberInfo,
		"Warning":       plog.SeverityNumberWarn,
		"Error":         plog.SeverityNumberError,
		"Critical":      plog.SeverityNumberFatal,
		"unknown":       plog.SeverityNumberUnspecified,
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, expected, asSeverity(input))
		})
	}
}

func TestSetIf(t *testing.T) {
	m := map[string]interface{}{}

	setIf(m, "key", nil)
	actual, found := m["key"]
	assert.False(t, found)
	assert.Nil(t, actual)

	v := ""
	setIf(m, "key", &v)
	actual, found = m["key"]
	assert.False(t, found)
	assert.Nil(t, actual)

	v = "ok"
	setIf(m, "key", &v)
	actual, found = m["key"]
	assert.True(t, found)
	assert.Equal(t, "ok", actual)
}

func TestExtractRawAttributes(t *testing.T) {
	badDuration := "invalid"
	goodDuration := "1234"

	tenantID := "tenant.id"
	operationVersion := "operation.version"
	resultType := "result.type"
	resultSignature := "result.signature"
	resultDescription := "result.description"
	callerIPAddress := "127.0.0.1"
	correlationID := "edb70d1a-eec2-4b4c-b2f4-60e3510160ee"
	level := "Informational"
	location := "location"

	identity := interface{}("someone")

	properties := interface{}(map[string]interface{}{
		"a": uint64(1),
		"b": true,
		"c": 1.23,
		"d": "ok",
	})

	tests := []struct {
		name     string
		log      azureLogRecord
		expected map[string]interface{}
	}{
		{
			name: "minimal",
			log: azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
			},
			expected: map[string]interface{}{
				azureOperationName:                 "operation.name",
				azureCategory:                      "category",
				conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAzure,
			},
		},
		{
			name: "bad-duration",
			log: azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
			},
			expected: map[string]interface{}{
				azureOperationName:                 "operation.name",
				azureCategory:                      "category",
				conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAzure,
			},
		},
		{
			name: "everything",
			log: azureLogRecord{
				Time:              "",
				ResourceID:        "resource.id",
				TenantID:          &tenantID,
				OperationName:     "operation.name",
				OperationVersion:  &operationVersion,
				Category:          "category",
				ResultType:        &resultType,
				ResultSignature:   &resultSignature,
				ResultDescription: &resultDescription,
				DurationMs:        &goodDuration,
				CallerIPAddress:   &callerIPAddress,
				CorrelationID:     &correlationID,
				Identity:          &identity,
				Level:             &level,
				Location:          &location,
				Properties:        &properties,
			},
			expected: map[string]interface{}{
				azureTenantID:                        "tenant.id",
				azureOperationName:                   "operation.name",
				azureOperationVersion:                "operation.version",
				azureCategory:                        "category",
				azureCorrelationID:                   correlationID,
				azureResultType:                      "result.type",
				azureResultSignature:                 "result.signature",
				azureResultDescription:               "result.description",
				azureDuration:                        int64(1234),
				conventions.AttributeNetSockPeerAddr: "127.0.0.1",
				azureIdentity:                        "someone",
				conventions.AttributeCloudRegion:     "location",
				conventions.AttributeCloudProvider:   conventions.AttributeCloudProviderAzure,
				azureProperties:                      properties,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractRawAttributes(tt.log))
		})
	}

}

func TestUnmarshalLogs(t *testing.T) {
	expectedMinimum := plog.NewLogs()
	resourceLogs := expectedMinimum.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("otelcol/azureresourcelogs")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	lr := scopeLogs.LogRecords().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID")
	minimumLogRecord.CopyTo(lr)

	expectedMinimum2 := plog.NewLogs()
	resourceLogs = expectedMinimum2.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID")
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("otelcol/azureresourcelogs")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	logRecords := scopeLogs.LogRecords()
	lr = logRecords.AppendEmpty()
	minimumLogRecord.CopyTo(lr)
	lr = logRecords.AppendEmpty()
	minimumLogRecord.CopyTo(lr)

	expectedMaximum := plog.NewLogs()
	resourceLogs = expectedMaximum.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID")
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("otelcol/azureresourcelogs")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	lr = scopeLogs.LogRecords().AppendEmpty()
	maximumLogRecord.CopyTo(lr)

	tests := []struct {
		file     string
		expected plog.Logs
	}{
		{
			file:     "log-minimum.json",
			expected: expectedMinimum,
		},
		{
			file:     "log-minimum-2.json",
			expected: expectedMinimum2,
		},
		{
			file:     "log-maximum.json",
			expected: expectedMaximum,
		},
	}

	sut := &AzureResourceLogsUnmarshaler{
		version: testBuildInfo.Version,
		logger:  zap.NewNop(),
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.file))
			assert.NoError(t, err)
			assert.NotNil(t, data)

			logs, err := sut.UnmarshalLogs(data)
			assert.NoError(t, err)

			assert.NoError(t, plogtest.CompareLogs(tt.expected, logs))
		})
	}
}
