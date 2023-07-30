// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func Test_mapLogRecordToSplunkEvent(t *testing.T) {
	ts := pcommon.Timestamp(123)

	tests := []struct {
		name             string
		logRecordFn      func() plog.LogRecord
		logResourceFn    func() pcommon.Resource
		configDataFn     func() *Config
		wantSplunkEvents []*splunk.Event
	}{
		{
			name: "valid",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylog")
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with_name",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylog")
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with_hec_token",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylog")
				logRecord.Attributes().PutStr(splunk.HecTokenLabel, "mytoken")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{},
					"unknown", "source", "sourcetype"),
			},
		},
		{
			name: "non-string attribute",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylog")
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().PutDouble("foo", 123)
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"foo": float64(123)}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with_config",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylog")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom"}, "unknown", "source", "sourcetype"),
			},
		},
		{
			name: "with_custom_mapping",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylog")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.Attributes().PutStr("mysource", "mysource")
				logRecord.Attributes().PutStr("mysourcetype", "mysourcetype")
				logRecord.Attributes().PutStr("myindex", "myindex")
				logRecord.Attributes().PutStr("myhost", "myhost")
				logRecord.SetSeverityText("DEBUG")
				logRecord.SetSeverityNumber(plog.SeverityNumberDebug)
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				return &Config{
					HecToOtelAttrs: splunk.HecToOtelAttrs{
						Source:     "mysource",
						SourceType: "mysourcetype",
						Index:      "myindex",
						Host:       "myhost",
					},
					HecFields: OtelToHecFields{
						SeverityNumber: "myseveritynum",
						SeverityText:   "myseverity",
					},
				}
			},
			wantSplunkEvents: []*splunk.Event{
				func() *splunk.Event {
					event := commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom", "myseverity": "DEBUG", "myseveritynum": plog.SeverityNumber(5)}, "myhost", "mysource", "mysourcetype")
					event.Index = "myindex"
					return event
				}(),
			},
		},
		{
			name: "log_is_empty",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(nil, 0, map[string]interface{}{}, "unknown", "source", "sourcetype"),
			},
		},
		{
			name: "with span and trace id",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.SetSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 50})
				logRecord.SetTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100})
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
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
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetDouble(42)
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(float64(42), ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with int body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetInt(42)
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(int64(42), ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with bool body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetBool(true)
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(true, ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with map body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				attVal := pcommon.NewValueMap()
				attMap := attVal.Map()
				attMap.PutDouble("23", 45)
				attMap.PutStr("foo", "bar")
				attVal.CopyTo(logRecord.Body())
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(map[string]interface{}{"23": float64(45), "foo": "bar"}, ts,
					map[string]interface{}{"custom": "custom"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with nil body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(nil, ts, map[string]interface{}{"custom": "custom"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with array body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				attVal := pcommon.NewValueSlice()
				attArray := attVal.Slice()
				attArray.AppendEmpty().SetStr("foo")
				attVal.CopyTo(logRecord.Body())
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent([]interface{}{"foo"}, ts, map[string]interface{}{"custom": "custom"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "log resource attribute",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylog")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: func() pcommon.Resource {
				resource := pcommon.NewResource()
				resource.Attributes().PutStr("resourceAttr1", "some_string")
				resource.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type-from-resource-attr")
				resource.Attributes().PutStr(splunk.DefaultIndexLabel, "index-resource")
				resource.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp-resource")
				resource.Attributes().PutStr(conventions.AttributeHostName, "myhost-resource")
				return resource
			},
			configDataFn: func() *Config {
				return createDefaultConfig().(*Config)
			},
			wantSplunkEvents: func() []*splunk.Event {
				event := commonLogSplunkEvent("mylog", ts, map[string]interface{}{
					"resourceAttr1": "some_string",
				}, "myhost-resource", "myapp-resource", "myapp-type-from-resource-attr")
				event.Index = "index-resource"
				return []*splunk.Event{
					event,
				}
			}(),
		},
		{
			name: "with severity",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylog")
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetSeverityText("DEBUG")
				logRecord.SetSeverityNumber(plog.SeverityNumberDebug)
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			configDataFn: func() *Config {
				config := createDefaultConfig().(*Config)
				config.Source = "source"
				config.SourceType = "sourcetype"
				return config
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom", "otel.log.severity.number": plog.SeverityNumberDebug, "otel.log.severity.text": "DEBUG"},
					"myhost", "myapp", "myapp-type"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range tt.wantSplunkEvents {
				config := tt.configDataFn()
				got := mapLogRecordToSplunkEvent(tt.logResourceFn(), tt.logRecordFn(), config)
				assert.EqualValues(t, want, got)
			}
		})
	}
}

func commonLogSplunkEvent(
	event interface{},
	ts pcommon.Timestamp,
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
	event := mapLogRecordToSplunkEvent(pcommon.NewResource(), plog.NewLogRecord(), &Config{})
	assert.Zero(t, event.Time)
	assert.Equal(t, event.Host, "unknown")
	assert.Zero(t, event.Source)
	assert.Zero(t, event.SourceType)
	assert.Zero(t, event.Index)
	assert.Nil(t, event.Event)
	assert.Empty(t, event.Fields)
}

func Test_nanoTimestampToEpochMilliseconds(t *testing.T) {
	splunkTs := nanoTimestampToEpochMilliseconds(1001000000)
	assert.Equal(t, 1.001, splunkTs)
	splunkTs = nanoTimestampToEpochMilliseconds(1001990000)
	assert.Equal(t, 1.002, splunkTs)
	splunkTs = nanoTimestampToEpochMilliseconds(0)
	assert.Zero(t, splunkTs)
}

func Test_mergeValue(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		val      any
		expected map[string]any
	}{
		{
			name:     "int",
			key:      "intKey",
			val:      0,
			expected: map[string]any{"intKey": 0},
		},
		{
			name:     "flat_array",
			key:      "arrayKey",
			val:      []any{0, 1, 2, 3},
			expected: map[string]any{"arrayKey": []any{0, 1, 2, 3}},
		},
		{
			name:     "nested_array",
			key:      "arrayKey",
			val:      []any{0, 1, []any{2, 3}},
			expected: map[string]any{"arrayKey": "[0,1,[2,3]]"},
		},
		{
			name:     "array_of_map",
			key:      "arrayKey",
			val:      []any{0, 1, map[string]any{"3": 3}},
			expected: map[string]any{"arrayKey": "[0,1,{\"3\":3}]"},
		},
		{
			name:     "flat_map",
			key:      "mapKey",
			val:      map[string]any{"1": 1, "2": 2},
			expected: map[string]any{"mapKey.1": 1, "mapKey.2": 2},
		},
		{
			name:     "nested_map",
			key:      "mapKey",
			val:      map[string]any{"1": 1, "2": 2, "nested": map[string]any{"3": 3, "4": 4}},
			expected: map[string]any{"mapKey.1": 1, "mapKey.2": 2, "mapKey.nested.3": 3, "mapKey.nested.4": 4},
		},
		{
			name:     "flat_array_in_nested_map",
			key:      "mapKey",
			val:      map[string]any{"1": 1, "2": 2, "nested": map[string]any{"3": 3, "flat_array": []any{4}}},
			expected: map[string]any{"mapKey.1": 1, "mapKey.2": 2, "mapKey.nested.3": 3, "mapKey.nested.flat_array": []any{4}},
		},
		{
			name:     "nested_array_in_nested_map",
			key:      "mapKey",
			val:      map[string]any{"1": 1, "2": 2, "nested": map[string]any{"3": 3, "nested_array": []any{4, []any{5}}}},
			expected: map[string]any{"mapKey.1": 1, "mapKey.2": 2, "mapKey.nested.3": 3, "mapKey.nested.nested_array": "[4,[5]]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := make(map[string]any)
			mergeValue(fields, tt.key, tt.val)
			assert.Equal(t, tt.expected, fields)
		})
	}

}
