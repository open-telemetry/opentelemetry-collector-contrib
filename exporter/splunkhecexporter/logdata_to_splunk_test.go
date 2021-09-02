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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func Test_mapLogRecordToSplunkEvent(t *testing.T) {
	logger := zap.NewNop()
	ts := pdata.Timestamp(123)

	tests := []struct {
		name             string
		logRecordFn      func() pdata.LogRecord
		logResourceFn    func() pdata.Resource
		configDataFn     func() *Config
		wantSplunkEvents []*splunk.Event
	}{
		{
			name: "valid",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom", "com.splunk.source": "myapp", "host.name": "myhost"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with_name",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.SetName("my very own name")
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom", "com.splunk.source": "myapp", "host.name": "myhost", "otel.log.name": "my very own name"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "non-string attribute",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertDouble("foo", 123)
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"foo": float64(123), "com.splunk.source": "myapp", "host.name": "myhost"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with_config",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom"}, "unknown", "source", "sourcetype"),
			},
		},
		{
			name: "log_is_empty",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(nil, 0, map[string]interface{}{}, "unknown", "source", "sourcetype"),
			},
		},
		{
			name: "with span and trace id",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.SetSpanID(pdata.NewSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 50}))
				logRecord.SetTraceID(pdata.NewTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}))
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: func() []*splunk.Event {
				event := commonLogSplunkEvent(nil, 0, map[string]interface{}{}, "unknown", "source", "sourcetype")
				event.Fields["span_id"] = "0000000000000032"
				event.Fields["trace_id"] = "00000000000000000000000000000064"
				return []*splunk.Event{event}
			}(),
		},
		{
			name: "with double body",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetDoubleVal(42)
				logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(float64(42), ts, map[string]interface{}{"custom": "custom", "com.splunk.source": "myapp", "host.name": "myhost"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with int body",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetIntVal(42)
				logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(int64(42), ts, map[string]interface{}{"custom": "custom", "com.splunk.source": "myapp", "host.name": "myhost"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with bool body",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetBoolVal(true)
				logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(true, ts, map[string]interface{}{"custom": "custom", "com.splunk.source": "myapp", "host.name": "myhost"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with map body",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				attVal := pdata.NewAttributeValueMap()
				attMap := attVal.MapVal()
				attMap.InsertDouble("23", 45)
				attMap.InsertString("foo", "bar")
				attVal.CopyTo(logRecord.Body())
				logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(map[string]interface{}{"23": float64(45), "foo": "bar"}, ts,
					map[string]interface{}{"custom": "custom", "com.splunk.source": "myapp", "host.name": "myhost"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with nil body",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(nil, ts, map[string]interface{}{"custom": "custom", "com.splunk.source": "myapp", "host.name": "myhost"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with array body",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				attVal := pdata.NewAttributeValueArray()
				attArray := attVal.ArrayVal()
				attArray.AppendEmpty().SetStringVal("foo")
				attVal.CopyTo(logRecord.Body())
				logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent([]interface{}{"foo"}, ts, map[string]interface{}{"custom": "custom", "com.splunk.source": "myapp", "host.name": "myhost"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "log resource attribute",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("mylog")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: func() pdata.Resource {
				attr := map[string]pdata.AttributeValue{
					"resourceAttr1":               pdata.NewAttributeValueString("some_string"),
					splunk.DefaultSourceTypeLabel: pdata.NewAttributeValueString("myapp-type-from-resource-attr"),
					splunk.DefaultIndexLabel:      pdata.NewAttributeValueString("index-resource"),
					splunk.DefaultSourceLabel:     pdata.NewAttributeValueString("myapp-resource"),
					conventions.AttributeHostName: pdata.NewAttributeValueString("myhost-resource"),
				}
				resource := pdata.NewResource()
				resource.Attributes().InitFromMap(attr)
				return resource
			},
			configDataFn: func() *Config {
				return &Config{}
			},
			wantSplunkEvents: func() []*splunk.Event {
				event := commonLogSplunkEvent("mylog", ts, map[string]interface{}{
					"com.splunk.source": "myapp-resource", "host.name": "myhost-resource", "resourceAttr1": "some_string",
				}, "myhost-resource", "myapp-resource", "myapp-type-from-resource-attr")
				event.Index = "index-resource"
				return []*splunk.Event{
					event,
				}
			}(),
		},
		{
			name: "with severity",
			logRecordFn: func() pdata.LogRecord {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().InsertString(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetSeverityText("DEBUG")
				logRecord.SetSeverityNumber(pdata.SeverityNumberDEBUG)
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pdata.NewResource,
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom", "com.splunk.source": "myapp", "host.name": "myhost", "otel.log.severity.number": pdata.SeverityNumberDEBUG, "otel.log.severity.text": "DEBUG"},
					"myhost", "myapp", "myapp-type"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range tt.wantSplunkEvents {
				got := mapLogRecordToSplunkEvent(tt.logResourceFn(), tt.logRecordFn(), tt.configDataFn(), logger)
				assert.EqualValues(t, want, got)
			}
		})
	}
}

func commonLogSplunkEvent(
	event interface{},
	ts pdata.Timestamp,
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

func Test_emptyLogRecord(t *testing.T) {
	event := mapLogRecordToSplunkEvent(pdata.NewResource(), pdata.NewLogRecord(), &Config{}, zap.NewNop())
	assert.Nil(t, event.Time)
	assert.Equal(t, event.Host, "unknown")
	assert.Zero(t, event.Source)
	assert.Zero(t, event.SourceType)
	assert.Zero(t, event.Index)
	assert.Nil(t, event.Event)
	assert.Empty(t, event.Fields)
}

func Test_nanoTimestampToEpochMilliseconds(t *testing.T) {
	splunkTs := nanoTimestampToEpochMilliseconds(1001000000)
	assert.Equal(t, 1.001, *splunkTs)
	splunkTs = nanoTimestampToEpochMilliseconds(1001990000)
	assert.Equal(t, 1.002, *splunkTs)
	splunkTs = nanoTimestampToEpochMilliseconds(0)
	assert.True(t, nil == splunkTs)
}
