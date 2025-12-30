// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func Test_mapLogRecordToSplunkEvent(t *testing.T) {
	ts := pcommon.Timestamp(123)

	tests := []struct {
		name             string
		logRecordFn      func() plog.LogRecord
		logResourceFn    func() pcommon.Resource
		toOtelAttrs      HecToOtelAttrs
		toHecAttrs       OtelToHecFields
		source           string
		sourceType       string
		index            string
		wantSplunkEvents []*Event
	}{
		{
			name: "valid",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylog")
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr("host.name", "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent("mylog", ts, map[string]any{"custom": "custom"},
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
				logRecord.Attributes().PutStr("host.name", "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent("mylog", ts, map[string]any{"custom": "custom"},
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
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent("mylog", ts, map[string]any{},
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
				logRecord.Attributes().PutStr("host.name", "myhost")
				logRecord.Attributes().PutDouble("foo", 123)
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent("mylog", ts, map[string]any{"foo": float64(123)}, "myhost", "myapp", "myapp-type"),
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
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent("mylog", ts, map[string]any{"custom": "custom"}, "unknown", "source", "sourcetype"),
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
			toOtelAttrs: HecToOtelAttrs{
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Host:       "myhost",
			},
			toHecAttrs: OtelToHecFields{
				SeverityNumber: "myseveritynum",
				SeverityText:   "myseverity",
			},
			source:     "",
			sourceType: "",
			index:      "",
			wantSplunkEvents: []*Event{
				func() *Event {
					event := commonLogSplunkEvent("mylog", ts, map[string]any{"custom": "custom", "myseverity": "DEBUG", "myseveritynum": plog.SeverityNumber(5)}, "myhost", "mysource", "mysourcetype")
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
			logResourceFn:    pcommon.NewResource,
			toOtelAttrs:      DefaultHecToOtelAttrs(),
			toHecAttrs:       DefaultOtelToHecFields(),
			source:           "source",
			sourceType:       "sourcetype",
			index:            "",
			wantSplunkEvents: []*Event{},
		},
		{
			name: "with span and trace id",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("foo")
				logRecord.SetSpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 50})
				logRecord.SetTraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100})
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: func() []*Event {
				event := commonLogSplunkEvent("foo", 0, map[string]any{}, "unknown", "source", "sourcetype")
				event.Fields["span_id"] = "0000000000000032"
				event.Fields["trace_id"] = "00000000000000000000000000000064"
				return []*Event{event}
			}(),
		},
		{
			name: "with double body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetDouble(42)
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr("host.name", "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent(float64(42), ts, map[string]any{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with int body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetInt(42)
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr("host.name", "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent(int64(42), ts, map[string]any{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with bool body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetBool(true)
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr("host.name", "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent(true, ts, map[string]any{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
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
				logRecord.Attributes().PutStr("host.name", "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent(map[string]any{"23": float64(45), "foo": "bar"}, ts,
					map[string]any{"custom": "custom"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with nil body",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr("host.name", "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn:    pcommon.NewResource,
			toOtelAttrs:      DefaultHecToOtelAttrs(),
			toHecAttrs:       DefaultOtelToHecFields(),
			source:           "source",
			sourceType:       "sourcetype",
			index:            "",
			wantSplunkEvents: []*Event{},
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
				logRecord.Attributes().PutStr("host.name", "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent([]any{"foo"}, ts, map[string]any{"custom": "custom"},
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
				resource.Attributes().PutStr("host.name", "myhost-resource")
				return resource
			},
			toOtelAttrs: DefaultHecToOtelAttrs(),
			toHecAttrs:  DefaultOtelToHecFields(),
			source:      "",
			sourceType:  "",
			index:       "",
			wantSplunkEvents: func() []*Event {
				event := commonLogSplunkEvent("mylog", ts, map[string]any{
					"resourceAttr1": "some_string",
				}, "myhost-resource", "myapp-resource", "myapp-type-from-resource-attr")
				event.Index = "index-resource"
				return []*Event{
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
				logRecord.Attributes().PutStr("host.name", "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetSeverityText("DEBUG")
				logRecord.SetSeverityNumber(plog.SeverityNumberDebug)
				logRecord.SetTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent("mylog", ts, map[string]any{"custom": "custom", "otel.log.severity.number": plog.SeverityNumberDebug, "otel.log.severity.text": "DEBUG"},
					"myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "valid timestamp",
			logRecordFn: func() plog.LogRecord {
				logRecord := plog.NewLogRecord()
				logRecord.Body().SetStr("mylog")
				logRecord.Attributes().PutStr(splunk.DefaultSourceLabel, "myapp")
				logRecord.Attributes().PutStr(splunk.DefaultSourceTypeLabel, "myapp-type")
				logRecord.Attributes().PutStr("host.name", "myhost")
				logRecord.Attributes().PutStr("custom", "custom")
				logRecord.SetObservedTimestamp(ts)
				return logRecord
			},
			logResourceFn: pcommon.NewResource,
			toOtelAttrs:   DefaultHecToOtelAttrs(),
			toHecAttrs:    DefaultOtelToHecFields(),
			source:        "source",
			sourceType:    "sourcetype",
			index:         "",
			wantSplunkEvents: []*Event{
				commonLogSplunkEvent("mylog", ts, map[string]any{"custom": "custom"},
					"myhost", "myapp", "myapp-type"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range tt.wantSplunkEvents {
				got := LogToSplunkEvent(tt.logResourceFn(), tt.logRecordFn(), tt.toOtelAttrs, tt.toHecAttrs, tt.source, tt.sourceType, tt.index)
				assert.Equal(t, want, got)
			}
		})
	}
}

func commonLogSplunkEvent(
	event any,
	ts pcommon.Timestamp,
	fields map[string]any,
	host string,
	source string,
	sourcetype string,
) *Event {
	return &Event{
		Time:       nanoTimestampToEpochMilliseconds(ts),
		Host:       host,
		Event:      event,
		Source:     source,
		SourceType: sourcetype,
		Fields:     fields,
	}
}

func Test_emptyLogRecord(t *testing.T) {
	event := LogToSplunkEvent(pcommon.NewResource(), plog.NewLogRecord(), DefaultHecToOtelAttrs(), DefaultOtelToHecFields(), "", "", "")
	assert.Nil(t, event)
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
