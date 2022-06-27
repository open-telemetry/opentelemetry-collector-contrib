package azuredataexplorerexporter

import (
	"encoding/json"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func Test_mapToAdxLog(t *testing.T) {
	logger := zap.NewNop()
	epoch, _ := time.Parse("2006-01-02T15:04:05Z07:00", "1970-01-01T00:00:00Z")
	defaultTime := pcommon.NewTimestampFromTime(epoch).AsTime().Format(time.RFC3339)
	tmap := make(map[string]interface{})
	tmap["key"] = "value"
	tmap[hostkey] = testhost

	scpMap := map[string]string{
		"name":    "testscope",
		"version": "1.0",
	}

	spanId := [8]byte{0, 0, 0, 0, 0, 0, 0, 50}
	traceId := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}

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
				logRecord.Body().SetStringVal("mylogsample")
				logRecord.Attributes().InsertString("test", "value")
				logRecord.SetTimestamp(ts)
				logRecord.SetObservedTimestamp(ts)
				logRecord.SetSpanID(pcommon.NewSpanID(spanId))
				logRecord.SetTraceID(pcommon.NewTraceID(traceId))
				logRecord.SetSeverityNumber(plog.SeverityNumberDEBUG)
				logRecord.SetSeverityText("DEBUG")
				return logRecord
			},
			logResourceFn: func() pcommon.Resource {
				return newDummyResource()
			},
			logScopeFn: func() pcommon.InstrumentationScope {
				return newScopeWithData()
			},
			expectedAdxLogs: []*AdxLog{
				{
					Timestamp:            tstr,
					ObservedTimestamp:    tstr,
					SpanId:               "0000000000000032",
					TraceId:              "00000000000000000000000000000064",
					Body:                 "mylogsample",
					SeverityText:         "DEBUG",
					SeverityNumber:       int32(plog.SeverityNumberDEBUG),
					ResourceAttributes:   tmap,
					InstrumentationScope: scpMap,
					LogsAttributes:       newMapFromAttr(`{"test":"value"}`),
				},
			},
		},
		{
			name: "without severity",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStringVal("mylogsample")
				logRecord.Attributes().InsertString("test", "value")
				logRecord.SetTimestamp(ts)
				logRecord.SetObservedTimestamp(ts)
				logRecord.SetSpanID(pcommon.NewSpanID(spanId))
				logRecord.SetTraceID(pcommon.NewTraceID(traceId))
				return logRecord
			},
			logResourceFn: func() pcommon.Resource {
				return newDummyResource()
			},
			logScopeFn: func() pcommon.InstrumentationScope {
				return newScopeWithData()
			},
			expectedAdxLogs: []*AdxLog{
				{
					Timestamp:            tstr,
					ObservedTimestamp:    tstr,
					SpanId:               "0000000000000032",
					TraceId:              "00000000000000000000000000000064",
					Body:                 "mylogsample",
					ResourceAttributes:   tmap,
					InstrumentationScope: scpMap,
					LogsAttributes:       newMapFromAttr(`{"test":"value"}`),
				},
			},
		},
		{
			name: "with nil body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Attributes().InsertString("test", "value")
				logRecord.SetTimestamp(ts)
				logRecord.SetObservedTimestamp(ts)
				logRecord.SetSpanID(pcommon.NewSpanID(spanId))
				logRecord.SetTraceID(pcommon.NewTraceID(traceId))
				logRecord.SetSeverityNumber(plog.SeverityNumberDEBUG)
				logRecord.SetSeverityText("DEBUG")
				return logRecord
			},
			logResourceFn: func() pcommon.Resource {
				return newDummyResource()
			},
			logScopeFn: func() pcommon.InstrumentationScope {
				return newScopeWithData()
			},
			expectedAdxLogs: []*AdxLog{
				{
					Timestamp:            tstr,
					ObservedTimestamp:    tstr,
					SpanId:               "0000000000000032",
					TraceId:              "00000000000000000000000000000064",
					SeverityText:         "DEBUG",
					SeverityNumber:       int32(plog.SeverityNumberDEBUG),
					ResourceAttributes:   tmap,
					InstrumentationScope: scpMap,
					LogsAttributes:       newMapFromAttr(`{"test":"value"}`),
				},
			},
		},
		{
			name: "with json body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				attVal := pcommon.NewValueMap()
				attMap := attVal.MapVal()
				attMap.InsertDouble("23", 45)
				attMap.InsertString("foo", "bar")
				attVal.CopyTo(logRecord.Body())
				logRecord.Attributes().InsertString("test", "value")
				logRecord.SetTimestamp(ts)
				logRecord.SetObservedTimestamp(ts)
				logRecord.SetSpanID(pcommon.NewSpanID(spanId))
				logRecord.SetTraceID(pcommon.NewTraceID(traceId))
				logRecord.SetSeverityNumber(plog.SeverityNumberDEBUG)
				logRecord.SetSeverityText("DEBUG")
				return logRecord
			},
			logResourceFn: func() pcommon.Resource {
				return newDummyResource()
			},
			logScopeFn: func() pcommon.InstrumentationScope {
				return newScopeWithData()
			},
			expectedAdxLogs: []*AdxLog{
				{
					Timestamp:            tstr,
					ObservedTimestamp:    tstr,
					SpanId:               "0000000000000032",
					TraceId:              "00000000000000000000000000000064",
					Body:                 `{"23":45,"foo":"bar"}`,
					SeverityText:         "DEBUG",
					SeverityNumber:       int32(plog.SeverityNumberDEBUG),
					ResourceAttributes:   tmap,
					InstrumentationScope: scpMap,
					LogsAttributes:       newMapFromAttr(`{"test":"value"}`),
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
					Timestamp:            defaultTime,
					ObservedTimestamp:    defaultTime,
					ResourceAttributes:   map[string]interface{}{},
					InstrumentationScope: map[string]string{},
					LogsAttributes:       map[string]interface{}{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range tt.expectedAdxLogs {
				got := mapToAdxLog(tt.logResourceFn(), tt.logScopeFn(), tt.logRecordFn(), logger)
				encoder := json.NewEncoder(ioutil.Discard)
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
