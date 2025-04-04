// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func TestSerializeLog(t *testing.T) {
	tests := []struct {
		name          string
		logCustomizer func(resource pcommon.Resource, scope pcommon.InstrumentationScope, record plog.LogRecord)
		wantErr       bool
		expected      any
	}{
		{name: "test attributes", logCustomizer: func(resource pcommon.Resource, scope pcommon.InstrumentationScope, record plog.LogRecord) {
			record.SetSeverityText("debug")
			record.Attributes().PutEmpty("empty")
			record.Attributes().PutStr("data_stream.type", "logs")
			record.Attributes().PutStr("string", "foo")
			record.Attributes().PutBool("bool", true)
			record.Attributes().PutDouble("double", 42.0)
			record.Attributes().PutInt("int", 42)
			record.Attributes().PutEmptyBytes("bytes").Append(42)
			record.Attributes().PutStr(elasticsearch.DocumentIDAttributeName, "my_id")
			_ = record.Attributes().PutEmptySlice("slice").FromRaw([]any{42, "foo"})
			record.Attributes().PutEmptySlice("map_slice").AppendEmpty().SetEmptyMap().PutStr("foo.bar", "baz")
			mapAttr := record.Attributes().PutEmptyMap("map")
			mapAttr.PutStr("foo.bar", "baz")
			mapAttr.PutEmptySlice("inner.slice").AppendEmpty().SetStr("foo")

			resource.Attributes().PutEmptyMap("resource_map").PutStr("foo", "bar")
			scope.Attributes().PutEmptyMap("scope_map").PutStr("foo", "bar")
		}, wantErr: false, expected: map[string]any{
			"@timestamp":         "0.0",
			"observed_timestamp": "0.0",
			"severity_text":      "debug",
			"resource": map[string]any{
				"attributes": map[string]any{
					"resource_map": map[string]any{
						"foo": "bar",
					},
				},
			},
			"scope": map[string]any{
				"attributes": map[string]any{
					"scope_map": map[string]any{
						"foo": "bar",
					},
				},
			},
			"attributes": map[string]any{
				"empty":  nil,
				"string": "foo",
				"bool":   true,
				"double": json.Number("42.0"),
				"int":    json.Number("42"),
				"bytes":  "2a",
				"slice":  []any{json.Number("42"), "foo"},
				"map_slice": []any{map[string]any{
					"foo.bar": "baz",
				}},
				"map": map[string]any{
					"foo.bar":     "baz",
					"inner.slice": []any{"foo"},
				},
			},
		}},
		{
			name: "text body",
			logCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, record plog.LogRecord) {
				record.Body().SetStr("foo")
			},
			wantErr: false,
			expected: map[string]any{
				"@timestamp":         "0.0",
				"observed_timestamp": "0.0",
				"resource":           map[string]any{},
				"scope":              map[string]any{},
				"body": map[string]any{
					"text": "foo",
				},
			},
		},
		{
			name: "map body",
			logCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, record plog.LogRecord) {
				record.Body().SetEmptyMap().PutStr("foo.bar", "baz")
			},
			wantErr: false,
			expected: map[string]any{
				"@timestamp":         "0.0",
				"observed_timestamp": "0.0",
				"resource":           map[string]any{},
				"scope":              map[string]any{},
				"body": map[string]any{
					"structured": map[string]any{
						"foo.bar": "baz",
					},
				},
			},
		},
		{
			name: "geo attributes",
			logCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, record plog.LogRecord) {
				record.Attributes().PutDouble("geo.location.lon", 1.1)
				record.Attributes().PutDouble("geo.location.lat", 2.2)
				record.Attributes().PutDouble("foo.bar.geo.location.lon", 3.3)
				record.Attributes().PutDouble("foo.bar.geo.location.lat", 4.4)
				record.Attributes().PutDouble("a.geo.location.lon", 5.5)
				record.Attributes().PutDouble("b.geo.location.lat", 6.6)
				record.Attributes().PutDouble("unrelatedgeo.location.lon", 7.7)
				record.Attributes().PutDouble("unrelatedgeo.location.lat", 8.8)
				record.Attributes().PutDouble("d", 9.9)
				record.Attributes().PutStr("e.geo.location.lon", "foo")
				record.Attributes().PutStr("e.geo.location.lat", "bar")
			},
			wantErr: false,
			expected: map[string]any{
				"@timestamp":         "0.0",
				"observed_timestamp": "0.0",
				"resource":           map[string]any{},
				"scope":              map[string]any{},
				"attributes": map[string]any{
					"geo.location":              []any{json.Number("1.1"), json.Number("2.2")},
					"foo.bar.geo.location":      []any{json.Number("3.3"), json.Number("4.4")},
					"a.geo.location.lon":        json.Number("5.5"),
					"b.geo.location.lat":        json.Number("6.6"),
					"unrelatedgeo.location.lon": json.Number("7.7"),
					"unrelatedgeo.location.lat": json.Number("8.8"),
					"d":                         json.Number("9.9"),
					"e.geo.location.lon":        "foo",
					"e.geo.location.lat":        "bar",
				},
			},
		},
		{
			name: "event_name takes precedent over attributes.event.name",
			logCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, record plog.LogRecord) {
				record.Attributes().PutStr("event.name", "foo")
				record.SetEventName("bar")
			},
			wantErr: false,
			expected: map[string]any{
				"@timestamp":         "0.0",
				"observed_timestamp": "0.0",
				"event_name":         "bar",
				"resource":           map[string]any{},
				"scope":              map[string]any{},
				"attributes": map[string]any{
					"event.name": "foo",
				},
			},
		},
		{
			name: "timestamp",
			logCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, record plog.LogRecord) {
				record.SetTimestamp(1721314113467654123)
			},
			wantErr: false,
			expected: map[string]any{
				"@timestamp":         "1721314113467.654123",
				"observed_timestamp": "0.0",
				"resource":           map[string]any{},
				"scope":              map[string]any{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := plog.NewLogs()
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
			record := scopeLogs.LogRecords().AppendEmpty()
			tt.logCustomizer(resourceLogs.Resource(), scopeLogs.Scope(), record)
			logs.MarkReadOnly()

			var buf bytes.Buffer
			ser := New()
			err := ser.SerializeLog(resourceLogs.Resource(), "", scopeLogs.Scope(), "", record, elasticsearch.Index{}, &buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("Log() error = %v, wantErr %v", err, tt.wantErr)
			}
			logBytes := buf.Bytes()
			eventAsJSON := string(logBytes)
			var result any
			decoder := json.NewDecoder(bytes.NewBuffer(logBytes))
			decoder.UseNumber()
			if err := decoder.Decode(&result); err != nil {
				t.Error(err)
			}

			assert.Equal(t, tt.expected, result, eventAsJSON)
		})
	}
}
