// Copyright 2020, OpenTelemetry Authors
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

package splunkhecexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

func Test_logDataToSplunk(t *testing.T) {
	logger := zap.NewNop()
	ts := pdata.TimestampUnixNano(123)

	tests := []struct {
		name               string
		logDataFn          func() pdata.Logs
		configDataFn       func() *Config
		wantSplunkEvents   []*splunk.Event
		wantNumDroppedLogs int
	}{
		{
			name: "valid",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.InitEmpty()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostHostname, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
			wantNumDroppedLogs: 0,
		},
		{
			name: "non-string attribute",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.InitEmpty()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostHostname, "myhost")
				logRecord.Attributes().InsertDouble("foo", 123)
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{}, "myhost", "myapp", "myapp-type"),
			},
			wantNumDroppedLogs: 0,
		},
		{
			name: "with_config",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.InitEmpty()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom"}, "unknown", "source", "sourcetype"),
			},
			wantNumDroppedLogs: 0,
		},
		{
			name: "log_is_nil",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.InitEmpty()
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents:   []*splunk.Event{},
			wantNumDroppedLogs: 1,
		},
		{
			name: "with double body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.InitEmpty()
				logRecord.Body().SetDoubleVal(42)
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostHostname, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(float64(42), ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
			wantNumDroppedLogs: 0,
		},
		{
			name: "with int body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.InitEmpty()
				logRecord.Body().SetIntVal(42)
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostHostname, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(int64(42), ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
			wantNumDroppedLogs: 0,
		},
		{
			name: "with bool body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.InitEmpty()
				logRecord.Body().SetBoolVal(true)
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostHostname, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(true, ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
			wantNumDroppedLogs: 0,
		},
		{
			name: "with map body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.InitEmpty()
				attMap := pdata.NewAttributeMap()
				attMap.InsertDouble("23", 45)
				attMap.InsertString("foo", "bar")
				logRecord.Body().SetMapVal(attMap)
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostHostname, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(map[string]interface{}{"23": float64(45), "foo": "bar"}, ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
			wantNumDroppedLogs: 0,
		},
		{
			name: "with nil body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.InitEmpty()
				logRecord.Body().InitEmpty()
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostHostname, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(nil, ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
			wantNumDroppedLogs: 0,
		},
		{
			name: "with array body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.InitEmpty()
				attArray := pdata.NewAnyValueArray()
				attArray.Append(pdata.NewAttributeValueString("foo"))
				logRecord.Body().SetArrayVal(attArray)
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostHostname, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent([]interface{}{"foo"}, ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
			wantNumDroppedLogs: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEvents, gotNumDroppedLogs := logDataToSplunk(logger, tt.logDataFn(), tt.configDataFn())
			assert.Equal(t, tt.wantNumDroppedLogs, gotNumDroppedLogs)
			require.Equal(t, len(tt.wantSplunkEvents), len(gotEvents))
			for i, want := range tt.wantSplunkEvents {
				assert.EqualValues(t, want, gotEvents[i])
			}
			assert.Equal(t, tt.wantSplunkEvents, gotEvents)
		})
	}
}

func makeLog(record pdata.LogRecord) pdata.Logs {
	logs := pdata.NewLogs()
	rl := pdata.NewResourceLogs()
	rl.InitEmpty()
	logs.ResourceLogs().Append(rl)
	ill := pdata.NewInstrumentationLibraryLogs()
	ill.InitEmpty()
	rl.InstrumentationLibraryLogs().Append(ill)
	ill.Logs().Append(record)
	return logs
}

func commonLogSplunkEvent(
	event interface{},
	ts pdata.TimestampUnixNano,
	fields map[string]interface{},
	host string,
	source string,
	sourcetype string,
) *splunk.Event {
	return &splunk.Event{
		Time:       nanoTimestampToEpochMilliseconds(ts),
		Host:       host,
		Event:      event,
		Source:     source,
		SourceType: sourcetype,
		Fields:     fields,
	}
}

func Test_nilLogs(t *testing.T) {
	events, dropped := logDataToSplunk(zap.NewNop(), pdata.NewLogs(), &Config{})
	assert.Equal(t, 0, dropped)
	assert.Equal(t, 0, len(events))
}

func Test_nilResourceLogs(t *testing.T) {
	logs := pdata.NewLogs()
	resourceLog := pdata.NewResourceLogs()
	logs.ResourceLogs().Append(resourceLog)
	events, dropped := logDataToSplunk(zap.NewNop(), logs, &Config{})
	assert.Equal(t, 0, dropped)
	assert.Equal(t, 0, len(events))
}

func Test_nilInstrumentationLogs(t *testing.T) {
	logs := pdata.NewLogs()
	resourceLog := pdata.NewResourceLogs()
	resourceLog.InitEmpty()
	logs.ResourceLogs().Append(resourceLog)
	ils := pdata.NewInstrumentationLibraryLogs()
	resourceLog.InstrumentationLibraryLogs().Append(ils)
	events, dropped := logDataToSplunk(zap.NewNop(), logs, &Config{})
	assert.Equal(t, 0, dropped)
	assert.Equal(t, 0, len(events))
}

func Test_nanoTimestampToEpochMilliseconds(t *testing.T) {
	splunkTs := nanoTimestampToEpochMilliseconds(1001000000)
	assert.Equal(t, 1.001, splunkTs)
	splunkTs = nanoTimestampToEpochMilliseconds(1001990000)
	assert.Equal(t, 1.002, splunkTs)
}
