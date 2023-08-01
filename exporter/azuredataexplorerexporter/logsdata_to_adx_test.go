// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func Test_mapToAdxLog(t *testing.T) {
	logger := zap.NewNop()
	epoch, _ := time.Parse("2006-01-02T15:04:05.999999999Z07:00", "1970-01-01T00:00:00.000000000Z")
	defaultTime := pcommon.NewTimestampFromTime(epoch).AsTime().Format(time.RFC3339Nano)
	tmap := make(map[string]interface{})
	tmap["key"] = "value"
	tmap[hostkey] = testhost

	spanID := [8]byte{0, 0, 0, 0, 0, 0, 0, 50}
	traceID := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}

	tests := []struct {
		name            string                // name of the test
		logRecordFn     func() plog.LogRecord // function that generates the logs
		logResourceFn   func() pcommon.Resource
		logScopeFn      func() pcommon.InstrumentationScope
		expectedAdxLogs []*AdxLog
	}{
		{
			name: "valid",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylogsample")
				logRecord.Attributes().PutStr("test", "value")
				logRecord.SetTimestamp(ts)
				logRecord.SetObservedTimestamp(ts)
				logRecord.SetSpanID(pcommon.SpanID(spanID))
				logRecord.SetTraceID(pcommon.TraceID(traceID))
				logRecord.SetSeverityNumber(plog.SeverityNumberDebug)
				logRecord.SetSeverityText("DEBUG")
				return logRecord
			},
			logResourceFn: newDummyResource,
			logScopeFn:    newScopeWithData,
			expectedAdxLogs: []*AdxLog{
				{
					Timestamp:          tstr,
					ObservedTimestamp:  tstr,
					SpanID:             "0000000000000032",
					TraceID:            "00000000000000000000000000000064",
					Body:               "mylogsample",
					SeverityText:       "DEBUG",
					SeverityNumber:     int32(plog.SeverityNumberDebug),
					ResourceAttributes: tmap,

					LogsAttributes: newMapFromAttr(`{"test":"value", "scope.name":"testscope", "scope.version":"1.0"}`),
				},
			},
		},
		{
			name: "without severity",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylogsample")
				logRecord.Attributes().PutStr("test", "value")
				logRecord.SetTimestamp(ts)
				logRecord.SetObservedTimestamp(ts)
				logRecord.SetSpanID(pcommon.SpanID(spanID))
				logRecord.SetTraceID(pcommon.TraceID(traceID))
				return logRecord
			},
			logResourceFn: newDummyResource,
			logScopeFn:    newScopeWithData,
			expectedAdxLogs: []*AdxLog{
				{
					Timestamp:          tstr,
					ObservedTimestamp:  tstr,
					SpanID:             "0000000000000032",
					TraceID:            "00000000000000000000000000000064",
					Body:               "mylogsample",
					ResourceAttributes: tmap,

					LogsAttributes: newMapFromAttr(`{"test":"value", "scope.name":"testscope", "scope.version":"1.0"}`),
				},
			},
		},
		{
			name: "with nil body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Attributes().PutStr("test", "value")
				logRecord.SetTimestamp(ts)
				logRecord.SetObservedTimestamp(ts)
				logRecord.SetSpanID(pcommon.SpanID(spanID))
				logRecord.SetTraceID(pcommon.TraceID(traceID))
				logRecord.SetSeverityNumber(plog.SeverityNumberDebug)
				logRecord.SetSeverityText("DEBUG")
				return logRecord
			},
			logResourceFn: newDummyResource,
			logScopeFn:    newScopeWithData,
			expectedAdxLogs: []*AdxLog{
				{
					Timestamp:          tstr,
					ObservedTimestamp:  tstr,
					SpanID:             "0000000000000032",
					TraceID:            "00000000000000000000000000000064",
					SeverityText:       "DEBUG",
					SeverityNumber:     int32(plog.SeverityNumberDebug),
					ResourceAttributes: tmap,

					LogsAttributes: newMapFromAttr(`{"test":"value", "scope.name":"testscope", "scope.version":"1.0"}`),
				},
			},
		},
		{
			name: "with json body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				attVal := pcommon.NewValueMap()
				attMap := attVal.Map()
				attMap.PutDouble("23", 45)
				attMap.PutStr("foo", "bar")
				attVal.CopyTo(logRecord.Body())
				logRecord.Attributes().PutStr("test", "value")
				logRecord.SetTimestamp(ts)
				logRecord.SetObservedTimestamp(ts)
				logRecord.SetSpanID(pcommon.SpanID(spanID))
				logRecord.SetTraceID(pcommon.TraceID(traceID))
				logRecord.SetSeverityNumber(plog.SeverityNumberDebug)
				logRecord.SetSeverityText("DEBUG")
				return logRecord
			},
			logResourceFn: newDummyResource,
			logScopeFn:    newScopeWithData,
			expectedAdxLogs: []*AdxLog{
				{
					Timestamp:          tstr,
					ObservedTimestamp:  tstr,
					SpanID:             "0000000000000032",
					TraceID:            "00000000000000000000000000000064",
					Body:               `{"23":45,"foo":"bar"}`,
					SeverityText:       "DEBUG",
					SeverityNumber:     int32(plog.SeverityNumberDebug),
					ResourceAttributes: tmap,

					LogsAttributes: newMapFromAttr(`{"test":"value", "scope.name":"testscope", "scope.version":"1.0"}`),
				},
			},
		},
		{
			name: "Empty log data",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			logScopeFn:    pcommon.NewInstrumentationScope,
			expectedAdxLogs: []*AdxLog{
				{
					Timestamp:          defaultTime,
					ObservedTimestamp:  defaultTime,
					ResourceAttributes: map[string]interface{}{},
					LogsAttributes:     map[string]interface{}{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range tt.expectedAdxLogs {
				got := mapToAdxLog(tt.logResourceFn(), tt.logScopeFn(), tt.logRecordFn(), logger)
				encoder := json.NewEncoder(io.Discard)
				assert.EqualValues(t, want, got)
				err := encoder.Encode(got)
				assert.NoError(t, err)
			}
		})
	}
}

func newScopeWithData() pcommon.InstrumentationScope {
	scp := pcommon.NewInstrumentationScope()
	scp.SetName("testscope")
	scp.SetVersion("1.0")
	return scp
}
