// Copyright OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"go.uber.org/zap"
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
			envelope := logPacker.LogRecordToEnvelope(logRecord)

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

			hexTraceID := logRecord.TraceID().HexString()
			assert.Equal(t, envelope.Tags[contracts.OperationId], hexTraceID)

			hexSpanID := logRecord.SpanID().HexString()
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
