package elasticsearchexporter

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"testing"
)

func TestSerializeLog(t *testing.T) {

	tests := []struct {
		name          string
		logCustomizer func(resource pcommon.Resource, scope pcommon.InstrumentationScope, record plog.LogRecord)
		wantErr       bool
		expected      interface{}
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
			_ = record.Attributes().PutEmptySlice("slice").FromRaw([]interface{}{42, "foo"})
			record.Attributes().PutEmptySlice("map_slice").AppendEmpty().SetEmptyMap().PutStr("foo.bar", "baz")
			mapAttr := record.Attributes().PutEmptyMap("map")
			mapAttr.PutStr("foo.bar", "baz")
			mapAttr.PutEmptySlice("inner.slice").AppendEmpty().SetStr("foo")

			resource.Attributes().PutEmptyMap("resource_map").PutStr("foo", "bar")
			scope.Attributes().PutEmptyMap("scope_map").PutStr("foo", "bar")
		}, wantErr: false, expected: map[string]interface{}{
			"@timestamp":         "1970-01-01T00:00:00.000000000Z",
			"observed_timestamp": "1970-01-01T00:00:00.000000000Z",
			"data_stream": map[string]interface{}{
				"type": "logs",
			},
			"severity_text": "debug",
			"resource": map[string]interface{}{
				"attributes": map[string]interface{}{
					"resource_map": `{"foo":"bar"}`,
				},
			},
			"scope": map[string]interface{}{
				"attributes": map[string]interface{}{
					"scope_map": `{"foo":"bar"}`,
				},
			},
			"attributes": map[string]interface{}{
				"empty":  nil,
				"string": "foo",
				"bool":   true,
				"double": json.Number("42.0"),
				"int":    json.Number("42"),
				"bytes":  "2a",
				"slice":  []interface{}{json.Number("42"), "foo"},
				"map_slice": []interface{}{map[string]interface{}{
					"foo.bar": "baz",
				}},
				"map": map[string]interface{}{
					"foo.bar":     "baz",
					"inner.slice": []interface{}{"foo"},
				},
			},
		}},
		{
			name: "text body",
			logCustomizer: func(resource pcommon.Resource, scope pcommon.InstrumentationScope, record plog.LogRecord) {
				record.Body().SetStr("foo")
			},
			wantErr: false,
			expected: map[string]interface{}{
				"@timestamp":         "1970-01-01T00:00:00.000000000Z",
				"observed_timestamp": "1970-01-01T00:00:00.000000000Z",
				"data_stream":        map[string]interface{}{},
				"resource":           map[string]interface{}{},
				"scope":              map[string]interface{}{},
				"body": map[string]interface{}{
					"text": "foo",
				},
			},
		},
		{
			name: "map body",
			logCustomizer: func(resource pcommon.Resource, scope pcommon.InstrumentationScope, record plog.LogRecord) {
				record.Body().SetEmptyMap().PutStr("foo.bar", "baz")
			},
			wantErr: false,
			expected: map[string]interface{}{
				"@timestamp":         "1970-01-01T00:00:00.000000000Z",
				"observed_timestamp": "1970-01-01T00:00:00.000000000Z",
				"data_stream":        map[string]interface{}{},
				"resource":           map[string]interface{}{},
				"scope":              map[string]interface{}{},
				"body": map[string]interface{}{
					"flattened": map[string]interface{}{
						"foo.bar": "baz",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			resourceLogs := plog.NewResourceLogs()
			scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
			record := scopeLogs.LogRecords().AppendEmpty()
			tt.logCustomizer(resourceLogs.Resource(), scopeLogs.Scope(), record)

			logBytes, err := serializeLog(resourceLogs.Resource(), "", scopeLogs.Scope(), "", record)
			if (err != nil) != tt.wantErr {
				t.Errorf("serializeLog() error = %v, wantErr %v", err, tt.wantErr)
			}
			eventAsJson := string(logBytes)
			var result interface{}
			decoder := json.NewDecoder(bytes.NewBuffer(logBytes))
			decoder.UseNumber()
			if err := decoder.Decode(&result); err != nil {
				t.Error(err)
			}

			assert.Equal(t, tt.expected, result, eventAsJson)
		})
	}
}
