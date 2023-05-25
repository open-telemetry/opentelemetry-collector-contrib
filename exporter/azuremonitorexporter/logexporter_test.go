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
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	defaultEnvelopeName = "Microsoft.ApplicationInsights.Message"
	defaultdBaseType    = "MessageData"
)

var (
	testLogs         = []byte(`{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"dotnet"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"timeUnixNano":"1643240673066096200","severityText":"Information","body":{"stringValue":"Message Body"},"flags":1,"traceId":"7b20d1349ef9b6d6f9d4d1d4a3ac2e82","spanId":"0c2ad924e1771630"},{"timeUnixNano":"0","observedTimeUnixNano":"1643240673066096200","severityText":"Information","body":{"stringValue":"Message Body"},"flags":1,"traceId":"7b20d1349ef9b6d6f9d4d1d4a3ac2e82","spanId":"0c2ad924e1771630"},{"timeUnixNano":"0","observedTimeUnixNano":"0","severityText":"Information","body":{"stringValue":"Message Body"},"flags":1,"traceId":"7b20d1349ef9b6d6f9d4d1d4a3ac2e82","spanId":"0c2ad924e1771630"}]}]}]}`)
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
		name      string
		logRecord plog.LogRecord
	}{
		{
			name:      "timestamp is correct",
			logRecord: getTestLogRecord(t, 0),
		},
		{
			name:      "timestamp is empty",
			logRecord: getTestLogRecord(t, 1),
		},
		{
			name:      "timestamp is empty and observed timestamp is empty",
			logRecord: getTestLogRecord(t, 2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logRecord := tt.logRecord
			logPacker := getLogPacker()
			envelope := logPacker.LogRecordToEnvelope(logRecord, getResource(), getScope())

			require.NotNil(t, envelope)
			assert.Equal(t, defaultEnvelopeName, envelope.Name)
			assert.Equal(t, toTime(timestampFromLogRecord(logRecord)).Format(time.RFC3339Nano), envelope.Time)
			require.NotNil(t, envelope.Data)
			envelopeData := envelope.Data.(*contracts.Data)
			assert.Equal(t, defaultdBaseType, envelopeData.BaseType)

			require.NotNil(t, envelopeData.BaseData)

			messageData := envelopeData.BaseData.(*contracts.MessageData)
			assert.Equal(t, messageData.Message, logRecord.Body().Str())
			assert.Equal(t, messageData.SeverityLevel, contracts.Information)

			hexTraceID := traceutil.TraceIDToHexOrEmptyString(logRecord.TraceID())
			assert.Equal(t, envelope.Tags[contracts.OperationId], hexTraceID)

			hexSpanID := traceutil.SpanIDToHexOrEmptyString(logRecord.SpanID())
			assert.Equal(t, envelope.Tags[contracts.OperationParentId], hexSpanID)
		})
	}
}

// Test conversion from logRecord.SeverityNumber() to contracts.SeverityLevel()
func TestToAiSeverityLevel(t *testing.T) {
	logPacker := getLogPacker()
	for sn, expectedSeverityLevel := range severityLevelMap {
		severityLevel := logPacker.toAiSeverityLevel(sn)
		assert.Equal(t, severityLevel, expectedSeverityLevel)
	}
}

// Test onLogData callback for the test logs data
func TestExporterLogDataCallback(t *testing.T) {
	mockTransportChannel := getMockTransportChannel()
	exporter := getLogsExporter(defaultConfig, mockTransportChannel)

	logs := getTestLogs(t)

	assert.NoError(t, exporter.onLogData(context.Background(), logs))

	mockTransportChannel.AssertNumberOfCalls(t, "Send", 3)
}

func TestLogDataAttributesMapping(t *testing.T) {
	logPacker := getLogPacker()
	logRecord := getTestLogRecord(t, 2)
	logRecord.Attributes().PutInt("attribute_1", 10)
	logRecord.Attributes().PutStr("attribute_2", "value_2")
	logRecord.Attributes().PutBool("attribute_3", true)
	logRecord.Attributes().PutDouble("attribute_4", 1.2)

	envelope := logPacker.LogRecordToEnvelope(logRecord, getResource(), getScope())

	actualProperties := envelope.Data.(*contracts.Data).BaseData.(*contracts.MessageData).Properties
	assert.Contains(t, actualProperties["attribute_1"], "10")
	assert.Contains(t, actualProperties["attribute_2"], "value_2")
	assert.Contains(t, actualProperties["attribute_3"], "true")
	assert.Contains(t, actualProperties["attribute_4"], "1.2")
}

func TestLogRecordToEnvelopeResourceAttributes(t *testing.T) {
	logRecord := getTestLogRecord(t, 1)
	logPacker := getLogPacker()
	resource := getResource()

	envelope := logPacker.LogRecordToEnvelope(logRecord, resource, getScope())

	require.NotEmpty(t, resource.Attributes())
	envelopeData := envelope.Data.(*contracts.Data).BaseData.(*contracts.MessageData)
	require.Subset(t, envelopeData.Properties, resource.Attributes().AsRaw())
}

func TestLogRecordToEnvelopeInstrumentationScope(t *testing.T) {
	const aiInstrumentationLibraryNameConvention = "instrumentationlibrary.name"
	const aiInstrumentationLibraryVersionConvention = "instrumentationlibrary.version"

	logRecord := getTestLogRecord(t, 1)
	logPacker := getLogPacker()
	scope := getScope()

	envelope := logPacker.LogRecordToEnvelope(logRecord, getResource(), scope)

	envelopeData := envelope.Data.(*contracts.Data).BaseData.(*contracts.MessageData)
	require.Equal(t, scope.Name(), envelopeData.Properties[aiInstrumentationLibraryNameConvention])
	require.Equal(t, scope.Version(), envelopeData.Properties[aiInstrumentationLibraryVersionConvention])
}

func TestLogRecordToEnvelopeCloudTags(t *testing.T) {
	const aiCloudRoleConvention = "ai.cloud.role"
	const aiCloudRoleInstanceConvention = "ai.cloud.roleInstance"

	logRecord := getTestLogRecord(t, 1)
	logPacker := getLogPacker()
	resource := getResource()

	envelope := logPacker.LogRecordToEnvelope(logRecord, resource, getScope())

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

func getTestLogs(tb testing.TB) plog.Logs {
	logsMarshaler := &plog.JSONUnmarshaler{}
	logs, err := logsMarshaler.UnmarshalLogs(testLogs)
	assert.NoError(tb, err, "Can't unmarshal testing logs data -> %s", err)
	return logs
}

func getTestLogRecord(tb testing.TB, index int) plog.LogRecord {
	var logRecord plog.LogRecord
	logs := getTestLogs(tb)
	resourceLogs := logs.ResourceLogs()
	scopeLogs := resourceLogs.At(0).ScopeLogs()
	logRecords := scopeLogs.At(0).LogRecords()
	logRecord = logRecords.At(index)

	return logRecord
}
