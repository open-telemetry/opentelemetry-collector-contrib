// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter

/*
Contains tests for logexporter.go and log_to_envelope.go
*/

import (
	"context"
	"testing"
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

const (
	defaultEnvelopeName = "Microsoft.ApplicationInsights.Message"
	defaultdBaseType    = "MessageData"
)

var (
	testLogTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	testLogTimestamp = pcommon.NewTimestampFromTime(testLogTime)
	testStringBody   = "Message Body"
	severityLevelMap = map[plog.SeverityNumber]contracts.SeverityLevel{
		plog.SeverityNumberTrace:       contracts.Verbose,
		plog.SeverityNumberDebug4:      contracts.Verbose,
		plog.SeverityNumberInfo:        contracts.Information,
		plog.SeverityNumberInfo4:       contracts.Information,
		plog.SeverityNumberWarn:        contracts.Warning,
		plog.SeverityNumberWarn4:       contracts.Warning,
		plog.SeverityNumberError:       contracts.Error,
		plog.SeverityNumberError4:      contracts.Error,
		plog.SeverityNumberFatal:       contracts.Critical,
		plog.SeverityNumberFatal4:      contracts.Critical,
		plog.SeverityNumberUnspecified: contracts.Information,
	}
)

// Tests proper wrapping of a log record to an envelope
func TestLogRecordToEnvelope(t *testing.T) {
	ts := time.Date(2021, 12, 11, 10, 9, 8, 1, time.UTC)
	timeNow = func() time.Time {
		return ts
	}

	tests := []struct {
		name  string
		index int
	}{
		{
			name:  "timestamp is correct",
			index: 0,
		},
		{
			name:  "timestamp is empty",
			index: 1,
		},
		{
			name:  "timestamp is empty and observed timestamp is empty",
			index: 2,
		},
		{
			name:  "non-string body",
			index: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource, scope, logRecord := getTestLogRecord(tt.index)
			logPacker := getLogPacker()
			envelope := logPacker.LogRecordToEnvelope(logRecord, resource, scope)

			require.NotNil(t, envelope)
			assert.Equal(t, defaultEnvelopeName, envelope.Name)
			assert.Equal(t, toTime(timestampFromLogRecord(logRecord)).Format(time.RFC3339Nano), envelope.Time)
			require.NotNil(t, envelope.Data)
			envelopeData := envelope.Data.(*contracts.Data)
			assert.Equal(t, defaultdBaseType, envelopeData.BaseType)

			require.NotNil(t, envelopeData.BaseData)

			messageData := envelopeData.BaseData.(*contracts.MessageData)
			assert.Equal(t, logRecord.Body().AsString(), messageData.Message)
			assert.Equal(t, contracts.Information, messageData.SeverityLevel)
			assert.Equal(t, defaultTraceIDAsHex, envelope.Tags[contracts.OperationId])
			assert.Equal(t, defaultSpanIDAsHex, envelope.Tags[contracts.OperationParentId])
			assert.Contains(t, envelope.Tags[contracts.InternalSdkVersion], "otelc-")
		})
	}
}

// Test conversion from logRecord.SeverityNumber() to contracts.SeverityLevel()
func TestToAiSeverityLevel(t *testing.T) {
	logPacker := getLogPacker()
	for sn, expectedSeverityLevel := range severityLevelMap {
		severityLevel := logPacker.toAiSeverityLevel(sn)
		assert.Equal(t, expectedSeverityLevel, severityLevel)
	}
}

// Test onLogData callback for the test logs data
func TestExporterLogDataCallback(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	exporter := getLogsExporter(defaultConfig, mockTransportChannel)

	logs := getTestLogs()

	assert.NoError(t, exporter.onLogData(context.Background(), logs))

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 4)
}

func TestLogDataAttributesMapping(t *testing.T) {
	logPacker := getLogPacker()
	resource, scope, logRecord := getTestLogRecord(2)
	logRecord.Attributes().PutInt("attribute_1", 10)
	logRecord.Attributes().PutStr("attribute_2", "value_2")
	logRecord.Attributes().PutBool("attribute_3", true)
	logRecord.Attributes().PutDouble("attribute_4", 1.2)

	envelope := logPacker.LogRecordToEnvelope(logRecord, resource, scope)

	actualProperties := envelope.Data.(*contracts.Data).BaseData.(*contracts.MessageData).Properties
	assert.Contains(t, actualProperties["attribute_1"], "10")
	assert.Contains(t, actualProperties["attribute_2"], "value_2")
	assert.Contains(t, actualProperties["attribute_3"], "true")
	assert.Contains(t, actualProperties["attribute_4"], "1.2")
}

func TestLogRecordToEnvelopeResourceAttributes(t *testing.T) {
	resource, scope, logRecord := getTestLogRecord(1)
	logPacker := getLogPacker()

	envelope := logPacker.LogRecordToEnvelope(logRecord, resource, scope)

	require.NotEmpty(t, resource.Attributes())
	envelopeData := envelope.Data.(*contracts.Data).BaseData.(*contracts.MessageData)
	require.Subset(t, envelopeData.Properties, resource.Attributes().AsRaw())
}

func TestLogRecordToEnvelopeInstrumentationScope(t *testing.T) {
	const aiInstrumentationLibraryNameConvention = "instrumentationlibrary.name"
	const aiInstrumentationLibraryVersionConvention = "instrumentationlibrary.version"

	resource, scope, logRecord := getTestLogRecord(1)
	logPacker := getLogPacker()

	envelope := logPacker.LogRecordToEnvelope(logRecord, resource, scope)

	envelopeData := envelope.Data.(*contracts.Data).BaseData.(*contracts.MessageData)
	require.Equal(t, scope.Name(), envelopeData.Properties[aiInstrumentationLibraryNameConvention])
	require.Equal(t, scope.Version(), envelopeData.Properties[aiInstrumentationLibraryVersionConvention])
}

func TestLogRecordToEnvelopeCloudTags(t *testing.T) {
	const aiCloudRoleConvention = "ai.cloud.role"
	const aiCloudRoleInstanceConvention = "ai.cloud.roleInstance"

	resource, scope, logRecord := getTestLogRecord(1)
	logPacker := getLogPacker()

	envelope := logPacker.LogRecordToEnvelope(logRecord, resource, scope)

	resourceAttributes := resource.Attributes().AsRaw()
	expectedCloudRole := resourceAttributes[conventions.AttributeServiceNamespace].(string) + "." + resourceAttributes[conventions.AttributeServiceName].(string)
	require.Equal(t, expectedCloudRole, envelope.Tags[aiCloudRoleConvention])
	expectedCloudRoleInstance := resourceAttributes[conventions.AttributeServiceInstanceID]
	require.Equal(t, expectedCloudRoleInstance, envelope.Tags[aiCloudRoleInstanceConvention])
}

func getLogsExporter(config *Config, transportChannel transportChannel) *logExporter {
	return &logExporter{
		config,
		transportChannel,
		zap.NewNop(),
	}
}

func getLogPacker() *logPacker {
	return newLogPacker(zap.NewNop())
}

func getTestLogs() plog.Logs {
	logs := plog.NewLogs()

	// add the resource
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resource := resourceLogs.Resource()
	resource.Attributes().PutStr(conventions.AttributeServiceName, defaultServiceName)
	resource.Attributes().PutStr(conventions.AttributeServiceNamespace, defaultServiceNamespace)
	resource.Attributes().PutStr(conventions.AttributeServiceInstanceID, defaultServiceInstance)

	// add the scope
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scope := scopeLogs.Scope()
	scope.SetName(defaultScopeName)
	scope.SetVersion(defaultScopeVersion)

	// add the log records
	log := scopeLogs.LogRecords().AppendEmpty()
	log.SetTimestamp(testLogTimestamp)
	log.SetSeverityNumber(plog.SeverityNumberInfo)
	log.SetSeverityText("Information")
	log.SetFlags(1)
	log.SetSpanID(defaultSpanID)
	log.SetTraceID(defaultTraceID)
	log.Body().SetStr(testStringBody)

	log = scopeLogs.LogRecords().AppendEmpty()
	log.SetObservedTimestamp(testLogTimestamp)
	log.SetSeverityNumber(plog.SeverityNumberInfo)
	log.SetSeverityText("Information")
	log.SetFlags(1)
	log.SetSpanID(defaultSpanID)
	log.SetTraceID(defaultTraceID)
	log.Body().SetStr(testStringBody)

	log = scopeLogs.LogRecords().AppendEmpty()
	log.SetSeverityNumber(plog.SeverityNumberInfo)
	log.SetSeverityText("Information")
	log.SetFlags(1)
	log.SetSpanID(defaultSpanID)
	log.SetTraceID(defaultTraceID)
	log.Body().SetStr(testStringBody)

	log = scopeLogs.LogRecords().AppendEmpty()
	log.SetTimestamp(testLogTimestamp)
	log.SetSeverityNumber(plog.SeverityNumberInfo)
	log.SetSeverityText("Information")
	log.SetFlags(1)
	log.SetSpanID(defaultSpanID)
	log.SetTraceID(defaultTraceID)

	bodyMap := log.Body().SetEmptyMap()
	bodyMap.PutStr("key1", "value1")
	bodyMap.PutBool("key2", true)

	return logs
}

func getTestLogRecord(index int) (pcommon.Resource, pcommon.InstrumentationScope, plog.LogRecord) {
	logs := getTestLogs()
	resourceLogs := logs.ResourceLogs()
	resource := resourceLogs.At(0).Resource()
	scopeLogs := resourceLogs.At(0).ScopeLogs()
	scope := scopeLogs.At(0).Scope()
	logRecords := scopeLogs.At(0).LogRecords()
	logRecord := logRecords.At(index)

	return resource, scope, logRecord
}
