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

	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"
)

const (
	defaultEnvelopeName = "Microsoft.ApplicationInsights.Message"
	defaultBaseType     = "MessageData"
	eventBaseType       = "EventData"
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
		name     string
		baseType string
		index    int
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
			assert.Equal(t, defaultBaseType, envelopeData.BaseType)

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

	assert.NoError(t, exporter.consumeLogs(context.Background(), logs))

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
	expectedCloudRole := resourceAttributes[string(conventions.ServiceNamespaceKey)].(string) + "." + resourceAttributes[string(conventions.ServiceNameKey)].(string)
	require.Equal(t, expectedCloudRole, envelope.Tags[aiCloudRoleConvention])
	expectedCloudRoleInstance := resourceAttributes[string(conventions.ServiceInstanceIDKey)]
	require.Equal(t, expectedCloudRoleInstance, envelope.Tags[aiCloudRoleInstanceConvention])
}

func getLogsExporter(config *Config, transportChannel appinsights.TelemetryChannel) *azureMonitorExporter {
	return &azureMonitorExporter{
		config,
		transportChannel,
		zap.NewNop(),
		newMetricPacker(zap.NewNop()),
	}
}

func getLogPacker() *logPacker {
	return newLogPacker(zap.NewNop(), defaultConfig)
}

func getTestLogs() plog.Logs {
	logs := plog.NewLogs()

	// add the resource
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resource := resourceLogs.Resource()
	resource.Attributes().PutStr(string(conventions.ServiceNameKey), defaultServiceName)
	resource.Attributes().PutStr(string(conventions.ServiceNamespaceKey), defaultServiceNamespace)
	resource.Attributes().PutStr(string(conventions.ServiceInstanceIDKey), defaultServiceInstance)

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

func TestHandleEventData(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{}
	packer := newLogPacker(logger, config)

	tests := []struct {
		name              string
		logRecord         func() plog.LogRecord
		expectedEventName string
		expectedProperty  map[string]string
	}{
		{
			name: "Event name from attributeMicrosoftCustomEventName",
			logRecord: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Attributes().PutStr(attributeMicrosoftCustomEventName, "CustomEvent")
				lr.Attributes().PutStr("test_attribute", "test_value")
				return lr
			},
			expectedEventName: "CustomEvent",
			expectedProperty: map[string]string{
				attributeMicrosoftCustomEventName: "CustomEvent",
				"test_attribute":                  "test_value",
			},
		},
		{
			name: "Event name from attributeApplicationInsightsEventMarkerAttribute",
			logRecord: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Attributes().PutStr(attributeApplicationInsightsEventMarkerAttribute, "MarkerEvent")
				lr.Attributes().PutStr("test_attribute", "test_value")
				return lr
			},
			expectedEventName: "MarkerEvent",
			expectedProperty: map[string]string{
				attributeApplicationInsightsEventMarkerAttribute: "MarkerEvent",
				"test_attribute": "test_value",
			},
		},
		{
			name: "No event name attributes",
			logRecord: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Attributes().PutStr("test_attribute", "test_value")
				return lr
			},
			expectedEventName: "",
			expectedProperty: map[string]string{
				"test_attribute": "test_value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := contracts.NewEnvelope()
			data := contracts.NewData()
			logRecord := tt.logRecord()

			packer.handleEventData(envelope, data, logRecord)

			eventData := data.BaseData.(*contracts.EventData)
			assert.Equal(t, tt.expectedEventName, eventData.Name)
			assert.Equal(t, tt.expectedProperty, eventData.Properties)
		})
	}
}

func TestSetAttributesAsProperties(t *testing.T) {
	properties := make(map[string]string)
	attributes := pcommon.NewMap()
	attributes.PutStr("string_key", "string_value")
	attributes.PutInt("int_key", 123)
	attributes.PutDouble("double_key", 4.56)
	attributes.PutBool("bool_key", true)

	setAttributesAsProperties(attributes, properties)

	assert.Equal(t, "string_value", properties["string_key"])
	assert.Equal(t, "123", properties["int_key"])
	assert.Equal(t, "4.56", properties["double_key"])
	assert.Equal(t, "true", properties["bool_key"])
}

func TestHandleExceptionDataWithDetails(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{}
	packer := newLogPacker(logger, config)

	tests := []struct {
		name             string
		severityNum      plog.SeverityNumber
		severityText     string
		exceptionType    string
		exceptionMessage string
		stackTrace       string
		resourceAttrs    map[string]any
	}{
		{
			name:             "Full exception details",
			severityNum:      plog.SeverityNumberError,
			severityText:     "RuntimeError",
			exceptionType:    "TypeError",
			exceptionMessage: "Cannot read property 'undefined'",
			stackTrace:       "at Object.method (/path/file.js:10)\nat Object.method2 (/path/file2.js:20)",
			resourceAttrs: map[string]any{
				string(conventions.ServiceNameKey): "testService",
				"custom.attr":                      "value",
			},
		},
		{
			name:             "Minimal exception details",
			severityNum:      plog.SeverityNumberFatal,
			severityText:     "FatalError",
			exceptionType:    "SystemError",
			exceptionMessage: "System crash",
			resourceAttrs:    map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := contracts.NewEnvelope()
			envelope.Tags = make(map[string]string)
			data := contracts.NewData()

			logRecord := plog.NewLogRecord()
			logRecord.SetSeverityNumber(tt.severityNum)
			logRecord.SetSeverityText(tt.severityText)

			attrs := logRecord.Attributes()
			attrs.PutStr(string(conventions.ExceptionTypeKey), tt.exceptionType)
			attrs.PutStr(string(conventions.ExceptionMessageKey), tt.exceptionMessage)
			if tt.stackTrace != "" {
				attrs.PutStr(string(conventions.ExceptionStacktraceKey), tt.stackTrace)
			}

			resource := pcommon.NewResource()
			for k, v := range tt.resourceAttrs {
				if str, ok := v.(string); ok {
					resource.Attributes().PutStr(k, str)
				}
			}

			scope := pcommon.NewInstrumentationScope()

			packer.handleExceptionData(envelope, data, logRecord, resource, scope)

			exceptionData := data.BaseData.(*contracts.ExceptionData)
			assert.Equal(t, tt.severityText, exceptionData.ProblemId)
			assert.NotEmpty(t, exceptionData.Properties)

			require.Len(t, exceptionData.Exceptions, 1)
			exception := exceptionData.Exceptions[0]
			assert.Equal(t, tt.exceptionType, exception.TypeName)
			assert.Equal(t, tt.exceptionMessage, exception.Message)

			if tt.stackTrace != "" {
				assert.Equal(t, tt.stackTrace, exception.Stack)
				assert.True(t, exception.HasFullStack)
			}

			// Resource attributes should be copied to properties
			for k, v := range tt.resourceAttrs {
				if str, ok := v.(string); ok {
					assert.Equal(t, str, exceptionData.Properties[k])
				}
			}
		})
	}
}
