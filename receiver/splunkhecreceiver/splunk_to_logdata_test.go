// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

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
		events    []*splunk.Event
		output    plog.ResourceLogsSlice
		hecConfig *Config
		wantErr   error
	}{
		{
			name: "happy_path",
			events: []*splunk.Event{
				{
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
			},
			hecConfig: defaultTestingHecConfig,
			output: func() plog.ResourceLogsSlice {
				return createLogsSlice(nanoseconds)
			}(),
			wantErr: nil,
		},
		{
			name: "double",
			events: []*splunk.Event{
				{
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
			events: []*splunk.Event{
				{
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
			events: []*splunk.Event{
				{
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
			events: []*splunk.Event{
				{
					Host:       "localhost",
					Source:     "mysource",
					SourceType: "mysourcetype",
					Index:      "myindex",
					Event:      "value",
					Fields: map[string]any{
						"foo": "bar",
					},
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
			events: []*splunk.Event{
				{
					Host:       "localhost",
					Source:     "mysource",
					SourceType: "mysourcetype",
					Index:      "myindex",
					Event:      "value",
					Fields: map[string]any{
						"foo": "bar",
					},
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
		{
			name: "group_events_by_resource_attributes",
			events: []*splunk.Event{
				{
					Time:       time,
					Host:       "1",
					Source:     "1",
					SourceType: "1",
					Index:      "1",
					Event:      "Event-1",
					Fields: map[string]any{
						"field": "value1",
					},
				},
				{
					Time:       time,
					Host:       "2",
					Source:     "2",
					SourceType: "2",
					Index:      "2",
					Event:      "Event-2",
					Fields: map[string]any{
						"field": "value2",
					},
				},
				{
					Time:       time,
					Host:       "1",
					Source:     "1",
					SourceType: "1",
					Index:      "1",
					Event:      "Event-3",
					Fields: map[string]any{
						"field": "value1",
					},
				},
				{
					Time:       time,
					Host:       "2",
					Source:     "2",
					SourceType: "2",
					Index:      "2",
					Event:      "Event-4",
					Fields: map[string]any{
						"field": "value2",
					},
				},
				{
					Time:       time,
					Host:       "1",
					Source:     "2",
					SourceType: "1",
					Index:      "2",
					Event:      "Event-5",
					Fields: map[string]any{
						"field": "value1-2",
					},
				},
				{
					Time:       time,
					Host:       "2",
					Source:     "1",
					SourceType: "2",
					Index:      "1",
					Event:      "Event-6",
					Fields: map[string]any{
						"field": "value2-1",
					},
				},
			},
			output: func() plog.ResourceLogsSlice {
				logs := plog.NewLogs()
				{
					lr := logs.ResourceLogs().AppendEmpty()
					updateResourceMap(lr.Resource().Attributes(), "1", "1", "1", "1")
					sl := lr.ScopeLogs().AppendEmpty()
					logRecord := sl.LogRecords().AppendEmpty()
					logRecord.Body().SetStr("Event-1")
					logRecord.SetTimestamp(pcommon.Timestamp(nanoseconds))
					logRecord.Attributes().PutStr("field", "value1")

					logRecord = sl.LogRecords().AppendEmpty()
					logRecord.Body().SetStr("Event-3")
					logRecord.SetTimestamp(pcommon.Timestamp(nanoseconds))
					logRecord.Attributes().PutStr("field", "value1")
				}
				{
					lr := logs.ResourceLogs().AppendEmpty()
					updateResourceMap(lr.Resource().Attributes(), "2", "2", "2", "2")
					sl := lr.ScopeLogs().AppendEmpty()
					logRecord := sl.LogRecords().AppendEmpty()
					logRecord.Body().SetStr("Event-2")
					logRecord.SetTimestamp(pcommon.Timestamp(nanoseconds))
					logRecord.Attributes().PutStr("field", "value2")

					logRecord = sl.LogRecords().AppendEmpty()
					logRecord.Body().SetStr("Event-4")
					logRecord.SetTimestamp(pcommon.Timestamp(nanoseconds))
					logRecord.Attributes().PutStr("field", "value2")
				}
				{
					lr := logs.ResourceLogs().AppendEmpty()
					updateResourceMap(lr.Resource().Attributes(), "1", "2", "1", "2")
					sl := lr.ScopeLogs().AppendEmpty()
					logRecord := sl.LogRecords().AppendEmpty()
					logRecord.Body().SetStr("Event-5")
					logRecord.SetTimestamp(pcommon.Timestamp(nanoseconds))
					logRecord.Attributes().PutStr("field", "value1-2")
				}
				{
					lr := logs.ResourceLogs().AppendEmpty()
					updateResourceMap(lr.Resource().Attributes(), "2", "1", "2", "1")
					sl := lr.ScopeLogs().AppendEmpty()
					logRecord := sl.LogRecords().AppendEmpty()
					logRecord.Body().SetStr("Event-6")
					logRecord.SetTimestamp(pcommon.Timestamp(nanoseconds))
					logRecord.Attributes().PutStr("field", "value2-1")
				}

				return logs.ResourceLogs()
			}(),
			hecConfig: defaultTestingHecConfig,
			wantErr:   nil,
		},
	}
	n := len(tests)
	for _, tt := range tests[n-1:] {
		t.Run(tt.name, func(t *testing.T) {
			result, err := splunkHecToLogData(zap.NewNop(), tt.events, func(resource pcommon.Resource) {}, tt.hecConfig)
			assert.Equal(t, tt.wantErr, err)
			require.Equal(t, tt.output.Len(), result.ResourceLogs().Len())
			for i := 0; i < result.ResourceLogs().Len(); i++ {
				assert.Equal(t, tt.output.At(i), result.ResourceLogs().At(i))
			}
		})
	}
}

func Test_SplunkHecRawToLogData(t *testing.T) {
	const (
		testTimestampVal = 1695146885
	)
	hecConfig := &Config{
		HecToOtelAttrs: splunk.HecToOtelAttrs{
			Source:     "mysource",
			SourceType: "mysourcetype",
			Index:      "myindex",
			Host:       "myhost",
		},
	}
	tests := []struct {
		name           string
		sc             io.Reader
		query          map[string][]string
		assertResource func(t *testing.T, got plog.Logs, slLen int)
		config         *Config
		time           pcommon.Timestamp
	}{
		{
			name: "all_mapping",
			sc: func() io.Reader {
				reader := io.NopCloser(bytes.NewReader([]byte("test")))
				return reader
			}(),
			query: func() map[string][]string {
				m := make(map[string][]string)
				k := []string{"foo"}
				m[host] = k
				m[sourcetype] = k
				m[source] = k
				m[index] = k
				m[queryTime] = []string{"1695146885"}
				return m
			}(),
			assertResource: func(t *testing.T, got plog.Logs, slLen int) {
				assert.Equal(t, 1, slLen)
				attrs := got.ResourceLogs().At(0).Resource().Attributes()
				assert.Equal(t, 4, attrs.Len())
				if v, ok := attrs.Get("myhost"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "host is not added to attributes")
				}
				if v, ok := attrs.Get("mysourcetype"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "sourcetype is not added to attributes")
				}
				if v, ok := attrs.Get("mysource"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "source is not added to attributes")
				}
				if v, ok := attrs.Get("myindex"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "index is not added to attributes")
				}
				assert.Equal(t, time.Unix(testTimestampVal, 0).Unix(), got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Timestamp().AsTime().Unix())
			},
			config: hecConfig,
			time:   pcommon.NewTimestampFromTime(time.Unix(testTimestampVal, 0)),
		},
		{
			name: "some_mapping",
			sc: func() io.Reader {
				reader := io.NopCloser(bytes.NewReader([]byte("test")))
				return reader
			}(),
			query: func() map[string][]string {
				m := make(map[string][]string)
				k := []string{"foo"}
				m[host] = k
				m[sourcetype] = k
				return m
			}(),
			assertResource: func(t *testing.T, got plog.Logs, slLen int) {
				assert.Equal(t, 1, slLen)
				attrs := got.ResourceLogs().At(0).Resource().Attributes()
				assert.Equal(t, 2, attrs.Len())
				if v, ok := attrs.Get("myhost"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "host is not added to attributes")
				}
				if v, ok := attrs.Get("mysourcetype"); ok {
					assert.Equal(t, "foo", v.AsString())
				} else {
					assert.Fail(t, "sourcetype is not added to attributes")
				}
				assert.Equal(t, time.Unix(0, 0).Unix(), got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Timestamp().AsTime().Unix())
			},
			config: hecConfig,
		},
		{
			name: "no splitting",
			sc: func() io.Reader {
				reader := io.NopCloser(bytes.NewReader([]byte("test\nfoo\r\nbar")))
				return reader
			}(),
			query: func() map[string][]string {
				return map[string][]string{}
			}(),
			assertResource: func(t *testing.T, got plog.Logs, slLen int) {
				assert.Equal(t, 1, got.LogRecordCount())
				assert.Equal(t, time.Unix(testTimestampVal, 0).Unix(), got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Timestamp().AsTime().Unix())
			},
			config: func() *Config {
				return &Config{
					Splitting: SplittingStrategyNone,
				}
			}(),
			time: pcommon.NewTimestampFromTime(time.Unix(testTimestampVal, 0)),
		},
		{
			name: "line splitting",
			sc: func() io.Reader {
				reader := io.NopCloser(bytes.NewReader([]byte("test\nfoo\r\nbar")))
				return reader
			}(),
			query: func() map[string][]string {
				return map[string][]string{}
			}(),
			assertResource: func(t *testing.T, got plog.Logs, slLen int) {
				assert.Equal(t, 3, got.LogRecordCount())
			},
			config: func() *Config {
				return &Config{}
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, slLen, err := splunkHecRawToLogData(tt.sc, tt.query, func(resource pcommon.Resource) {}, tt.config, tt.time)
			require.NoError(t, err)
			tt.assertResource(t, result, slLen)
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
	assert.NoError(t, convertToValue(zap.NewNop(), nil, value))
	assert.Equal(t, pcommon.NewValueEmpty(), value)
}

func TestConvertToValueString(t *testing.T) {
	value := pcommon.NewValueEmpty()
	assert.NoError(t, convertToValue(zap.NewNop(), "foo", value))
	assert.Equal(t, pcommon.NewValueStr("foo"), value)
}

func TestConvertToValueBool(t *testing.T) {
	value := pcommon.NewValueEmpty()
	assert.NoError(t, convertToValue(zap.NewNop(), false, value))
	assert.Equal(t, pcommon.NewValueBool(false), value)
}

func TestConvertToValueFloat(t *testing.T) {
	value := pcommon.NewValueEmpty()
	assert.NoError(t, convertToValue(zap.NewNop(), 12.3, value))
	assert.Equal(t, pcommon.NewValueDouble(12.3), value)
}

func TestConvertToValueMap(t *testing.T) {
	value := pcommon.NewValueEmpty()
	assert.NoError(t, convertToValue(zap.NewNop(), map[string]any{"foo": "bar"}, value))
	atts := pcommon.NewValueMap()
	attMap := atts.Map()
	attMap.PutStr("foo", "bar")
	assert.Equal(t, atts, value)
}

func TestConvertToValueArray(t *testing.T) {
	value := pcommon.NewValueEmpty()
	assert.NoError(t, convertToValue(zap.NewNop(), []any{"foo"}, value))
	arrValue := pcommon.NewValueSlice()
	arr := arrValue.Slice()
	arr.AppendEmpty().SetStr("foo")
	assert.Equal(t, arrValue, value)
}

func TestConvertToValueInvalid(t *testing.T) {
	assert.Error(t, convertToValue(zap.NewNop(), splunk.Event{}, pcommon.NewValueEmpty()))
}

func TestConvertToValueInvalidInMap(t *testing.T) {
	assert.Error(t, convertToValue(zap.NewNop(), map[string]any{"foo": splunk.Event{}}, pcommon.NewValueEmpty()))
}

func TestConvertToValueInvalidInArray(t *testing.T) {
	assert.Error(t, convertToValue(zap.NewNop(), []any{splunk.Event{}}, pcommon.NewValueEmpty()))
}
