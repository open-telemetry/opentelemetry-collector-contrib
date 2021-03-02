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
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func Test_logDataToSplunk(t *testing.T) {
	logger := zap.NewNop()
	ts := pdata.TimestampUnixNano(123)

	tests := []struct {
		name             string
		logDataFn        func() pdata.Logs
		configDataFn     func() *Config
		wantSplunkEvents []*splunk.Event
	}{
		{
			name: "valid",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
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
		},
		{
			name: "non-string attribute",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
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
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"foo": float64(123)}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with_config",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
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
		},
		{
			name: "log_is_empty",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(nil, 0, nil, "unknown", "source", "sourcetype"),
			},
		},
		{
			name: "with double body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetDoubleVal(42)
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
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
		},
		{
			name: "with int body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetIntVal(42)
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
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
		},
		{
			name: "with bool body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetBoolVal(true)
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
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
		},
		{
			name: "with map body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				attVal := pdata.NewAttributeValueMap()
				attMap := attVal.MapVal()
				attMap.InsertDouble("23", 45)
				attMap.InsertString("foo", "bar")
				attVal.CopyTo(logRecord.Body())
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
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
		},
		{
			name: "with nil body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
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
		},
		{
			name: "with array body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				attVal := pdata.NewAttributeValueArray()
				attArray := attVal.ArrayVal()
				attArray.Append(pdata.NewAttributeValueString("foo"))
				attVal.CopyTo(logRecord.Body())
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := tt.logDataFn()
			logsWrapper := logDataWrapper{&logs}

			ch, cancel := logsWrapper.eventsInChunks(logger, tt.configDataFn())
			defer cancel()

			events := bytes.Split(bytes.TrimSpace((<-ch).buf.Bytes()), []byte("\r\n\r\n"))

			require.Equal(t, len(tt.wantSplunkEvents), len(events))

			var got splunk.Event
			var gots []*splunk.Event

			for i, event := range events {
				json.Unmarshal(event, &got)
				want := tt.wantSplunkEvents[i]
				// float64 back to int64. int64 unmarshalled to float64 because Event is interface{}.
				if _, ok := want.Event.(int64); ok {
					if g, ok := got.Event.(float64); ok {
						got.Event = int64(g)
					}
				}
				assert.EqualValues(t, tt.wantSplunkEvents[i], &got)
				gots = append(gots, &got)
			}
			assert.Equal(t, tt.wantSplunkEvents, gots)
		})
	}
}

func makeLog(record pdata.LogRecord) pdata.Logs {
	logs := pdata.NewLogs()
	logs.ResourceLogs().Resize(1)
	rl := logs.ResourceLogs().At(0)
	rl.InstrumentationLibraryLogs().Resize(1)
	ill := rl.InstrumentationLibraryLogs().At(0)
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
	logs := pdata.NewLogs()
	ldWrap := logDataWrapper{&logs}
	eventsCh, cancel := ldWrap.eventsInChunks(zap.NewNop(), &Config{})
	defer cancel()
	events := <-eventsCh
	assert.Equal(t, 0, events.buf.Len())
}

func Test_nilResourceLogs(t *testing.T) {
	logs := pdata.NewLogs()
	logs.ResourceLogs().Resize(1)
	ldWrap := logDataWrapper{&logs}
	eventsCh, cancel := ldWrap.eventsInChunks(zap.NewNop(), &Config{})
	defer cancel()
	events := <-eventsCh
	assert.Equal(t, 0, events.buf.Len())
}

func Test_nilInstrumentationLogs(t *testing.T) {
	logs := pdata.NewLogs()
	logs.ResourceLogs().Resize(1)
	resourceLog := logs.ResourceLogs().At(0)
	resourceLog.InstrumentationLibraryLogs().Resize(1)
	ldWrap := logDataWrapper{&logs}
	eventsCh, cancel := ldWrap.eventsInChunks(zap.NewNop(), &Config{})
	defer cancel()
	events := <-eventsCh
	assert.Equal(t, 0, events.buf.Len())
}

func Test_nanoTimestampToEpochMilliseconds(t *testing.T) {
	splunkTs := nanoTimestampToEpochMilliseconds(1001000000)
	assert.Equal(t, 1.001, *splunkTs)
	splunkTs = nanoTimestampToEpochMilliseconds(1001990000)
	assert.Equal(t, 1.002, *splunkTs)
	splunkTs = nanoTimestampToEpochMilliseconds(0)
	assert.True(t, nil == splunkTs)
}

func Test_numLogs(t *testing.T) {
	logs := logDataWrapper{&[]pdata.Logs{pdata.NewLogs()}[0]}
	logs.ResourceLogs().Resize(2)

	rl0 := logs.ResourceLogs().At(0)
	rl0.InstrumentationLibraryLogs().Resize(2)
	rl0.InstrumentationLibraryLogs().At(0).Logs().Append(pdata.NewLogRecord())
	rl0.InstrumentationLibraryLogs().At(1).Logs().Append(pdata.NewLogRecord())
	rl0.InstrumentationLibraryLogs().At(1).Logs().Append(pdata.NewLogRecord())

	rl1 := logs.ResourceLogs().At(1)
	rl1.InstrumentationLibraryLogs().Resize(3)
	rl1.InstrumentationLibraryLogs().At(0).Logs().Append(pdata.NewLogRecord())
	rl1.InstrumentationLibraryLogs().At(1).Logs().Append(pdata.NewLogRecord())
	rl1.InstrumentationLibraryLogs().At(2).Logs().Append(pdata.NewLogRecord())

	// Indices of LogRecord(s) created.
	//     0            1      <- ResourceLogs parent index
	//    / \         / | \
	//   0   1      0  1  2    <- InstrumentationLibraryLogs parent index
	//  /   / \    /  /  /
	// 0   0   1  0  0  0      <- LogRecord index

	_0_0_0 := &logIndex{resource: 0, library: 0, record: 0}
	got := logs.numLogs(_0_0_0)
	assert.Equal(t, 6, got)

	_0_1_1 := &logIndex{resource: 0, library: 1, record: 1}
	got = logs.numLogs(_0_1_1)
	assert.Equal(t, 4, got)
}

func Test_subLogs(t *testing.T) {
	logs := logDataWrapper{&[]pdata.Logs{pdata.NewLogs()}[0]}
	logs.ResourceLogs().Resize(2)

	rl0 := logs.ResourceLogs().At(0)
	rl0.InstrumentationLibraryLogs().Resize(2)

	log := pdata.NewLogRecord()
	log.SetName("(0, 0, 0)")
	rl0.InstrumentationLibraryLogs().At(0).Logs().Append(log)

	log = pdata.NewLogRecord()
	log.SetName("(0, 1, 0)")
	rl0.InstrumentationLibraryLogs().At(1).Logs().Append(log)

	log = pdata.NewLogRecord()
	log.SetName("(0, 1, 1)")
	rl0.InstrumentationLibraryLogs().At(1).Logs().Append(log)

	rl1 := logs.ResourceLogs().At(1)
	rl1.InstrumentationLibraryLogs().Resize(3)

	log = pdata.NewLogRecord()
	log.SetName("(1, 0, 0)")
	rl1.InstrumentationLibraryLogs().At(0).Logs().Append(log)

	log = pdata.NewLogRecord()
	log.SetName("(1, 1, 0)")
	rl1.InstrumentationLibraryLogs().At(1).Logs().Append(log)

	log = pdata.NewLogRecord()
	log.SetName("(1, 2, 0)")
	rl1.InstrumentationLibraryLogs().At(2).Logs().Append(log)

	// Indices of LogRecord(s) created.
	//     0            1      <- ResourceLogs parent index
	//    / \         / | \
	//   0   1      0  1  2    <- InstrumentationLibraryLogs parent index
	//  /   / \    /  /  /
	// 0   0   1  0  0  0      <- LogRecord index

	// Logs subset from leftmost index.
	_0_0_0 := &logIndex{resource: 0, library: 0, record: 0}
	got := logDataWrapper{logs.subLogs(_0_0_0)}

	assert.Equal(t, 6, got.numLogs(_0_0_0))
	orig := *got.InternalRep().Orig
	assert.Equal(t, "(0, 0, 0)", orig[0].InstrumentationLibraryLogs[0].Logs[0].Name)
	assert.Equal(t, "(0, 1, 0)", orig[0].InstrumentationLibraryLogs[1].Logs[0].Name)
	assert.Equal(t, "(0, 1, 1)", orig[0].InstrumentationLibraryLogs[1].Logs[1].Name)
	assert.Equal(t, "(1, 0, 0)", orig[1].InstrumentationLibraryLogs[0].Logs[0].Name)
	assert.Equal(t, "(1, 1, 0)", orig[1].InstrumentationLibraryLogs[1].Logs[0].Name)
	assert.Equal(t, "(1, 2, 0)", orig[1].InstrumentationLibraryLogs[2].Logs[0].Name)

	// Logs subset from rightmost index.
	_1_2_0 := &logIndex{resource: 1, library: 2, record: 0}
	got = logDataWrapper{logs.subLogs(_1_2_0)}

	assert.Equal(t, 1, got.numLogs(_0_0_0))
	orig = *got.InternalRep().Orig
	assert.Equal(t, "(1, 2, 0)", orig[0].InstrumentationLibraryLogs[0].Logs[0].Name)

	// Logs subset from an in-between index.
	_1_1_0 := &logIndex{resource: 1, library: 1, record: 0}
	got = logDataWrapper{logs.subLogs(_1_1_0)}

	assert.Equal(t, 2, got.numLogs(_0_0_0))
	orig = *got.InternalRep().Orig
	assert.Equal(t, "(1, 1, 0)", orig[0].InstrumentationLibraryLogs[0].Logs[0].Name)
	assert.Equal(t, "(1, 2, 0)", orig[0].InstrumentationLibraryLogs[1].Logs[0].Name)
}
