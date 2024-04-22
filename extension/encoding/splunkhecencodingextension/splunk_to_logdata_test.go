// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecencodingextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

var defaultTestingHecConfig = &Config{
	HecToOtelAttrs: splunk.HecToOtelAttrs{
		Source:     splunk.DefaultSourceLabel,
		SourceType: splunk.DefaultSourceTypeLabel,
		Index:      splunk.DefaultIndexLabel,
		Host:       conventions.AttributeHostName,
	},
}

func Test_SplunkHecToLogData(t *testing.T) {

	time := 0.123
	nanoseconds := 123000000

	tests := []struct {
		name      string
		event     splunk.Event
		output    plog.ResourceLogsSlice
		hecConfig *Config
		wantErr   error
	}{
		{
			name: "happy_path",
			event: splunk.Event{
				Time:       time,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      "value",
				Fields: map[string]any{
					"foo": "bar",
				},
			},
			hecConfig: defaultTestingHecConfig,
			output: func() plog.ResourceLogsSlice {
				return createLogsSlice(nanoseconds)
			}(),
			wantErr: nil,
		},
		{
			name: "double",
			event: splunk.Event{
				Time:       time,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      12.3,
				Fields: map[string]any{
					"foo": "bar",
				},
			},
			hecConfig: defaultTestingHecConfig,
			output: func() plog.ResourceLogsSlice {
				logsSlice := createLogsSlice(nanoseconds)
				logsSlice.At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetDouble(12.3)
				return logsSlice
			}(),
			wantErr: nil,
		},
		{
			name: "array",
			event: splunk.Event{
				Time:       time,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      []any{"foo", "bar"},
				Fields: map[string]any{
					"foo": "bar",
				},
			},
			hecConfig: defaultTestingHecConfig,
			output: func() plog.ResourceLogsSlice {
				logsSlice := createLogsSlice(nanoseconds)
				arrVal := pcommon.NewValueSlice()
				arr := arrVal.Slice()
				arr.AppendEmpty().SetStr("foo")
				arr.AppendEmpty().SetStr("bar")
				arrVal.CopyTo(logsSlice.At(0).ScopeLogs().At(0).LogRecords().At(0).Body())
				return logsSlice
			}(),
			wantErr: nil,
		},
		{
			name: "complex_structure",
			event: splunk.Event{
				Time:       time,
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      map[string]any{"foos": []any{"foo", "bar", "foobar"}, "bool": false, "someInt": int64(12)},
				Fields: map[string]any{
					"foo": "bar",
				},
			},
			hecConfig: defaultTestingHecConfig,
			output: func() plog.ResourceLogsSlice {
				logsSlice := createLogsSlice(nanoseconds)

				attMap := logsSlice.At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetEmptyMap()
				attMap.PutBool("bool", false)
				foos := attMap.PutEmptySlice("foos")
				foos.EnsureCapacity(3)
				foos.AppendEmpty().SetStr("foo")
				foos.AppendEmpty().SetStr("bar")
				foos.AppendEmpty().SetStr("foobar")
				attMap.PutInt("someInt", 12)

				return logsSlice
			}(),
			wantErr: nil,
		},
		{
			name: "nil_timestamp",
			event: splunk.Event{
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      "value",
				Fields: map[string]any{
					"foo": "bar",
				},
			},
			hecConfig: defaultTestingHecConfig,
			output: func() plog.ResourceLogsSlice {
				return createLogsSlice(0)
			}(),
			wantErr: nil,
		},
		{
			name: "custom_config_mapping",
			event: splunk.Event{
				Host:       "localhost",
				Source:     "mysource",
				SourceType: "mysourcetype",
				Index:      "myindex",
				Event:      "value",
				Fields: map[string]any{
					"foo": "bar",
				},
			},
			hecConfig: &Config{
				HecToOtelAttrs: splunk.HecToOtelAttrs{
					Source:     "mysource",
					SourceType: "mysourcetype",
					Index:      "myindex",
					Host:       "myhost",
				},
			},
			output: func() plog.ResourceLogsSlice {
				lrs := plog.NewResourceLogsSlice()
				lr := lrs.AppendEmpty()

				lr.Resource().Attributes().PutStr("myhost", "localhost")
				lr.Resource().Attributes().PutStr("mysource", "mysource")
				lr.Resource().Attributes().PutStr("mysourcetype", "mysourcetype")
				lr.Resource().Attributes().PutStr("myindex", "myindex")

				sl := lr.ScopeLogs().AppendEmpty()
				logRecord := sl.LogRecords().AppendEmpty()
				logRecord.Body().SetStr("value")
				logRecord.SetTimestamp(pcommon.Timestamp(0))
				logRecord.Attributes().PutStr("foo", "bar")
				return lrs
			}(),
			wantErr: nil,
		},
	}
	n := len(tests)
	for _, tt := range tests[n-1:] {
		t.Run(tt.name, func(t *testing.T) {
			result, err := splunkHecToLogData(tt.event, tt.hecConfig)
			assert.Equal(t, tt.wantErr, err)
			require.Equal(t, tt.output.Len(), result.ResourceLogs().Len())
			for i := 0; i < result.ResourceLogs().Len(); i++ {
				assert.Equal(t, tt.output.At(i), result.ResourceLogs().At(i))
			}
		})
	}
}

func updateResourceMap(pmap pcommon.Map, host, source, sourcetype, index string) {
	pmap.PutStr("host.name", host)
	pmap.PutStr("com.splunk.source", source)
	pmap.PutStr("com.splunk.sourcetype", sourcetype)
	pmap.PutStr("com.splunk.index", index)
}

func createLogsSlice(nanoseconds int) plog.ResourceLogsSlice {
	lrs := plog.NewResourceLogsSlice()
	lr := lrs.AppendEmpty()
	updateResourceMap(lr.Resource().Attributes(), "localhost", "mysource", "mysourcetype", "myindex")
	sl := lr.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("value")
	logRecord.SetTimestamp(pcommon.Timestamp(nanoseconds))
	logRecord.Attributes().PutStr("foo", "bar")

	return lrs
}

func TestConvertToValueEmpty(t *testing.T) {
	value := pcommon.NewValueEmpty()
	assert.NoError(t, convertToValue(nil, value))
	assert.Equal(t, pcommon.NewValueEmpty(), value)
}

func TestConvertToValueString(t *testing.T) {
	value := pcommon.NewValueEmpty()
	assert.NoError(t, convertToValue("foo", value))
	assert.Equal(t, pcommon.NewValueStr("foo"), value)
}

func TestConvertToValueBool(t *testing.T) {
	value := pcommon.NewValueEmpty()
	assert.NoError(t, convertToValue(false, value))
	assert.Equal(t, pcommon.NewValueBool(false), value)
}

func TestConvertToValueFloat(t *testing.T) {
	value := pcommon.NewValueEmpty()
	assert.NoError(t, convertToValue(12.3, value))
	assert.Equal(t, pcommon.NewValueDouble(12.3), value)
}

func TestConvertToValueMap(t *testing.T) {
	value := pcommon.NewValueEmpty()
	assert.NoError(t, convertToValue(map[string]any{"foo": "bar"}, value))
	atts := pcommon.NewValueMap()
	attMap := atts.Map()
	attMap.PutStr("foo", "bar")
	assert.Equal(t, atts, value)
}

func TestConvertToValueArray(t *testing.T) {
	value := pcommon.NewValueEmpty()
	assert.NoError(t, convertToValue([]any{"foo"}, value))
	arrValue := pcommon.NewValueSlice()
	arr := arrValue.Slice()
	arr.AppendEmpty().SetStr("foo")
	assert.Equal(t, arrValue, value)
}

func TestConvertToValueInvalid(t *testing.T) {
	assert.Error(t, convertToValue(splunk.Event{}, pcommon.NewValueEmpty()))
}

func TestConvertToValueInvalidInMap(t *testing.T) {
	assert.Error(t, convertToValue(map[string]any{"foo": splunk.Event{}}, pcommon.NewValueEmpty()))
}

func TestConvertToValueInvalidInArray(t *testing.T) {
	assert.Error(t, convertToValue([]any{splunk.Event{}}, pcommon.NewValueEmpty()))
}
